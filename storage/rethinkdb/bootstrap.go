package rethinkdb

import (
	"fmt"
	"strings"

	"github.com/dancannon/gorethink"
)

func makeDB(session *gorethink.Session, name string) error {
	_, err := gorethink.DBCreate(name).RunWrite(session)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}

		return err
	}
	// Note: Wait() not supported by RethinkDB 2.3+
	resp, err := gorethink.DB(name).Wait().Run(session)
	if resp != nil {
		resp.Close()
	}

	return err
}

// Table holds the configuration for setting up a RethinkDB table
type Table struct {
	Name       string
	PrimaryKey interface{}
	// Keys are the index names. If len(value) is 0, it is a simple index
	// on the field matching the key. Otherwise, it is a compound index
	// on the list of fields in the corrensponding slice value.
	SecondaryIndexes map[string][]string
	Config           map[string]string
}

func (t Table) term(dbName string) gorethink.Term {
	return gorethink.DB(dbName).Table(t.Name)
}

func (t Table) wait(session *gorethink.Session, dbName string) error {
	// Note: Wait() not supported by RethinkDB 2.3+
	resp, err := t.term(dbName).Wait().Run(session)

	if resp != nil {
		resp.Close()
	}

	return err
}

func (t Table) create(session *gorethink.Session, dbName string, numReplicas uint) error {
	createOpts := gorethink.TableCreateOpts{
		PrimaryKey: t.PrimaryKey,
		Durability: "hard",
	}

	if _, err := gorethink.DB(dbName).TableCreate(t.Name, createOpts).RunWrite(session); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("unable to run table creation: %s", err)
		}
	}

	reconfigureOpts := gorethink.ReconfigureOpts{
		Shards:   1,
		Replicas: numReplicas,
	}

	if _, err := t.term(dbName).Reconfigure(reconfigureOpts).RunWrite(session); err != nil {
		return fmt.Errorf("unable to reconfigure table replication: %s", err)
	}

	if err := t.wait(session, dbName); err != nil {
		return fmt.Errorf("unable to wait for table to be ready after reconfiguring replication: %s", err)
	}

	if _, err := t.term(dbName).Config().Update(t.Config).RunWrite(session); err != nil {
		return fmt.Errorf("unable to configure table linearizability: %s", err)
	}

	if err := t.wait(session, dbName); err != nil {
		return fmt.Errorf("unable to wait for table to be ready after configuring linearizability: %s", err)
	}

	for indexName, fieldNames := range t.SecondaryIndexes {
		if len(fieldNames) == 0 {
			// The field name is the index name.
			fieldNames = []string{indexName}
		}

		if _, err := t.term(dbName).IndexCreateFunc(indexName, func(row gorethink.Term) interface{} {
			fields := make([]interface{}, len(fieldNames))

			for i, fieldName := range fieldNames {
				term := row
				for _, subfield := range strings.Split(fieldName, ".") {
					term = term.Field(subfield)
				}

				fields[i] = term
			}

			if len(fields) == 1 {
				return fields[0]
			}

			return fields
		}).RunWrite(session); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("unable to create secondary index %q: %s", indexName, err)
			}
		}
	}

	if err := t.wait(session, dbName); err != nil {
		return fmt.Errorf("unable to wait for table to be ready after creating secondary indexes: %s", err)
	}

	return nil
}

// SetupDB handles creating the database and creating all tables and indexes.
func SetupDB(session *gorethink.Session, dbName string, tables []Table) error {
	if err := makeDB(session, dbName); err != nil {
		return fmt.Errorf("unable to create database: %s", err)
	}

	cursor, err := gorethink.DB("rethinkdb").Table("server_config").Count().Run(session)
	if err != nil {
		return fmt.Errorf("unable to query db server config: %s", err)
	}

	var replicaCount uint
	if err := cursor.One(&replicaCount); err != nil {
		return fmt.Errorf("unable to scan db server config count: %s", err)
	}

	for _, table := range tables {
		if err = table.create(session, dbName, replicaCount); err != nil {
			return fmt.Errorf("unable to create table %q: %s", table.Name, err)
		}
	}

	return nil
}

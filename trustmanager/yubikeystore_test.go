// +build pkcs11

package trustmanager

import (
	"crypto/rand"
	"testing"

	"github.com/docker/notary/passphrase"
	"github.com/docker/notary/tuf/data"
	"github.com/stretchr/testify/assert"
)

func clearAllKeys(t *testing.T) {
	// TODO(cyli): this is creating a new yubikey store because for some reason,
	// removing and then adding with the same YubiKeyStore causes
	// non-deterministic failures at least on Mac OS
	ret := passphrase.ConstantRetriever("passphrase")
	store, err := NewYubiKeyStore(NewKeyMemoryStore(ret), ret)
	assert.NoError(t, err)

	for k := range store.ListKeys() {
		err := store.RemoveKey(k)
		assert.NoError(t, err)
	}
}

func testAddKey(t *testing.T, store *YubiKeyStore) (data.PrivateKey, error) {
	privKey, err := GenerateECDSAKey(rand.Reader)
	assert.NoError(t, err)

	err = store.AddKey(privKey.ID(), data.CanonicalRootRole, privKey)
	return privKey, err
}

func TestAddKeyToNextEmptyYubikeySlot(t *testing.T) {
	if !YubikeyAccessible() {
		t.Skip("Must have Yubikey access.")
	}
	clearAllKeys(t)

	ret := passphrase.ConstantRetriever("passphrase")
	store, err := NewYubiKeyStore(NewKeyMemoryStore(ret), ret)
	assert.NoError(t, err)
	SetYubikeyKeyMode(KeymodeNone)
	defer func() {
		SetYubikeyKeyMode(KeymodeTouch | KeymodePinOnce)
	}()

	keys := make([]string, 0, numSlots)

	// create the maximum number of keys
	for i := 0; i < numSlots; i++ {
		privKey, err := testAddKey(t, store)
		assert.NoError(t, err)
		keys = append(keys, privKey.ID())
	}

	// create a new store, to make sure we're not just using the keys cache
	store, err = NewYubiKeyStore(NewKeyMemoryStore(ret), ret)
	assert.NoError(t, err)
	listedKeys := store.ListKeys()
	assert.Len(t, listedKeys, numSlots)
	for _, k := range keys {
		r, ok := listedKeys[k]
		assert.True(t, ok)
		assert.Equal(t, data.CanonicalRootRole, r)
	}

	// add another key - should fail because there are no more slots
	_, err = testAddKey(t, store)
	assert.Error(t, err)

	// delete one of the middle keys, and assert we can still create a new key
	err = store.RemoveKey(keys[numSlots/2])
	assert.NoError(t, err)

	_, err = testAddKey(t, store)
	assert.NoError(t, err)
}
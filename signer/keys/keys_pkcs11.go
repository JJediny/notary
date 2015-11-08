// +build pkcs11

package keys

import (
	"github.com/docker/notary/tuf/data"
	"github.com/miekg/pkcs11"
)

// HSMRSAKey represents the information for an HSMRSAKey with ObjectHandle for private portion
type HSMRSAKey struct {
	id      string
	public  []byte
	private pkcs11.ObjectHandle
}

// NewHSMRSAKey returns a HSMRSAKey
func NewHSMRSAKey(public []byte, private pkcs11.ObjectHandle) *HSMRSAKey {
	return &HSMRSAKey{
		public:  public,
		private: private,
	}
}

// Algorithm implements a method of the data.Key interface
func (rsa *HSMRSAKey) Algorithm() string {
	return data.RSAKey
}

// ID implements a method of the data.Key interface
func (rsa *HSMRSAKey) ID() string {
	if rsa.id == "" {
		pubK := data.NewPublicKey(rsa.Algorithm(), rsa.Public())
		rsa.id = pubK.ID()
	}
	return rsa.id
}

// Public implements a method of the data.Key interface
func (rsa *HSMRSAKey) Public() []byte {
	return rsa.public
}

// Private implements a method of the data.PrivateKey interface
func (rsa *HSMRSAKey) Private() []byte {
	// Not possible to return private key bytes from a hardware device
	return nil
}

// PKCS11ObjectHandle returns the PKCS11 object handle stored in the HSMRSAKey
// structure
func (rsa *HSMRSAKey) PKCS11ObjectHandle() pkcs11.ObjectHandle {
	return rsa.private
}
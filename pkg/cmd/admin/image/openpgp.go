package image

// TODO: Remove this wrapper when the 'containers/image' is suitable to pull as godep.

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp"
)

type SigningMechanism interface {
	Close() error
	// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
	SupportsSigning() error
	// Sign creates a (non-detached) signature of input using keyIdentity.
	// Fails with a SigningNotSupportedError if the mechanism does not support signing.
	Sign(input []byte, keyIdentity string) ([]byte, error)
	// Verify parses unverifiedSignature and returns the content and the signer's identity
	Verify(unverifiedSignature []byte) (contents []byte, keyIdentity string, err error)
}

// A GPG/OpenPGP signing mechanism, implemented using x/crypto/openpgp.
type openpgpSigningMechanism struct {
	keyring openpgp.EntityList
}

// SigningNotSupportedError is returned when trying to sign using a mechanism which does not support that.
type SigningNotSupportedError string

func (err SigningNotSupportedError) Error() string {
	return string(err)
}

// InvalidSignatureError is returned when parsing an invalid signature.
type InvalidSignatureError struct {
	msg string
}

func (err InvalidSignatureError) Error() string {
	return err.msg
}

// newGPGSigningMechanismInDirectory returns a new GPG/OpenPGP signing mechanism, using optionalDir if not empty.
// The caller must call .Close() on the returned SigningMechanism.
func newGPGSigningMechanismInDirectory(optionalDir string) (SigningMechanism, error) {
	m := &openpgpSigningMechanism{
		keyring: openpgp.EntityList{},
	}

	homeDir := os.Getenv("HOME")
	gpgHome := optionalDir
	if gpgHome == "" {
		gpgHome = os.Getenv("GNUPGHOME")
		if gpgHome == "" {
			gpgHome = path.Join(homeDir, ".gnupg")
		}
	}

	pubring, err := ioutil.ReadFile(path.Join(gpgHome, "pubring.gpg"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		_, err := m.importKeysFromBytes(pubring)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// newEphemeralGPGSigningMechanism returns a new GPG/OpenPGP signing mechanism which
// recognizes _only_ public keys from the supplied blob, and returns the identities
// of these keys.
// The caller must call .Close() on the returned SigningMechanism.
func newEphemeralGPGSigningMechanism(blob []byte) (SigningMechanism, []string, error) {
	m := &openpgpSigningMechanism{
		keyring: openpgp.EntityList{},
	}
	keyIdentities, err := m.importKeysFromBytes(blob)
	if err != nil {
		return nil, nil, err
	}
	return m, keyIdentities, nil
}

func (m *openpgpSigningMechanism) Close() error {
	return nil
}

// importKeysFromBytes imports public keys from the supplied blob and returns their identities.
// The blob is assumed to have an appropriate format (the caller is expected to know which one).
func (m *openpgpSigningMechanism) importKeysFromBytes(blob []byte) ([]string, error) {
	keyring, err := openpgp.ReadKeyRing(bytes.NewReader(blob))
	if err != nil {
		k, e2 := openpgp.ReadArmoredKeyRing(bytes.NewReader(blob))
		if e2 != nil {
			return nil, err // The original error  -- FIXME: is this better?
		}
		keyring = k
	}

	keyIdentities := []string{}
	for _, entity := range keyring {
		if entity.PrimaryKey == nil {
			continue
		}
		// Uppercase the fingerprint to be compatible with gpgme
		keyIdentities = append(keyIdentities, strings.ToUpper(fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)))
		m.keyring = append(m.keyring, entity)
	}
	return keyIdentities, nil
}

// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
func (m *openpgpSigningMechanism) SupportsSigning() error {
	return SigningNotSupportedError("signing is not supported in github.com/containers/image built with the containers_image_openpgp build tag")
}

// Sign creates a (non-detached) signature of input using keyIdentity.
// Fails with a SigningNotSupportedError if the mechanism does not support signing.
func (m *openpgpSigningMechanism) Sign(input []byte, keyIdentity string) ([]byte, error) {
	return nil, SigningNotSupportedError("signing is not supported in github.com/containers/image built with the containers_image_openpgp build tag")
}

// Verify parses unverifiedSignature and returns the content and the signer's identity
func (m *openpgpSigningMechanism) Verify(unverifiedSignature []byte) (contents []byte, keyIdentity string, err error) {
	md, err := openpgp.ReadMessage(bytes.NewReader(unverifiedSignature), m.keyring, nil, nil)
	if err != nil {
		return nil, "", err
	}
	if !md.IsSigned {
		return nil, "", errors.New("not signed")
	}
	content, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, "", err
	}
	if md.SignatureError != nil {
		return nil, "", fmt.Errorf("signature error: %v", md.SignatureError)
	}
	if md.SignedBy == nil {
		return nil, "", InvalidSignatureError{msg: fmt.Sprintf("Invalid GPG signature: %#v", md.Signature)}
	}
	if md.Signature.SigLifetimeSecs != nil {
		expiry := md.Signature.CreationTime.Add(time.Duration(*md.Signature.SigLifetimeSecs) * time.Second)
		if time.Now().After(expiry) {
			return nil, "", InvalidSignatureError{msg: fmt.Sprintf("Signature expired on %s", expiry)}
		}
	}

	// Uppercase the fingerprint to be compatible with gpgme
	return content, strings.ToUpper(fmt.Sprintf("%x", md.SignedBy.PublicKey.Fingerprint)), nil
}

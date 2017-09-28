package signature

// These tests are expected to pass unmodified for _both_ mechanism_gpgme.go and mechanism_openpgp.go.

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGPGHomeDirectory = "./fixtures"
)

// Many of the tests use two fixtures: V4 signature packets (*.signature), and V3 signature packets (*.signature-v3)

// fixtureVariants loads V3 and V4 signature fixture variants based on the v4 fixture path, and returns a map which makes it easy to test both.
func fixtureVariants(t *testing.T, v4Path string) map[string][]byte {
	v4, err := ioutil.ReadFile(v4Path)
	require.NoError(t, err)
	v3Path := v4Path + "-v3"
	v3, err := ioutil.ReadFile(v3Path)
	require.NoError(t, err)
	return map[string][]byte{v4Path: v4, v3Path: v3}
}

func TestSigningNotSupportedError(t *testing.T) {
	// A stupid test just to keep code coverage
	s := "test"
	err := SigningNotSupportedError(s)
	assert.Equal(t, s, err.Error())
}

func TestNewGPGSigningMechanism(t *testing.T) {
	// A dumb test just for code coverage. We test more with newGPGSigningMechanismInDirectory().
	mech, err := NewGPGSigningMechanism()
	assert.NoError(t, err)
	mech.Close()
}

func TestNewGPGSigningMechanismInDirectory(t *testing.T) {
	// A dumb test just for code coverage.
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	assert.NoError(t, err)
	mech.Close()
	// The various GPG failure cases are not obviously easy to reach.

	// Test that using the default directory (presumably in user’s home)
	// cannot use TestKeyFingerprint.
	signatures := fixtureVariants(t, "./fixtures/invalid-blob.signature")
	mech, err = newGPGSigningMechanismInDirectory("")
	require.NoError(t, err)
	defer mech.Close()
	for version, signature := range signatures {
		_, _, err := mech.Verify(signature)
		assert.Error(t, err, version)
	}

	// Similarly, using a newly created empty directory makes TestKeyFingerprint
	// unavailable
	emptyDir, err := ioutil.TempDir("", "signing-empty-directory")
	require.NoError(t, err)
	defer os.RemoveAll(emptyDir)
	mech, err = newGPGSigningMechanismInDirectory(emptyDir)
	require.NoError(t, err)
	defer mech.Close()
	for version, signature := range signatures {
		_, _, err := mech.Verify(signature)
		assert.Error(t, err, version)
	}

	// If pubring.gpg is unreadable in the directory, either initializing
	// the mechanism fails (with openpgp), or it succeeds (sadly, gpgme) and
	// later verification fails.
	unreadableDir, err := ioutil.TempDir("", "signing-unreadable-directory")
	require.NoError(t, err)
	defer os.RemoveAll(unreadableDir)
	f, err := os.OpenFile(filepath.Join(unreadableDir, "pubring.gpg"), os.O_RDONLY|os.O_CREATE, 0000)
	require.NoError(t, err)
	f.Close()
	mech, err = newGPGSigningMechanismInDirectory(unreadableDir)
	if err == nil {
		defer mech.Close()
		for version, signature := range signatures {
			_, _, err := mech.Verify(signature)
			assert.Error(t, err, version)
		}
	}

	// Setting the directory parameter to testGPGHomeDirectory makes the key available.
	mech, err = newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()
	for version, signature := range signatures {
		_, _, err := mech.Verify(signature)
		assert.NoError(t, err, version)
	}

	// If we use the default directory mechanism, GNUPGHOME is respected.
	origGNUPGHOME := os.Getenv("GNUPGHOME")
	defer os.Setenv("GNUPGHOME", origGNUPGHOME)
	os.Setenv("GNUPGHOME", testGPGHomeDirectory)
	mech, err = newGPGSigningMechanismInDirectory("")
	require.NoError(t, err)
	defer mech.Close()
	for version, signature := range signatures {
		_, _, err := mech.Verify(signature)
		assert.NoError(t, err, version)
	}
}

func TestNewEphemeralGPGSigningMechanism(t *testing.T) {
	// Empty input: This is accepted anyway by GPG, just returns no keys.
	mech, keyIdentities, err := NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()
	assert.Empty(t, keyIdentities)
	// Try validating a signature when the key is unknown.
	signatures := fixtureVariants(t, "./fixtures/invalid-blob.signature")
	for version, signature := range signatures {
		_, _, err := mech.Verify(signature)
		require.Error(t, err, version)
	}

	// Successful import
	keyBlob, err := ioutil.ReadFile("./fixtures/public-key.gpg")
	require.NoError(t, err)
	mech, keyIdentities, err = NewEphemeralGPGSigningMechanism(keyBlob)
	require.NoError(t, err)
	defer mech.Close()
	assert.Equal(t, []string{TestKeyFingerprint}, keyIdentities)
	// After import, the signature should validate.
	for version, signature := range signatures {
		content, signingFingerprint, err := mech.Verify(signature)
		require.NoError(t, err, version)
		assert.Equal(t, []byte("This is not JSON\n"), content, version)
		assert.Equal(t, TestKeyFingerprint, signingFingerprint, version)
	}

	// Two keys: Read the binary-format pubring.gpg, and concatenate it twice.
	// (Using two copies of public-key.gpg, in the ASCII-armored format, works with
	// gpgmeSigningMechanism but not openpgpSigningMechanism.)
	keyBlob, err = ioutil.ReadFile("./fixtures/pubring.gpg")
	require.NoError(t, err)
	mech, keyIdentities, err = NewEphemeralGPGSigningMechanism(bytes.Join([][]byte{keyBlob, keyBlob}, nil))
	require.NoError(t, err)
	defer mech.Close()
	assert.Equal(t, []string{TestKeyFingerprint, TestKeyFingerprint}, keyIdentities)

	// Invalid input: This is, sadly, accepted anyway by GPG, just returns no keys.
	// For openpgpSigningMechanism we can detect this and fail.
	mech, keyIdentities, err = NewEphemeralGPGSigningMechanism([]byte("This is invalid"))
	assert.True(t, err != nil || len(keyIdentities) == 0)
	if err == nil {
		mech.Close()
	}
	assert.Empty(t, keyIdentities)
	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func TestGPGSigningMechanismClose(t *testing.T) {
	// Closing a non-ephemeral mechanism does not remove anything in the directory.
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	err = mech.Close()
	assert.NoError(t, err)
	_, err = os.Lstat(testGPGHomeDirectory)
	assert.NoError(t, err)
	_, err = os.Lstat(filepath.Join(testGPGHomeDirectory, "pubring.gpg"))
	assert.NoError(t, err)
}

func TestGPGSigningMechanismSign(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()

	if err := mech.SupportsSigning(); err != nil {
		t.Skipf("Signing not supported: %v", err)
	}

	// Successful signing
	content := []byte("content")
	signature, err := mech.Sign(content, TestKeyFingerprint)
	require.NoError(t, err)

	signedContent, signingFingerprint, err := mech.Verify(signature)
	require.NoError(t, err)
	assert.EqualValues(t, content, signedContent)
	assert.Equal(t, TestKeyFingerprint, signingFingerprint)

	// Error signing
	_, err = mech.Sign(content, "this fingerprint doesn't exist")
	assert.Error(t, err)
	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func assertSigningError(t *testing.T, content []byte, fingerprint string, err error, msgAndArgs ...interface{}) {
	assert.Error(t, err, msgAndArgs...)
	assert.Nil(t, content, msgAndArgs...)
	assert.Empty(t, fingerprint, msgAndArgs...)
}

func TestGPGSigningMechanismVerify(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()

	// Successful verification
	signatures := fixtureVariants(t, "./fixtures/invalid-blob.signature")
	for variant, signature := range signatures {
		content, signingFingerprint, err := mech.Verify(signature)
		require.NoError(t, err, variant)
		assert.Equal(t, []byte("This is not JSON\n"), content, variant)
		assert.Equal(t, TestKeyFingerprint, signingFingerprint, variant)
	}

	// For extra paranoia, test that we return nil data on error.

	// Completely invalid signature.
	content, signingFingerprint, err := mech.Verify([]byte{})
	assertSigningError(t, content, signingFingerprint, err)

	content, signingFingerprint, err = mech.Verify([]byte("invalid signature"))
	assertSigningError(t, content, signingFingerprint, err)

	// Literal packet, not a signature
	signature, err := ioutil.ReadFile("./fixtures/unsigned-literal.signature") // Not fixtureVariants, the “literal data” packet does not have V3/V4 versions.
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Encrypted data, not a signature.
	signature, err = ioutil.ReadFile("./fixtures/unsigned-encrypted.signature") // Not fixtureVariants, the “public-key encrypted session key” does not have V3/V4 versions.
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// FIXME? Is there a way to create a multi-signature so that gpgme_op_verify returns multiple signatures?

	// Expired signature
	signature, err = ioutil.ReadFile("./fixtures/expired.signature") // Not fixtureVariants, V3 signature packets don’t support expiration.
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Corrupt signature
	signatures = fixtureVariants(t, "./fixtures/corrupt.signature")
	for version, signature := range signatures {
		content, signingFingerprint, err := mech.Verify(signature)
		assertSigningError(t, content, signingFingerprint, err, version)
	}

	// Valid signature with an unknown key
	signatures = fixtureVariants(t, "./fixtures/unknown-key.signature")
	for version, signature := range signatures {
		content, signingFingerprint, err := mech.Verify(signature)
		assertSigningError(t, content, signingFingerprint, err, version)
	}

	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func TestGPGSigningMechanismUntrustedSignatureContents(t *testing.T) {
	mech, _, err := NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()

	// A valid signature
	signatures := fixtureVariants(t, "./fixtures/invalid-blob.signature")
	for version, signature := range signatures {
		content, shortKeyID, err := mech.UntrustedSignatureContents(signature)
		require.NoError(t, err, version)
		assert.Equal(t, []byte("This is not JSON\n"), content, version)
		assert.Equal(t, TestKeyShortID, shortKeyID, version)
	}

	// Completely invalid signature.
	_, _, err = mech.UntrustedSignatureContents([]byte{})
	assert.Error(t, err)

	_, _, err = mech.UntrustedSignatureContents([]byte("invalid signature"))
	assert.Error(t, err)

	// Literal packet, not a signature
	signature, err := ioutil.ReadFile("./fixtures/unsigned-literal.signature") // Not fixtureVariants, the “literal data” packet does not have V3/V4 versions.
	require.NoError(t, err)
	content, shortKeyID, err := mech.UntrustedSignatureContents(signature)
	assert.Error(t, err)

	// Encrypted data, not a signature.
	signature, err = ioutil.ReadFile("./fixtures/unsigned-encrypted.signature") // Not fixtureVariants, the “public-key encrypted session key” does not have V3/V4 versions.
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	assert.Error(t, err)

	// Expired signature
	signature, err = ioutil.ReadFile("./fixtures/expired.signature") // Not fixtureVariants, V3 signature packets don’t support expiration.
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte("This signature is expired.\n"), content)
	assert.Equal(t, TestKeyShortID, shortKeyID)

	// Corrupt signature
	signatures = fixtureVariants(t, "./fixtures/corrupt.signature")
	for version, signature := range signatures {
		content, shortKeyID, err := mech.UntrustedSignatureContents(signature)
		require.NoError(t, err, version)
		assert.Equal(t, []byte(`{"critical":{"identity":{"docker-reference":"testing/manifest"},"image":{"docker-manifest-digest":"sha256:20bf21ed457b390829cdbeec8795a7bea1626991fda603e0d01b4e7f60427e55"},"type":"atomic container signature"},"optional":{"creator":"atomic ","timestamp":1458239713}}`), content, version)
		assert.Equal(t, TestKeyShortID, shortKeyID, version)
	}

	// Valid signature with an unknown key
	signatures = fixtureVariants(t, "./fixtures/unknown-key.signature")
	for version, signature := range signatures {
		content, shortKeyID, err := mech.UntrustedSignatureContents(signature)
		require.NoError(t, err, version)
		assert.Equal(t, []byte(`{"critical":{"identity":{"docker-reference":"testing/manifest"},"image":{"docker-manifest-digest":"sha256:20bf21ed457b390829cdbeec8795a7bea1626991fda603e0d01b4e7f60427e55"},"type":"atomic container signature"},"optional":{"creator":"atomic 0.1.13-dev","timestamp":1464633474}}`), content, version)
		assert.Equal(t, "BB75E91990DF8F7E", shortKeyID, version)
	}
}

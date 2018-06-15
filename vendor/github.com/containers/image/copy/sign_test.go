package copy

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/image/directory"
	"github.com/containers/image/docker"
	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGPGHomeDirectory = "../signature/fixtures"
	// TestKeyFingerprint is the fingerprint of the private key in testGPGHomeDirectory.
	// Keep this in sync with signature/fixtures_info_test.go
	testKeyFingerprint = "1D8230F6CDB6A06716E414C1DB72F2188BB46CC8"
)

func TestCreateSignature(t *testing.T) {
	manifestBlob := []byte("Something")
	manifestDigest, err := manifest.Digest(manifestBlob)
	require.NoError(t, err)

	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		t.Skipf("Signing not supported: %v", err)
	}

	os.Setenv("GNUPGHOME", testGPGHomeDirectory)
	defer os.Unsetenv("GNUPGHOME")

	// Signing a directory: reference, which does not have a DockerRefrence(), fails.
	tempDir, err := ioutil.TempDir("", "signature-dir-dest")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	dirRef, err := directory.NewReference(tempDir)
	require.NoError(t, err)
	dirDest, err := dirRef.NewImageDestination(nil)
	require.NoError(t, err)
	defer dirDest.Close()
	_, err = createSignature(dirDest, manifestBlob, testKeyFingerprint, ioutil.Discard)
	assert.Error(t, err)

	// Set up a docker: reference
	dockerRef, err := docker.ParseReference("//busybox")
	require.NoError(t, err)
	dockerDest, err := dockerRef.NewImageDestination(&types.SystemContext{RegistriesDirPath: "/this/doesnt/exist", DockerPerHostCertDirPath: "/this/doesnt/exist"})
	require.NoError(t, err)
	defer dockerDest.Close()

	// Signing with an unknown key fails
	_, err = createSignature(dockerDest, manifestBlob, "this key does not exist", ioutil.Discard)
	assert.Error(t, err)

	// Success
	mech, err = signature.NewGPGSigningMechanism()
	require.NoError(t, err)
	defer mech.Close()
	sig, err := createSignature(dockerDest, manifestBlob, testKeyFingerprint, ioutil.Discard)
	require.NoError(t, err)
	verified, err := signature.VerifyDockerManifestSignature(sig, manifestBlob, "docker.io/library/busybox:latest", mech, testKeyFingerprint)
	require.NoError(t, err)
	assert.Equal(t, "docker.io/library/busybox:latest", verified.DockerReference)
	assert.Equal(t, manifestDigest, verified.DockerManifestDigest)
}

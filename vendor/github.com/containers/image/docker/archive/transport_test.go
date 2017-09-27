package archive

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sha256digestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sha256digest    = "@sha256:" + sha256digestHex
	tarFixture      = "fixtures/almostempty.tar"
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "docker-archive", Transport.Name())
}

func TestTransportParseReference(t *testing.T) {
	testParseReference(t, Transport.ParseReference)
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{ // A semi-representative assortment of values; everything is rejected.
		"docker.io/library/busybox:notlatest",
		"docker.io/library/busybox",
		"docker.io/library",
		"docker.io",
		"",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}

func TestParseReference(t *testing.T) {
	testParseReference(t, ParseReference)
}

// testParseReference is a test shared for Transport.ParseReference and ParseReference.
func testParseReference(t *testing.T, fn func(string) (types.ImageReference, error)) {
	for _, c := range []struct{ input, expectedPath, expectedRef string }{
		{"", "", ""}, // Empty input is explicitly rejected
		{"/path", "/path", ""},
		{"/path:busybox:notlatest", "/path", "docker.io/library/busybox:notlatest"}, // Explicit tag
		{"/path:busybox" + sha256digest, "", ""},                                    // Digest references are forbidden
		{"/path:busybox", "/path", "docker.io/library/busybox:latest"},              // Default tag
		// A github.com/distribution/reference value can have a tag and a digest at the same time!
		{"/path:busybox:latest" + sha256digest, "", ""},                                         // Both tag and digest is rejected
		{"/path:docker.io/library/busybox:latest", "/path", "docker.io/library/busybox:latest"}, // All implied values explicitly specified
		{"/path:UPPERCASEISINVALID", "", ""},                                                    // Invalid input
	} {
		ref, err := fn(c.input)
		if c.expectedPath == "" {
			assert.Error(t, err, c.input)
		} else {
			require.NoError(t, err, c.input)
			archiveRef, ok := ref.(archiveReference)
			require.True(t, ok, c.input)
			assert.Equal(t, c.expectedPath, archiveRef.path, c.input)
			if c.expectedRef == "" {
				assert.Nil(t, archiveRef.destinationRef, c.input)
			} else {
				require.NotNil(t, archiveRef.destinationRef, c.input)
				assert.Equal(t, c.expectedRef, archiveRef.destinationRef.String(), c.input)
			}
		}
	}
}

// refWithTagAndDigest is a reference.NamedTagged and reference.Canonical at the same time.
type refWithTagAndDigest struct{ reference.Canonical }

func (ref refWithTagAndDigest) Tag() string {
	return "notLatest"
}

// A common list of reference formats to test for the various ImageReference methods.
var validReferenceTestCases = []struct{ input, dockerRef, stringWithinTransport string }{
	{"/pathonly", "", "/pathonly"},
	{"/path:busybox:notlatest", "docker.io/library/busybox:notlatest", "/path:docker.io/library/busybox:notlatest"},          // Explicit tag
	{"/path:docker.io/library/busybox:latest", "docker.io/library/busybox:latest", "/path:docker.io/library/busybox:latest"}, // All implied values explicitly specified
	{"/path:example.com/ns/foo:bar", "example.com/ns/foo:bar", "/path:example.com/ns/foo:bar"},                               // All values explicitly specified
}

func TestReferenceTransport(t *testing.T) {
	ref, err := ParseReference("/tmp/archive.tar")
	require.NoError(t, err)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	for _, c := range validReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		stringRef := ref.StringWithinTransport()
		assert.Equal(t, c.stringWithinTransport, stringRef, c.input)
		// Do one more round to verify that the output can be parsed, to an equal value.
		ref2, err := Transport.ParseReference(stringRef)
		require.NoError(t, err, c.input)
		stringRef2 := ref2.StringWithinTransport()
		assert.Equal(t, stringRef, stringRef2, c.input)
	}
}

func TestReferenceDockerReference(t *testing.T) {
	for _, c := range validReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		dockerRef := ref.DockerReference()
		if c.dockerRef != "" {
			require.NotNil(t, dockerRef, c.input)
			assert.Equal(t, c.dockerRef, dockerRef.String(), c.input)
		} else {
			require.Nil(t, dockerRef, c.input)
		}
	}
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	for _, c := range validReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		assert.Equal(t, "", ref.PolicyConfigurationIdentity(), c.input)
	}
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	for _, c := range validReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		assert.Empty(t, "", ref.PolicyConfigurationNamespaces(), c.input)
	}
}

func TestReferenceNewImage(t *testing.T) {
	for _, suffix := range []string{"", ":thisisignoredbutaccepted"} {
		ref, err := ParseReference(tarFixture + suffix)
		require.NoError(t, err, suffix)
		img, err := ref.NewImage(nil)
		assert.NoError(t, err, suffix)
		defer img.Close()
	}
}

func TestReferenceNewImageSource(t *testing.T) {
	for _, suffix := range []string{"", ":thisisignoredbutaccepted"} {
		ref, err := ParseReference(tarFixture + suffix)
		require.NoError(t, err, suffix)
		src, err := ref.NewImageSource(nil, nil)
		assert.NoError(t, err, suffix)
		defer src.Close()
	}
}

func TestReferenceNewImageDestination(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "docker-archive-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ref, err := ParseReference(filepath.Join(tmpDir, "no-reference"))
	require.NoError(t, err)
	dest, err := ref.NewImageDestination(nil)
	assert.Error(t, err)

	ref, err = ParseReference(filepath.Join(tmpDir, "with-reference") + "busybox:latest")
	require.NoError(t, err)
	dest, err = ref.NewImageDestination(nil)
	assert.NoError(t, err)
	defer dest.Close()
}

func TestReferenceDeleteImage(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "docker-archive-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for i, suffix := range []string{"", ":thisisignoredbutaccepted"} {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("file%d.tar", i))
		err := ioutil.WriteFile(testFile, []byte("nonempty"), 0644)
		require.NoError(t, err, suffix)

		ref, err := ParseReference(testFile + suffix)
		require.NoError(t, err, suffix)
		err = ref.DeleteImage(nil)
		assert.Error(t, err, suffix)

		_, err = os.Lstat(testFile)
		assert.NoError(t, err, suffix)
	}
}

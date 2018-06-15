package alltransports

import (
	"testing"

	"github.com/containers/image/transports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageName(t *testing.T) {
	// This primarily tests error handling, TestImageNameHandling is a table-driven
	// test for the expected values.
	for _, name := range []string{
		"",         // Empty
		"busybox",  // No transport name
		":busybox", // Empty transport name
		"docker:",  // Empty transport reference
	} {
		_, err := ParseImageName(name)
		assert.Error(t, err, name)
	}
}

// A table-driven test summarizing the various transports' behavior.
func TestImageNameHandling(t *testing.T) {
	// Always registered transports
	for _, c := range []struct{ transport, input, roundtrip string }{
		{"dir", "/etc", "/etc"},
		{"docker", "//busybox", "//busybox:latest"},
		{"docker", "//busybox:notlatest", "//busybox:notlatest"}, // This also tests handling of multiple ":" characters
		{"docker-daemon", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		{"docker-daemon", "busybox:latest", "busybox:latest"},
		{"docker-archive", "/var/lib/oci/busybox.tar:busybox:latest", "/var/lib/oci/busybox.tar:docker.io/library/busybox:latest"},
		{"docker-archive", "busybox.tar:busybox:latest", "busybox.tar:docker.io/library/busybox:latest"},
		{"oci", "/etc:sometag", "/etc:sometag"},
		// "atomic" not tested here because it depends on per-user configuration for the default cluster.
		// "containers-storage" not tested here because it needs to initialize various directories on the fs.
	} {
		fullInput := c.transport + ":" + c.input
		ref, err := ParseImageName(fullInput)
		require.NoError(t, err, fullInput)
		s := transports.ImageName(ref)
		assert.Equal(t, c.transport+":"+c.roundtrip, s, fullInput)
	}

	// Possibly stubbed-out transports: Only verify that something is registered.
	for _, c := range []string{"ostree"} {
		transport := transports.Get(c)
		assert.NotNil(t, transport, c)
	}
}

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPathToAuth(t *testing.T) {
	uid := fmt.Sprintf("%d", os.Getuid())

	tmpDir, err := ioutil.TempDir("", "TestGetPathToAuth")
	require.NoError(t, err)

	// Environment is per-process, so this looks very unsafe; actually it seems fine because tests are not
	// run in parallel unless they opt in by calling t.Parallel().  So don’t do that.
	oldXRD, hasXRD := os.LookupEnv("XDG_RUNTIME_DIR")
	defer func() {
		if hasXRD {
			os.Setenv("XDG_RUNTIME_DIR", oldXRD)
		} else {
			os.Unsetenv("XDG_RUNTIME_DIR")
		}
	}()

	for _, c := range []struct {
		sys      *types.SystemContext
		xrd      string
		expected string
	}{
		// Default paths
		{&types.SystemContext{}, "", "/run/containers/" + uid + "/auth.json"},
		{nil, "", "/run/containers/" + uid + "/auth.json"},
		// SystemContext overrides
		{&types.SystemContext{AuthFilePath: "/absolute/path"}, "", "/absolute/path"},
		{&types.SystemContext{RootForImplicitAbsolutePaths: "/prefix"}, "", "/prefix/run/containers/" + uid + "/auth.json"},
		// XDG_RUNTIME_DIR defined
		{nil, tmpDir, tmpDir + "/containers/auth.json"},
		{nil, tmpDir + "/thisdoesnotexist", ""},
	} {
		if c.xrd != "" {
			os.Setenv("XDG_RUNTIME_DIR", c.xrd)
		} else {
			os.Unsetenv("XDG_RUNTIME_DIR")
		}
		res, err := getPathToAuth(c.sys)
		if c.expected == "" {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, c.expected, res)
		}
	}
}

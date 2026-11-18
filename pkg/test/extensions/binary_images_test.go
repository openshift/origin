package extensions

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeImagesBinary creates an executable that behaves like an extension
// binary's `images` command and prints the given output.
func writeImagesBinary(t *testing.T, name, output string) *TestBinary {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s' " + quote(output) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return &TestBinary{binaryPath: path}
}

// quote wraps s in single quotes so a shell script can printf it verbatim.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func TestBinaryListImagesKeepsRepeatedIndexes(t *testing.T) {
	binary := writeImagesBinary(t, "fake-tests-ext", `[
		{"index":0,"registry":"example.invalid/repro","name":"first","version":"v1","mapped":{"index":0,"registry":"quay.io","name":"openshift/community-e2e-images","version":"tag-a"}},
		{"index":0,"registry":"example.invalid/repro","name":"second","version":"v1"},
		{"index":2,"registry":"registry.k8s.io","name":"e2e-test-images/agnhost","version":"2.63.0"},
		{"index":2,"registry":"registry.k8s.io","name":"e2e-test-images/busybox","version":"1.37.0-1"}
	]`)

	images, err := binary.ListImages(context.Background())
	require.NoError(t, err)

	want := []extension.Image{
		{Index: 0, Registry: "example.invalid/repro", Name: "first", Version: "v1",
			Mapped: &extension.Image{Index: 0, Registry: "quay.io", Name: "openshift/community-e2e-images", Version: "tag-a"}},
		{Index: 0, Registry: "example.invalid/repro", Name: "second", Version: "v1"},
		{Index: 2, Registry: "registry.k8s.io", Name: "e2e-test-images/agnhost", Version: "2.63.0"},
		{Index: 2, Registry: "registry.k8s.io", Name: "e2e-test-images/busybox", Version: "1.37.0-1"},
	}
	assert.Equal(t, want, images)
}

func TestBinaryListImagesEmptyOutputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
	}{
		{name: "null", output: "null"},
		{name: "empty array", output: "[]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := writeImagesBinary(t, "empty-tests-ext", tc.output)
			images, err := binary.ListImages(context.Background())
			require.NoError(t, err)
			assert.Empty(t, images)
		})
	}
}

func TestBinariesListImagesKeepsRepeatedIndexesAcrossBinaries(t *testing.T) {
	first := writeImagesBinary(t, "first-tests-ext", `[
		{"index":0,"registry":"example.invalid/repro","name":"first","version":"v1"},
		{"index":2,"registry":"registry.k8s.io","name":"e2e-test-images/agnhost","version":"2.63.0"}
	]`)
	second := writeImagesBinary(t, "second-tests-ext", `[
		{"index":0,"registry":"example.invalid/repro","name":"second","version":"v1","mapped":{"index":0,"registry":"quay.io","name":"openshift/community-e2e-images","version":"tag-b"}},
		{"index":2,"registry":"registry.k8s.io","name":"e2e-test-images/busybox","version":"1.37.0-1"}
	]`)

	allImages, err := TestBinaries{first, second}.ListImages(context.Background(), 2)
	require.NoError(t, err)

	sort.Slice(allImages, func(i, j int) bool {
		return allImages[i].Name < allImages[j].Name
	})
	want := []extension.Image{
		{Index: 2, Registry: "registry.k8s.io", Name: "e2e-test-images/agnhost", Version: "2.63.0"},
		{Index: 2, Registry: "registry.k8s.io", Name: "e2e-test-images/busybox", Version: "1.37.0-1"},
		{Index: 0, Registry: "example.invalid/repro", Name: "first", Version: "v1"},
		{Index: 0, Registry: "example.invalid/repro", Name: "second", Version: "v1",
			Mapped: &extension.Image{Index: 0, Registry: "quay.io", Name: "openshift/community-e2e-images", Version: "tag-b"}},
	}
	assert.Equal(t, want, allImages)
}

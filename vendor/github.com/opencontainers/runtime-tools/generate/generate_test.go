package generate_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rfc2119 "github.com/opencontainers/runtime-tools/error"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validate"
)

// Smoke test to ensure that _at the very least_ our default configuration
// passes the validation tests. If this test fails, something is _very_ wrong
// and needs to be fixed immediately (as it will break downstreams that depend
// on us for a "sane default" and do compliance testing -- such as umoci).
func TestGenerateValid(t *testing.T) {
	bundle, err := ioutil.TempDir("", "TestGenerateValid_bundle")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(bundle)

	// Create our toy bundle.
	rootfsPath := filepath.Join(bundle, "rootfs")
	if err := os.Mkdir(rootfsPath, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(bundle, "config.json")
	g, err := generate.New("linux")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&g).SaveToFile(configPath, generate.ExportOptions{Seccomp: false}); err != nil {
		t.Fatal(err)
	}

	// Validate the bundle.
	v, err := validate.NewValidatorFromPath(bundle, true, runtime.GOOS)
	if err != nil {
		t.Errorf("unexpected NewValidatorFromPath error: %+v", err)
	}
	if err := v.CheckAll(); err != nil {
		levelErrors, err := specerror.SplitLevel(err, rfc2119.Must)
		if err != nil {
			t.Errorf("unexpected non-multierror: %+v", err)
			return
		}
		for _, e := range levelErrors.Warnings {
			t.Logf("unexpected warning: %v", e)
		}
		if err := levelErrors.Error; err != nil {
			t.Errorf("unexpected MUST error(s): %+v", err)
		}
	}
}

func TestRemoveMount(t *testing.T) {
	g, err := generate.New("linux")
	if err != nil {
		t.Fatal(err)
	}
	size := len(g.Mounts())
	g.RemoveMount("/dev/shm")
	if size-1 != len(g.Mounts()) {
		t.Errorf("Unable to remove /dev/shm from mounts")
	}
}

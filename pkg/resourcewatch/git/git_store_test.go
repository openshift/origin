package git

import (
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/origin/pkg/resourcewatch/json"
	"github.com/openshift/origin/pkg/resourcewatch/observe"
)

func TestNewGitStorage(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, path string)
		wantErrText string
	}{
		{
			name: "initializes repository",
		},
		{
			name: "opens existing repository",
			setup: func(t *testing.T, path string) {
				t.Helper()
				command := exec.Command("git", "init", path)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("failed to initialize test repository: %v: %s", err, output)
				}
			},
		},
		{
			name: "rejects invalid repository",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(path, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			wantErrText: "validating Git repository",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repository")
			if test.setup != nil {
				test.setup(t, path)
			}

			storage, err := NewGitStorage(path)
			if len(test.wantErrText) > 0 {
				if err == nil || !strings.Contains(err.Error(), test.wantErrText) {
					t.Fatalf("expected error containing %q, got %v", test.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewGitStorage returned an unexpected error: %v", err)
			}
			if storage.path != path {
				t.Fatalf("expected storage path %q, got %q", path, storage.path)
			}
		})
	}
}

func BenchmarkGitSink(b *testing.B) {
	os.Setenv("REPOSITORY_PATH", b.TempDir())

	// Don't use git configuration from the user's home directory
	os.Setenv("HOME", "")

	// Because we have no .gitconfig
	os.Setenv("GIT_COMMITTER_NAME", "run-resourcewatch")
	os.Setenv("GIT_COMMITTER_EMAIL", "ci-monitor@openshift.io")

	gitStorage, err := gitInitStorage()
	if err != nil {
		b.Fatalf("Failed to initialise git storage: %v", err)
	}

	resources, err := readTestData(b)
	if err != nil {
		b.Fatalf("Failed to read test data: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gitWrite(context.TODO(), gitStorage, resources[i])
	}
}

func readTestData(b *testing.B) ([]*observe.ResourceObservation, error) {
	file, err := os.Open("testdata/observations.json.gz")
	if err != nil {
		return nil, err
	}
	uncompressed, err := gzip.NewReader(file)
	if err != nil {
		b.Fatalf("Failed to decompress test data: %v", err)
	}

	source, err := json.Source(uncompressed)
	if err != nil {
		b.Fatalf("Failed to initialise json source: %v", err)
	}

	resourceC := make(chan *observe.ResourceObservation)

	// Run the json source. We don't need it when we exit this function because
	// we've already pulled all our test data from it.
	sourceCtx, sourceCancel := context.WithCancel(context.Background())
	defer sourceCancel()
	source(sourceCtx, logr.Discard(), resourceC)

	b.Logf("Reading %d test observations", b.N)
	resources := make([]*observe.ResourceObservation, b.N)
	for i := 0; i < b.N; i++ {
		resources[i] = <-resourceC
	}

	return resources, nil
}

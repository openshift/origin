package operator

import (
	"compress/gzip"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/openshift/origin/pkg/resourcewatch/observe"
)

func BenchmarkGitSink(b *testing.B) {
	repoDir := b.TempDir()
	os.Setenv("REPOSITORY_PATH", repoDir)

	// Because we have no .gitconfig
	os.Setenv("GIT_COMMITTER_NAME", "run-resourcewatch")
	os.Setenv("GIT_COMMITTER_EMAIL", "ci-monitor@openshift.io")

	// Don't use git configuration from the user's home directory
	os.Setenv("HOME", "")

	gitStorage, err := gitInitStorage()
	if err != nil {
		b.Fatalf("Failed to initialise git storage: %v", err)
	}

	file, err := os.Open("testdata/observations.json.gz")
	if err != nil {
		b.Fatalf("Failed to open test data: %v", err)
	}

	uncompressed, err := gzip.NewReader(file)
	if err != nil {
		b.Fatalf("Failed to decompress test data: %v", err)
	}

	source, err := jsonSource(uncompressed)
	if err != nil {
		b.Fatalf("Failed to initialise json source: %v", err)
	}

	resourceC := make(chan *observe.ResourceObservation, 1000000)
	source(b.Context(), logr.Discard(), resourceC)

	for b.Loop() {
		gitWrite(gitStorage, <-resourceC)
	}

	// gitStorage creates unmanaged go threads which can prevent us from cleaning up.
	// Until we address that, just give them a chance to finish
	time.Sleep(5 * time.Second)
}

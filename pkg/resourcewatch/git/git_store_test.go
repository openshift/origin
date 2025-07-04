package git

import (
	"compress/gzip"
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/openshift/origin/pkg/resourcewatch/json"
	"github.com/openshift/origin/pkg/resourcewatch/observe"
)

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
		gitWrite(gitStorage, resources[i])
	}
	b.StopTimer()

	// gitStorage creates unmanaged go threads which can prevent us from cleaning up.
	// Until we address that, just give them a chance to finish
	time.Sleep(5 * time.Second)
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

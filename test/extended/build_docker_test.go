// +build extended

package extended

// This extended test exercise the Docker strategy build. This test succeed when
// the Docker image is successfully built.

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/test/util"
)

func init() {
	util.RequireServer()
}

func TestDockerStrategyBuild(t *testing.T) {
	namespace := util.RandomNamespace("docker")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := util.GetBuildFixture("fixtures/docker-build.json")
	client, _ := util.GetClusterAdminClient(util.KubeConfigPath())

	repo := util.CreateSampleImageRepository(namespace)
	if repo == nil {
		t.Fatal("Failed to create ImageRepository")
	}
	defer util.DeleteSampleImageRepository(repo, namespace)

	// TODO: Tweak the selector to match the build name
	watcher, err := client.Builds(namespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	newBuild, err := client.Builds(namespace).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for event := range watcher.ResultChan() {
		build, ok := event.Object.(*buildapi.Build)
		if !ok {
			t.Fatalf("cannot convert input to Build")
		}

		if build.Name == newBuild.Name {
			switch build.Status {
			case buildapi.BuildStatusFailed, buildapi.BuildStatusError:
				t.Fatalf("Unexpected build status: ", buildapi.BuildStatusFailed)
			case buildapi.BuildStatusComplete:
				// If the Docker build strategy finishes with Complete, then this test
				// succeeded
				return
			}
		}
	}
}

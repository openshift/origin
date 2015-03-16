// +build extended

package extended

// This extended tests excercise the scenario of having the 'contextDir' set to
// directory where the application sources resides inside the repository.
// The STI strategy is used for this build and this test succeed when the Build
// completes and the resulting image is used for a Pod that replies to HTTP
// request.

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/test/util"
)

func init() {
	util.RequireServer()
}

func TestSTIContextDirBuild(t *testing.T) {
	namespace := util.RandomNamespace("contextdir")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := util.GetBuildFixture("fixtures/contextdir-build.json")
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

		// Iterate over watcher's results and search for
		// the build we just started. Also make sure that
		// the build is running, complete, or has failed
		if build.Name == newBuild.Name {
			switch build.Status {
			case buildapi.BuildStatusFailed, buildapi.BuildStatusError:
				t.Fatalf("Unexpected build status: ", buildapi.BuildStatusFailed)
			case buildapi.BuildStatusComplete:
				err := util.VerifyImage(repo, namespace, func(addr string) error {
					resp, err := http.Get("http://" + addr)
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					body, err := ioutil.ReadAll(resp.Body)
					if strings.TrimSpace(string(body)) != "success" {
						return fmt.Errorf("Expected 'success' got '%v'", body)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("The build image failed validation: %v", err)
				}
				return
			}
		}
	}
}

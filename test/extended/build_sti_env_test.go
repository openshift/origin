// +build extended

package extended

// This extended test exercise the scenario of having the '.sti/environment'
// file in the application sources that should set the defined environment
// variables for the resulting application. HTTP request is made to Pod that
// runs the output image and this HTTP request should reply the value of the
// TEST_VAR.

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

func TestSTIEnvironmentBuild(t *testing.T) {
	namespace := util.RandomNamespace("stienv")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := util.GetBuildFixture("fixtures/sti-env-build.json")
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

// +build extended

package extended

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	buildapi "github.com/openshift/origin/pkg/build/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireServer()
}

// TestSTIContextDirBuild excercises the scenario of having the 'contextDir' set to
// directory where the application sources resides inside the repository.
// The STI strategy is used for this build and this test succeed when the Build
// completes and the resulting image is used for a Pod that replies to HTTP
// request.
func TestSTIContextDirBuild(t *testing.T) {
	namespace := testutil.RandomNamespace("contextdir")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := testutil.GetBuildFixture("fixtures/contextdir-build.json")
	client, _ := testutil.GetClusterAdminClient(testutil.KubeConfigPath())

	repo := testutil.CreateSampleImageRepository(namespace)
	if repo == nil {
		t.Fatal("Failed to create ImageRepository")
	}
	defer testutil.DeleteSampleImageRepository(repo, namespace)

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
				err := testutil.VerifyImage(repo, namespace, validateContextDirImage)
				if err != nil {
					t.Fatalf("The build image failed validation: %v", err)
				}
				return
			}
		}
	}
}

// TestDockerStrategyBuild exercises the Docker strategy build. This test succeed when
// the Docker image is successfully built.
func TestDockerStrategyBuild(t *testing.T) {
	namespace := testutil.RandomNamespace("docker")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := testutil.GetBuildFixture("fixtures/docker-build.json")
	client, _ := testutil.GetClusterAdminClient(testutil.KubeConfigPath())

	repo := testutil.CreateSampleImageRepository(namespace)
	if repo == nil {
		t.Fatal("Failed to create ImageRepository")
	}
	defer testutil.DeleteSampleImageRepository(repo, namespace)

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

// TestSTIContextDirBuild exercises the scenario of having the '.sti/environment'
// file in the application sources that should set the defined environment
// variables for the resulting application. HTTP request is made to Pod that
// runs the output image and this HTTP request should reply the value of the
// TEST_VAR.
func TestSTIEnvironmentBuild(t *testing.T) {
	namespace := testutil.RandomNamespace("stienv")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := testutil.GetBuildFixture("fixtures/sti-env-build.json")
	client, _ := testutil.GetClusterAdminClient(testutil.KubeConfigPath())

	repo := testutil.CreateSampleImageRepository(namespace)
	if repo == nil {
		t.Fatal("Failed to create ImageRepository")
	}
	defer testutil.DeleteSampleImageRepository(repo, namespace)

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
				err := testutil.VerifyImage(repo, namespace, validateSTIEnvironment)
				if err != nil {
					t.Fatalf("The build image failed validation: %v", err)
				}
				return
			}
		}
	}
}

// validateSTIEnvironment verifies that the environment variable set in
// .sti/environment file is returned from the service that runs the build image
func validateSTIEnvironment(address string) error {
	resp, err := http.Get("http://" + address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "success" {
		return fmt.Errorf("Expected 'success' got '%v'", body)
	}
	return nil
}

// validateContextDirImage verifies that the image with contextDir set can
// properly start and respond to HTTP requests
func validateContextDirImage(address string) error {
	resp, err := http.Get("http://" + address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "success" {
		return fmt.Errorf("Expected 'success' got '%v'", body)
	}
	return nil
}

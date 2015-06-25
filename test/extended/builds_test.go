// +build extended

package extended

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	buildapi "github.com/openshift/origin/pkg/build/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireServer()
}

// TestPushSecretName exercises one of the complex Build scenarios, where you
// first build a Docker image using Docker build strategy, which will later be
// consumed by Custom build strategy to verify that the 'PushSecretName' (Docker
// credentials) were successfully transported to the builder. The content of the
// Secret file is verified in the end.
func TestPushSecretName(t *testing.T) {
	namespace := testutil.RandomNamespace("secret")
	client, _ := testutil.GetClusterAdminClient(testutil.KubeConfigPath())
	kclient, _ := testutil.GetClusterAdminKubeClient(testutil.KubeConfigPath())

	stream := testutil.CreateSampleImageStream(namespace)
	if stream == nil {
		t.Fatal("Failed to create ImageStream")
	}
	defer testutil.DeleteSampleImageStream(stream, namespace)

	// Create Secret with dockercfg
	secret := testutil.GetSecretFixture("fixtures/test-secret.json")
	// TODO: Why do I need to set namespace here?
	secret.Namespace = namespace
	_, err := kclient.Secrets(namespace).Create(secret)
	if err != nil {
		t.Fatalf("Failed to create Secret: %v", err)
	}

	watcher, err := client.Builds(namespace).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// First build the builder image (custom build builder)
	dockerBuild := testutil.GetBuildFixture("fixtures/test-secret-build.json")
	newDockerBuild, err := client.Builds(namespace).Create(dockerBuild)
	if err != nil {
		t.Fatalf("Unable to create Build %s: %v", dockerBuild.Name, err)
	}
	waitForComplete(newDockerBuild, watcher, t)

	// Now build the application image using custom build (run the previous image)
	// Custom build will copy the dockercfg file into the application image.
	customBuild := testutil.GetBuildFixture("fixtures/test-custom-build.json")
	imageName := fmt.Sprintf("%s/%s/%s", os.Getenv("REGISTRY_ADDR"), namespace, stream.Name)
	customBuild.Parameters.Strategy.CustomStrategy.Image = imageName
	newCustomBuild, err := client.Builds(namespace).Create(customBuild)
	if err != nil {
		t.Fatalf("Unable to create Build %s: %v", dockerBuild.Name, err)
	}
	waitForComplete(newCustomBuild, watcher, t)

	// Verify that the dockercfg file is there
	if err := testutil.VerifyImage(stream, "application", namespace, validatePushSecret); err != nil {
		t.Fatalf("Image verification failed: %v", err)
	}
}

// TestSTIEnvironmentBuild exercises the scenario where you have .sti/environment
// file in your source code repository and you use STI build strategy. In that
// case the STI build should read that file and set all environment variables
// from that file to output image.
func TestSTIEnvironmentBuild(t *testing.T) {
	namespace := testutil.RandomNamespace("stienv")
	fmt.Printf("Using '%s' namespace\n", namespace)

	build := testutil.GetBuildFixture("fixtures/test-env-build.json")
	client, _ := testutil.GetClusterAdminClient(testutil.KubeConfigPath())

	stream := testutil.CreateSampleImageStream(namespace)
	if stream == nil {
		t.Fatal("Failed to create ImageStream")
	}
	defer testutil.DeleteSampleImageStream(stream, namespace)

	// TODO: Tweak the selector to match the build name
	watcher, err := client.Builds(namespace).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	newBuild, err := client.Builds(namespace).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	waitForComplete(newBuild, watcher, t)
	if err := testutil.VerifyImage(stream, "", namespace, validateSTIEnvironment); err != nil {
		t.Fatalf("The build image failed validation: %v", err)
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

// validatePushSecret verifies that the content of the sample dockercfg is
// properly returned from the Pod running the image that contains this file.
func validatePushSecret(address string) error {
	expected := `{"https://registryhost/v1":{"auth":"secret","email":"john@doe.com"}}`
	resp, err := http.Get("http://" + address + "/SECRET_FILE")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != expected {
		return fmt.Errorf("Expected '%s' '%s'", expected, body)
	}
	return nil
}

// waitForComplete waits for the Build to finish
func waitForComplete(build *buildapi.Build, w watch.Interface, t *testing.T) {
	for event := range w.ResultChan() {
		eventBuild, ok := event.Object.(*buildapi.Build)
		if !ok {
			t.Fatalf("Cannot convert input to Build")
		}
		if build.Name != eventBuild.Name {
			continue
		}
		switch eventBuild.Status {
		case buildapi.BuildStatusFailed, buildapi.BuildStatusError:
			t.Fatalf("Unexpected status for Build %s: %v", eventBuild.Name, buildapi.BuildStatusFailed)
		case buildapi.BuildStatusComplete:
			return
		default:
			fmt.Printf("Build %s updated: %v\n", eventBuild.Name, eventBuild.Status)
		}
	}
}

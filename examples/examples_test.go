package examples

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	configapi "github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func TestExamples(t *testing.T) {
	expected := map[string]runtime.Object{

		"guestbook/template.json": &templateapi.Template{},

		"hello-openshift/hello-pod.json":     &kapi.Pod{},
		"hello-openshift/hello-project.json": &projectapi.Project{},

		"sample-app/github-webhook-example.json":           nil, // Skip.
		"sample-app/docker-registry-config.json":           &configapi.Config{},
		"sample-app/application-template-stibuild.json":    &templateapi.Template{},
		"sample-app/application-template-dockerbuild.json": &templateapi.Template{},

		"jenkins/jenkins-config.json":         &configapi.Config{},
		"jenkins/docker-registry-config.json": &configapi.Config{},
		"jenkins/application-template.json":   &templateapi.Template{},

		"../test/integration/fixtures/test-deployment-config.json": &deployapi.DeploymentConfig{},
		"../test/integration/fixtures/test-image-repository.json":  &imageapi.ImageRepository{},
		"../test/integration/fixtures/test-image.json":             &imageapi.Image{},
		"../test/integration/fixtures/test-mapping.json":           &imageapi.ImageRepositoryMapping{},
		"../test/integration/fixtures/test-route.json":             &routeapi.Route{},
		"../test/integration/fixtures/test-service.json":           &kapi.Service{},
	}

	// Add the root directory to search for files you want to test, if is not in the list below.
	rootDirs := []string{".", "../test/integration/fixtures"}
	files := []string{}

	for _, rootDir := range rootDirs {
		err := filepath.Walk(rootDir, func(path string, f os.FileInfo, err error) error {
			if filepath.Ext(path) == ".json" {
				files = append(files, path)
			}
			return err
		})

		if err != nil {
			t.Errorf("%v", err)
		}
	}

	// Check all files which are expected to be validated, each file should have
	// an existent corresponding JSON file on disk.
	for fileName := range expected {
		if exists := func() bool {
			for _, file := range files {
				if fileName == file {
					return true
				}
			}
			return false
		}(); exists == false {
			t.Errorf("No JSON file was found for the expected file: '%v'", fileName)
			continue
		}
	}

	for _, file := range files {
		expectedObject, ok := expected[file]
		if !ok {
			t.Errorf("No test case defined for example JSON file '%v'", file)
			continue
		}
		if expectedObject == nil {
			continue
		}

		jsonData, _ := ioutil.ReadFile(file)
		if err := latest.Codec.DecodeInto(jsonData, expectedObject); err != nil {
			t.Errorf("Unexpected error while decoding example JSON file '%v': %v", file, err)
		}
	}
}

func TestReadme(t *testing.T) {
	path := "../README.md"
	_, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Unable to read file: %v", err)
	}
}

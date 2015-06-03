package examples

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/validation"
	configapi "github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

type mockService struct{}

func (mockService) ListServices(kapi.Context) (*kapi.ServiceList, error) {
	return &kapi.ServiceList{}, nil
}

func walkJSONFiles(inDir string, fn func(name, path string, data []byte)) error {
	err := filepath.Walk(inDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != inDir {
			return filepath.SkipDir
		}
		name := filepath.Base(path)
		ext := filepath.Ext(name)
		if ext != "" {
			name = name[:len(name)-len(ext)]
		}
		if !(ext == ".json" || ext == ".yaml") {
			return nil
		}
		glog.Infof("testing %s", path)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		fn(name, path, data)
		return nil
	})
	return err
}

func TestExampleObjectSchemas(t *testing.T) {
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Setup(true, nil)
	cases := map[string]map[string]runtime.Object{
		"../examples/hello-openshift": {
			"hello-pod":     &kapi.Pod{},
			"hello-project": &projectapi.Project{},
		},
		"../examples/sample-app": {
			"github-webhook-example":           nil, // Skip.
			"application-template-stibuild":    &templateapi.Template{},
			"application-template-dockerbuild": &templateapi.Template{},
			"application-template-custombuild": &templateapi.Template{},
		},
		"../examples/jenkins": {
			"jenkins-config":       &configapi.Config{},
			"application-template": &templateapi.Template{},
		},
		"../examples/image-repositories": {
			"image-repositories": &imageapi.ImageRepositoryList{},
		},
		"../examples/image-streams": {
			"image-streams-centos7": &imageapi.ImageStreamList{},
			"image-streams-rhel7":   &imageapi.ImageStreamList{},
		},
		"../examples/db-templates": {
			"mysql-template":      &templateapi.Template{},
			"postgresql-template": &templateapi.Template{},
			"mongodb-template":    &templateapi.Template{},
		},
		"../test/integration/fixtures": {
			"test-deployment-config":        &deployapi.DeploymentConfig{},
			"test-image":                    &imageapi.Image{},
			"test-image-repository":         &imageapi.ImageRepository{},
			"test-image-repository-mapping": &imageapi.ImageRepositoryMapping{},
			"test-image-stream":             &imageapi.ImageStream{},
			"test-image-stream-mapping":     &imageapi.ImageStreamMapping{},
			"test-route":                    &routeapi.Route{},
			"test-service":                  &kapi.Service{},
			"test-buildcli":                 &kapi.List{},
			"test-buildcli-beta2":           &kapi.List{},
		},
		"../test/templates/fixtures": {
			"crunchydata-pod": nil, // Explicitly fails validation, but should pass transformation
			"guestbook_list":  &templateapi.Template{},
			"guestbook":       &templateapi.Template{},
		},
	}

	for path, expected := range cases {
		tested := 0
		err := walkJSONFiles(path, func(name, path string, data []byte) {
			expectedType, found := expected[name]
			if !found {
				t.Errorf("%s does not have a test case defined", path)
				return
			}
			tested += 1
			if expectedType == nil {
				t.Logf("%q is skipped", path)
				return
			}
			if err := latest.Codec.DecodeInto(data, expectedType); err != nil {
				t.Errorf("%s did not decode correctly: %v\n%s", path, err, string(data))
				return
			}
			if errors := validation.ValidateObject(expectedType); len(errors) > 0 {
				t.Errorf("%s did not validate correctly: %v", path, errors)
			}
		})
		if err != nil {
			t.Errorf("Expected no error, Got %v", err)
		}
		if tested != len(expected) {
			t.Errorf("Expected %d examples, Got %d", len(expected), tested)
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

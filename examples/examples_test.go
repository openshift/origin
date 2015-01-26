package examples

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	//"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildv "github.com/openshift/origin/pkg/build/api/validation"
	configapi "github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployv "github.com/openshift/origin/pkg/deploy/api/validation"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagev "github.com/openshift/origin/pkg/image/api/validation"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectv "github.com/openshift/origin/pkg/project/api/validation"
	routeapi "github.com/openshift/origin/pkg/route/api"
	routev "github.com/openshift/origin/pkg/route/api/validation"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templatev "github.com/openshift/origin/pkg/template/api/validation"
)

type mockService struct{}

func (mockService) ListServices(kapi.Context) (*kapi.ServiceList, error) {
	return &kapi.ServiceList{}, nil
}

func validateObject(obj runtime.Object) (errors []error) {
	ctx := kapi.NewDefaultContext()

	if m, err := meta.Accessor(obj); err == nil {
		if len(m.Namespace()) == 0 {
			m.SetNamespace(kapi.NamespaceDefault)
		}
	}

	switch t := obj.(type) {

	case *kapi.ReplicationController:
		errors = validation.ValidateReplicationController(t)
	case *kapi.Service:
		errors = validation.ValidateService(t, mockService{}, ctx)
	case *kapi.Pod:
		errors = validation.ValidatePod(t)

	case *imageapi.Image:
		errors = imagev.ValidateImage(t)
	case *imageapi.ImageRepository:
		// TODO: validate image repository
		// 	errors = imagev.ValidateImageRepository(t)
	case *imageapi.ImageRepositoryMapping:
		errors = imagev.ValidateImageRepositoryMapping(t)
	case *deployapi.DeploymentConfig:
		errors = deployv.ValidateDeploymentConfig(t)
	case *deployapi.Deployment:
		errors = deployv.ValidateDeployment(t)
	case *projectapi.Project:
		// this is a global resource that should not have a namespace
		t.Namespace = ""
		errors = projectv.ValidateProject(t)
	case *routeapi.Route:
		errors = routev.ValidateRoute(t)

	case *buildapi.BuildConfig:
		errors = buildv.ValidateBuildConfig(t)
	case *buildapi.Build:
		errors = buildv.ValidateBuild(t)

	case *templateapi.Template:
		errors = templatev.ValidateTemplate(t)
		for i := range t.Items {
			obj, err := latest.Codec.Decode(t.Items[i].RawJSON)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			errors = append(errors, validateObject(obj)...)
		}
	case *configapi.Config:
		for i := range t.Items {
			obj, err := latest.Codec.Decode(t.Items[i].RawJSON)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			errors = append(errors, validateObject(obj)...)
		}
	default:
		if list, err := runtime.ExtractList(obj); err == nil {
			for i := range list {
				errs := validateObject(list[i])
				errors = append(errors, errs...)
			}
			return
		}
		return []error{fmt.Errorf("no validation defined for %#v", obj)}
	}
	return errors
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
	kubelet.SetupCapabilities(true)
	cases := map[string]map[string]runtime.Object{
		"../examples/guestbook": {
			"template": &templateapi.Template{},
		},
		"../examples/hello-openshift": {
			"hello-pod":     &kapi.Pod{},
			"hello-project": &projectapi.Project{},
		},
		"../examples/sample-app": {
			"github-webhook-example":           nil, // Skip.
			"docker-registry-config":           &configapi.Config{},
			"docker-registry-template":         &templateapi.Template{},
			"application-template-stibuild":    &templateapi.Template{},
			"application-template-dockerbuild": &templateapi.Template{},
			"application-template-custombuild": &templateapi.Template{},
			"project":                          &projectapi.Project{},
		},
		"../examples/jenkins": {
			"jenkins-config":         &configapi.Config{},
			"docker-registry-config": &configapi.Config{},
			"application-template":   &templateapi.Template{},
		},
		"../test/integration/fixtures": {
			"test-deployment-config": &deployapi.DeploymentConfig{},
			"test-image-repository":  &imageapi.ImageRepository{},
			"test-image":             &imageapi.Image{},
			"test-mapping":           &imageapi.ImageRepositoryMapping{},
			"test-route":             &routeapi.Route{},
			"test-service":           &kapi.Service{},
		},
		"../test/templates/fixtures": {
			"crunchydata-pod": nil, // Explicitly fails validation, but should pass transformation
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
			if errors := validateObject(expectedType); len(errors) > 0 {
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

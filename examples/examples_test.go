package examples

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kvalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/kubernetes/pkg/capabilities"

	"github.com/openshift/origin/pkg/api/validation"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

type mockService struct{}

func (mockService) ListServices(apirequest.Context) (*kapi.ServiceList, error) {
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
		if ext == ".yaml" {
			data, err = yaml.ToJSON(data)
			if err != nil {
				return err
			}
		}
		fn(name, path, data)
		return nil
	})
	return err
}

func TestExampleObjectSchemas(t *testing.T) {
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Setup(true, capabilities.PrivilegedSources{}, 0)
	cases := map[string]map[string]runtime.Object{
		"../examples/wordpress/template": {
			"wordpress-mysql": &templateapi.Template{},
		},
		"../examples/hello-openshift": {
			"hello-pod":     &kapi.Pod{},
			"hello-project": &projectapi.Project{},
		},
		"../examples/sample-app": {
			"github-webhook-example":             nil, // Skip.
			"application-template-stibuild":      &templateapi.Template{},
			"application-template-dockerbuild":   &templateapi.Template{},
			"application-template-custombuild":   &templateapi.Template{},
			"application-template-pullspecbuild": &templateapi.Template{},
		},
		"../examples/jenkins": {
			"jenkins-ephemeral-template":  &templateapi.Template{},
			"jenkins-persistent-template": &templateapi.Template{},
			"application-template":        &templateapi.Template{},
		},
		"../examples/image-streams": {
			"image-streams-centos7": &imageapi.ImageStreamList{},
			"image-streams-rhel7":   &imageapi.ImageStreamList{},
		},
		"../examples/db-templates": {
			"mysql-persistent-template":      &templateapi.Template{},
			"postgresql-persistent-template": &templateapi.Template{},
			"mongodb-persistent-template":    &templateapi.Template{},
			"mariadb-persistent-template":    &templateapi.Template{},
			"redis-persistent-template":      &templateapi.Template{},
			"mysql-ephemeral-template":       &templateapi.Template{},
			"postgresql-ephemeral-template":  &templateapi.Template{},
			"mongodb-ephemeral-template":     &templateapi.Template{},
			"mariadb-ephemeral-template":     &templateapi.Template{},
			"redis-ephemeral-template":       &templateapi.Template{},
		},
		"../test/extended/testdata/ldap": {
			"ldapserver-buildconfig":         &buildapi.BuildConfig{},
			"ldapserver-deploymentconfig":    &appsapi.DeploymentConfig{},
			"ldapserver-imagestream":         &imageapi.ImageStream{},
			"ldapserver-imagestream-testenv": &imageapi.ImageStream{},
			"ldapserver-service":             &kapi.Service{},
		},
		"../test/integration/testdata": {
			// TODO fix this test to  handle json and yaml
			"project-request-template-with-quota": nil, // skip a yaml file
			"test-replication-controller":         nil, // skip &api.ReplicationController
			"test-deployment-config":              &appsapi.DeploymentConfig{},
			"test-image":                          &imageapi.Image{},
			"test-image-stream":                   &imageapi.ImageStream{},
			"test-image-stream-mapping":           nil, // skip &imageapi.ImageStreamMapping{},
			"test-route":                          &routeapi.Route{},
			"test-service":                        &kapi.Service{},
			"test-service-with-finalizer":         &kapi.Service{},
			"test-buildcli":                       &kapi.List{},
			"test-buildcli-beta2":                 &kapi.List{},
			"test-egress-network-policy":          &networkapi.EgressNetworkPolicy{},
		},
		"../test/templates/testdata": {
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
			if err := runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), data, expectedType); err != nil {
				t.Errorf("%s did not decode correctly: %v\n%s", path, err, string(data))
				return
			}

			validateObject(path, expectedType, t)

		})
		if err != nil {
			t.Errorf("Expected no error, Got %v", err)
		}
		if tested != len(expected) {
			t.Errorf("Expected %d examples, Got %d", len(expected), tested)
		}
	}
}

func validateObject(path string, obj runtime.Object, t *testing.T) {
	// if an object requires a namespace server side, be sure that it is filled in for validation
	if validation.HasObjectMeta(obj) {
		namespaceRequired, err := validation.GetRequiresNamespace(obj)
		if err != nil {
			t.Errorf("Expected no error, Got %v", err)
			return
		}

		if namespaceRequired {
			objectMeta, objectMetaErr := meta.Accessor(obj)
			if objectMetaErr != nil {
				t.Errorf("Expected no error, Got %v", objectMetaErr)
				return
			}

			objectMeta.SetNamespace(metav1.NamespaceDefault)
		}
	}

	switch typedObj := obj.(type) {
	case *kapi.Pod:
		if errors := kvalidation.ValidatePod(typedObj); len(errors) > 0 {
			t.Errorf("%s did not validate correctly: %v", path, errors)
		}

	case *kapi.Service:
		if errors := kvalidation.ValidateService(typedObj); len(errors) > 0 {
			t.Errorf("%s did not validate correctly: %v", path, errors)
		}

	case *kapi.List, *imageapi.ImageStreamList:
		if list, err := meta.ExtractList(typedObj); err == nil {
			runtime.DecodeList(list, legacyscheme.Codecs.UniversalDecoder())
			for i := range list {
				validateObject(path, list[i], t)
			}

		} else {
			t.Errorf("Expected no error, Got %v", err)

		}

	default:
		if errors := validation.Validator.Validate(obj); len(errors) > 0 {
			t.Errorf("%s with %v did not validate correctly: %v", path, reflect.TypeOf(obj), errors)
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

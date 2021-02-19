package examples

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	app "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
)

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
		klog.Infof("testing %s", path)
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
	cases := map[string]map[string]runtime.Object{
		"../examples/sample-app": {
			"github-webhook-example":             nil, // Skip.
			"application-template-stibuild":      &templatev1.Template{},
			"application-template-dockerbuild":   &templatev1.Template{},
			"application-template-pullspecbuild": &templatev1.Template{},
		},
		"../examples/jenkins": {
			"jenkins-ephemeral-template":  &templatev1.Template{},
			"jenkins-persistent-template": &templatev1.Template{},
			"application-template":        &templatev1.Template{},
		},
		"../examples/image-streams": {
			"image-streams-centos7": &imagev1.ImageStreamList{},
			"image-streams-rhel7":   &imagev1.ImageStreamList{},
		},
		"../examples/db-templates": {
			"mysql-persistent-template":      &templatev1.Template{},
			"postgresql-persistent-template": &templatev1.Template{},
			"mongodb-persistent-template":    &templatev1.Template{},
			"mariadb-persistent-template":    &templatev1.Template{},
			"redis-persistent-template":      &templatev1.Template{},
			"mysql-ephemeral-template":       &templatev1.Template{},
			"postgresql-ephemeral-template":  &templatev1.Template{},
			"mongodb-ephemeral-template":     &templatev1.Template{},
			"mariadb-ephemeral-template":     &templatev1.Template{},
			"redis-ephemeral-template":       &templatev1.Template{},
		},
		"../test/extended/testdata/ldap": {
			"ldapserver-deployment": &app.Deployment{},
			"ldapserver-config-cm":  &corev1.ConfigMap{},
			"ldapserver-scripts-cm": &corev1.ConfigMap{},
			"ldapserver-service":    &corev1.Service{},
		},
	}

	_, codecs := apitesting.SchemeForOrDie(
		appsv1.Install,
		buildv1.Install,
		imagev1.Install,
		routev1.Install,
		templatev1.Install,
		corev1.AddToScheme,

		appsv1.DeprecatedInstallWithoutGroup,
		buildv1.DeprecatedInstallWithoutGroup,
		imagev1.DeprecatedInstallWithoutGroup,
		routev1.DeprecatedInstallWithoutGroup,
		templatev1.DeprecatedInstallWithoutGroup,
	)

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
			if err := runtime.DecodeInto(codecs.UniversalDecoder(), data, expectedType); err != nil {
				t.Errorf("%s did not decode correctly: %v\n%s", path, err, string(data))
				return
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

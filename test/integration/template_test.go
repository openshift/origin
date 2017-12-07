package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/client/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTemplate(t *testing.T) {
	masterConfig, path, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	for _, version := range []schema.GroupVersion{v1.SchemeGroupVersion} {
		config, err := testutil.GetClusterAdminClientConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		config.GroupVersion = &version

		template := &templateapi.Template{
			Parameters: []templateapi.Parameter{
				{
					Name:  "NAME",
					Value: "test",
				},
			},
		}

		templateObjects := []runtime.Object{
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "${NAME}-tester",
					Namespace: "somevalue",
				},
				Spec: v1.ServiceSpec{
					ClusterIP:       "1.2.3.4",
					SessionAffinity: "some-bad-${VALUE}",
				},
			},
		}
		templateapi.AddObjectsToTemplate(template, templateObjects, v1.SchemeGroupVersion)

		templateProcessor := internalversion.NewTemplateProcessorClient(templateclient.NewForConfigOrDie(config).Template().RESTClient(), "default")
		obj, err := templateProcessor.Process(template)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obj.Objects) != 1 {
			t.Fatalf("unexpected object: %#v", obj)
		}
		if err := runtime.DecodeList(obj.Objects, unstructured.UnstructuredJSONScheme); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		svc := obj.Objects[0].(*unstructured.Unstructured).Object
		spec := svc["spec"].(map[string]interface{})
		meta := svc["metadata"].(map[string]interface{})
		// keep existing values
		if spec["clusterIP"] != "1.2.3.4" {
			t.Fatalf("unexpected object: %#v", svc)
		}
		// replace a value
		if meta["name"] != "test-tester" {
			t.Fatalf("unexpected object: %#v", svc)
		}
		// clear namespace
		if meta["namespace"] != "" {
			t.Fatalf("unexpected object: %#v", svc)
		}
		// preserve values exactly
		if spec["sessionAffinity"] != "some-bad-${VALUE}" {
			t.Fatalf("unexpected object: %#v", svc)
		}
	}
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
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		fn(name, path, data)
		return nil
	})
	return err
}

func TestTemplateTransformationFromConfig(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	walkJSONFiles("../templates/fixtures", func(name, path string, data []byte) {
		template, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), data)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", path, err)
			return
		}
		templateProcessor := internalversion.NewTemplateProcessorClient(templateclient.NewForConfigOrDie(clusterAdminClientConfig).Template().RESTClient(), "default")
		config, err := templateProcessor.Process(template.(*templateapi.Template))
		if err != nil {
			t.Errorf("%q: unexpected error: %v", path, err)
			return
		}
		if len(config.Objects) == 0 {
			t.Errorf("%q: no items in config object", path)
			return
		}
		t.Logf("tested %q", path)
	})
}

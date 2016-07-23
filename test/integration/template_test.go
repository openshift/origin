package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestTemplate(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, path, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, version := range []unversioned.GroupVersion{v1.SchemeGroupVersion} {
		config, err := testutil.GetClusterAdminClientConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		config.GroupVersion = &version
		c, err := client.New(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

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
				ObjectMeta: v1.ObjectMeta{
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

		obj, err := c.TemplateConfigs("default").Create(template)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obj.Objects) != 1 {
			t.Fatalf("unexpected object: %#v", obj)
		}
		if err := runtime.DecodeList(obj.Objects, runtime.UnstructuredJSONScheme); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		svc := obj.Objects[0].(*runtime.Unstructured).Object
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
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	walkJSONFiles("../templates/fixtures", func(name, path string, data []byte) {
		template, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), data)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", path, err)
			return
		}
		config, err := clusterAdminClient.TemplateConfigs("default").Create(template.(*templateapi.Template))
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

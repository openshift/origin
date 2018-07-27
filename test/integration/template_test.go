package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"

	templateapi "github.com/openshift/api/template/v1"
	"github.com/openshift/origin/pkg/client/templateprocessing"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestTemplate(t *testing.T) {
	masterConfig, path, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
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

		corev1Scheme := runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(corev1Scheme))
		corev1Codec := serializer.NewCodecFactory(corev1Scheme).LegacyCodec(corev1.SchemeGroupVersion)

		template.Objects = append(template.Objects, runtime.RawExtension{
			Raw: []byte(runtime.EncodeOrDie(corev1Codec, &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "${NAME}-tester",
					Namespace: "somevalue",
				},
				Spec: v1.ServiceSpec{
					ClusterIP:       "1.2.3.4",
					SessionAffinity: "some-bad-${VALUE}",
				},
			})),
		})

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		obj, err := templateprocessing.NewDynamicTemplateProcessor(dynamicClient).ProcessToList(template)
		if err != nil {
			t.Fatal(err)
		}
		if len(obj.Items) != 1 {
			t.Fatalf("unexpected object: %#v", obj)
		}
		svc := obj.Items[0].Object
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

	templatev1Scheme := runtime.NewScheme()
	utilruntime.Must(templateapi.Install(templatev1Scheme))
	templatev1Codec := serializer.NewCodecFactory(templatev1Scheme).LegacyCodec(templateapi.GroupVersion)

	walkJSONFiles("../templates/fixtures", func(name, path string, data []byte) {
		t.Logf("staring %q", path)
		template, err := runtime.Decode(templatev1Codec, data)
		if err != nil {
			t.Errorf("%q: %v", path, err)
			return
		}
		dynamicClient, err := dynamic.NewForConfig(clusterAdminClientConfig)
		if err != nil {
			t.Errorf("%q: %v", path, err)
			return
		}
		processedList, err := templateprocessing.NewDynamicTemplateProcessor(dynamicClient).ProcessToList(template.(*templateapi.Template))
		if err != nil {
			t.Errorf("%q: %v", path, err)
			return
		}
		if len(processedList.Items) == 0 {
			t.Errorf("%q: no items in config object", path)
			return
		}

	})
}

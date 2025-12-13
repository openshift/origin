package templates

import (
	"io/ioutil"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/rest"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	templateapi "github.com/openshift/api/template/v1"
	"github.com/openshift/library-go/pkg/template/templateprocessingclient"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:Templates] template-api", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("templates")

	g.It("TestTemplate [apigroup:template.openshift.io]", g.Label("Size:S"), func() {
		t := g.GinkgoT()

		for _, version := range []schema.GroupVersion{v1.SchemeGroupVersion} {
			config := rest.CopyConfig(oc.AdminConfig())
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
			obj, err := templateprocessingclient.NewDynamicTemplateProcessor(dynamicClient).ProcessToList(template)
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
			if _, ok := meta["namespace"]; ok {
				t.Fatalf("unexpected object: %#v", svc)
			}
			// preserve values exactly
			if spec["sessionAffinity"] != "some-bad-${VALUE}" {
				t.Fatalf("unexpected object: %#v", svc)
			}
		}

	})
})

var _ = g.Describe("[sig-devex][Feature:Templates] template-api", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("templates")

	g.It("TestTemplateTransformationFromConfig [apigroup:template.openshift.io]", g.Label("Size:S"), func() {
		t := g.GinkgoT()

		clusterAdminClientConfig := oc.AdminConfig()

		templateFixtures := []string{
			exutil.FixturePath("testdata", "templates", "crunchydata-pod.json"),
			exutil.FixturePath("testdata", "templates", "guestbook.json"),
			exutil.FixturePath("testdata", "templates", "guestbook_list.json"),
		}

		for _, path := range templateFixtures {
			data, err := ioutil.ReadFile(path)
			o.Expect(err).NotTo(o.HaveOccurred())

			template, err := runtime.Decode(unstructured.UnstructuredJSONScheme, data)
			if err != nil {
				t.Errorf("%q: %v", path, err)
				return
			}
			dynamicClient, err := dynamic.NewForConfig(clusterAdminClientConfig)
			if err != nil {
				t.Errorf("%q: %v", path, err)
				return
			}
			processedList, err := templateprocessingclient.NewDynamicTemplateProcessor(dynamicClient).ProcessToListFromUnstructured(template.(*unstructured.Unstructured))
			if err != nil {
				t.Errorf("%q: %v", path, err)
				return
			}
			if len(processedList.Items) == 0 {
				t.Errorf("%q: no items in config object", path)
				return
			}

		}
	})
})

package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	psapi "k8s.io/pod-security-admission/api"

	exetcd "github.com/openshift/origin/test/extended/etcd"
	exutil "github.com/openshift/origin/test/extended/util"
)

const NamespaceReplaceHereTag = "{REPLACE_WITH_NAMESPACE_HERE}"

var _ = g.Describe("[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply", func() {
	var (
		// mapper is used to translate the gvr provided by etcd
		// storage data to the gvk required to create correct resource
		// yaml.
		mapper *restmapper.DeferredDiscoveryRESTMapper
	)

	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("server-side-apply")
	oc.KubeFramework().NamespacePodSecurityLevel = psapi.LevelBaseline

	g.BeforeEach(func() {
		// Only init once per worker
		if mapper != nil {
			return
		}
		kubeClient, err := kubernetes.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		mapper = restmapper.NewDeferredDiscoveryRESTMapper(discocache.NewMemCacheClient(kubeClient.Discovery()))
	})

	storageData := exetcd.GetOpenshiftEtcdStorageData(NamespaceReplaceHereTag)
	for key := range storageData {
		gvr := key
		data := storageData[gvr]

		// Apply for core types is already well-tested, so skip
		// openshift types that are just aliases.
		aliasToCoreType := data.ExpectedGVK != nil
		if aliasToCoreType {
			continue
		}

		g.It(fmt.Sprintf("should work for %s [apigroup:%s]", gvr, gvr.Group), g.Label("Size:M"), func() {
			// create the testing namespace
			testNamespace := oc.SetupProject()

			exist, err := exutil.DoesApiResourceExist(oc.AdminConfig(), gvr.Resource, gvr.GroupVersion().String())
			o.Expect(err).NotTo(o.HaveOccurred())
			if !exist {
				e2eskipper.Skipf("Resource %s does not exist", gvr.Resource)
			}

			for _, prerequisite := range data.Prerequisites {
				// The etcd storage test for oauthclientauthorizations needs to
				// manually create a service account secret but that is not
				// necessary (or possible) when interacting with an apiserver.
				// The service account secret will be created by the controller
				// manager.
				if gvr.Resource == "oauthclientauthorizations" && prerequisite.GvrData.Resource == "secrets" {
					continue
				}
				resourceClient, unstructuredObj := createResource(oc, mapper, prerequisite.GvrData, prerequisite.Stub, testNamespace)

				// we need to wait for the tokens to appear at the SA otherwise creation
				// of the oauthclientauthorization object fails
				if gvr.Resource == "oauthclientauthorizations" && prerequisite.GvrData.Resource == "serviceaccounts" {
					waitForSATokens(oc.AdminKubeClient().CoreV1().ServiceAccounts(testNamespace), unstructuredObj.Object)
				}
				defer deleteResource(resourceClient, unstructuredObj.GetName())
			}

			resourceClient, unstructuredObj := createResource(oc, mapper, gvr, data.Stub, testNamespace)
			defer deleteResource(resourceClient, unstructuredObj.GetName())

			serializedObj, err := json.Marshal(unstructuredObj.Object)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("updating the %s via apply", unstructuredObj.GetKind()))
			obj, err := resourceClient.Patch(
				context.Background(),
				unstructuredObj.GetName(),
				types.ApplyPatchType,
				serializedObj,
				metav1.PatchOptions{
					FieldManager: "apply_test",
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking that the field managers are as expected"))
			accessor, err := meta.Accessor(obj)
			o.Expect(err).NotTo(o.HaveOccurred())
			managedFields := accessor.GetManagedFields()
			o.Expect(managedFields).NotTo(o.BeNil())
			if !findManager(managedFields, "create_test") {
				g.Fail(fmt.Sprintf("Couldn't find create_test: %v", managedFields))
			}
			if !findManager(managedFields, "apply_test") {
				g.Fail(fmt.Sprintf("Couldn't find apply_test: %v", managedFields))
			}
		})
	}
})

func findManager(managedFields []metav1.ManagedFieldsEntry, manager string) bool {
	for _, entry := range managedFields {
		if entry.Manager == manager {
			return true
		}
	}
	return false
}

func deleteResource(resourceClient dynamic.ResourceInterface, name string) {
	err := resourceClient.Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		framework.Logf("Unexpected error deleting resource: %v", err)
	}
}

func createResource(
	oc *exutil.CLI,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	gvr schema.GroupVersionResource,
	stub, testNamespace string) (
	dynamic.ResourceInterface, *unstructured.Unstructured) {

	// Discover the gvk from the gvr
	gvk, err := mapper.KindFor(gvr)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Supply a value for namespace if the scope requires
	mapping, err := mapper.RESTMapping(gvk.GroupKind())
	o.Expect(err).NotTo(o.HaveOccurred())
	namespace := ""
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		o.Expect(testNamespace).NotTo(o.HaveLen(0))
		namespace = testNamespace
	}

	// Ensure that any stub embedding the etcd test namespace
	// is updated to use local test namespace instead.
	stub = strings.Replace(stub, NamespaceReplaceHereTag, testNamespace, -1)

	// Create unstructured object from stub
	unstructuredObj := unstructured.Unstructured{}
	err = json.Unmarshal([]byte(stub), &unstructuredObj.Object)
	o.Expect(err).NotTo(o.HaveOccurred())
	unstructuredObj.SetGroupVersionKind(gvk)

	dynamicClient, err := dynamic.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	resourceClient := dynamicClient.Resource(gvr).Namespace(namespace)

	g.By(fmt.Sprintf("creating a %s", gvk.Kind))
	_, err = resourceClient.Create(context.Background(), &unstructuredObj, metav1.CreateOptions{
		FieldManager: "create_test",
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "the attempted unstructured object: %v", unstructuredObj)

	return resourceClient, &unstructuredObj
}

// waitForSATokens waits for the kube-controller-manager to populate the SA from the unstructured object
// to be populated with a secret defining its tokens
func waitForSATokens(saClient corev1client.ServiceAccountInterface, unstructuredObj map[string]interface{}) {
	name, _, err := unstructured.NestedString(unstructuredObj, "metadata", "name")
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.PollImmediate(time.Second, 15*time.Second, func() (bool, error) {
		sa, err := saClient.Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("unexpected error retrieving SA %q: %v", name, err)
			return false, nil
		}
		return len(sa.Secrets) > 0, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

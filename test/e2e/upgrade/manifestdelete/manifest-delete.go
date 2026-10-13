package manifestdelete

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	apiregistrationsclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// UpgradeTest contains artifacts used during test.
type UpgradeTest struct {
	oc     *exutil.CLI
	config *restclient.Config
}

type object struct {
	kind      string
	namespace string
	name      string
}

type unstructured struct {
	name      string
	namespace string
	group     string
	version   string
	resource  string
}

var (
	// Update this array to contain objects which should have been deleted in this release. For example:
	//	object{kind: "Namespace",
	//		namespace: "",
	//		name:      "openshift-foobar"},
	//
	deletes = []object{}

	// Update this array to contain unstructured objects which should have been deleted in this release. For example:
	//	unstructured{name: "dns",
	//		namespace: "openshift-dns-operator",
	//		group:     "monitoring.coreos.com",
	//		version:   "v1",
	//		resource:  "prometheusrules"},
	//
	unstructuredDeletes = []unstructured{{
		name:      "kube-apiserver",
		namespace: "openshift-kube-apiserver",
		group:     "monitoring.coreos.com",
		version:   "v1",
		resource:  "prometheusrules",
	}}
)

func (obj object) uniqueName() string {
	if len(obj.namespace) == 0 {
		return obj.name
	}
	return obj.namespace + "/" + obj.name
}

func (obj object) String() string {
	return fmt.Sprintf("%s %s", obj.kind, obj.uniqueName())
}

func (u unstructured) uniqueName() string {
	if len(u.namespace) == 0 {
		return u.name
	}
	return u.namespace + "/" + u.name
}

func (u unstructured) String() string {
	return fmt.Sprintf("unstructured %s/%s, %s, %s", u.group, u.version, u.resource, u.uniqueName())
}

func (UpgradeTest) Name() string { return "check-for-deletes" }
func (UpgradeTest) DisplayName() string {
	return "[bz-Cluster Version Operator] Verify object deletions after upgrade success"
}

// Setup creates artifacts to be used by Test
func (t *UpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	g.By("Setting up upgrade delete test")
	oc := exutil.NewCLIWithFramework(f)
	t.oc = oc
	config, err := framework.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())
	t.config = config
	framework.Logf("Post-upgrade delete test setup complete")
}

// Test fails if any of the resources specified above in 'deletes' and 'unstructuredDeletes'
// exist on the upgraded cluster.
func (t *UpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	if len(deletes) == 0 && len(unstructuredDeletes) == 0 {
		framework.Logf("No object deletions in this release to verify")
		return
	}

	// Block until upgrade is done
	g.By("Waiting for upgrade to finish before checking for deletes")
	<-done

	for _, r := range deletes {
		framework.Logf("Checking for object %s", r)
		var err error
		kind := strings.ToLower(r.kind)
		switch kind {
		case "configmap":
			_, err = t.oc.AdminKubeClient().CoreV1().ConfigMaps(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "namespace":
			_, err = t.oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, r.name, metav1.GetOptions{})
		case "service":
			_, err = t.oc.AdminKubeClient().CoreV1().Services(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "serviceaccount":
			_, err = t.oc.AdminKubeClient().CoreV1().ServiceAccounts(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "customobjectdefinition":
			crdClient := apiextensionsclientset.NewForConfigOrDie(t.oc.AdminConfig())
			_, err = crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, r.name, metav1.GetOptions{})
		case "apiservice":
			apiClient := apiregistrationsclientset.NewForConfigOrDie(t.oc.AdminConfig())
			_, err = apiClient.ApiregistrationV1().APIServices().Get(ctx, r.name, metav1.GetOptions{})
		case "deployment":
			_, err = t.oc.AdminKubeClient().AppsV1().Deployments(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "daemonset":
			_, err = t.oc.AdminKubeClient().AppsV1().DaemonSets(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "job":
			_, err = t.oc.AdminKubeClient().BatchV1().Jobs(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "clusterrolebinding":
			_, err = t.oc.AdminKubeClient().RbacV1().ClusterRoleBindings().Get(ctx, r.name, metav1.GetOptions{})
		case "clusterrole":
			_, err = t.oc.AdminKubeClient().RbacV1().ClusterRoles().Get(ctx, r.name, metav1.GetOptions{})
		case "rolebinding":
			_, err = t.oc.AdminKubeClient().RbacV1().RoleBindings(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "role":
			_, err = t.oc.AdminKubeClient().RbacV1().Roles(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
		case "securitycontextconstraints":
			_, err = t.oc.AdminSecurityClient().SecurityV1().SecurityContextConstraints().Get(ctx, r.name, metav1.GetOptions{})
		default:
			framework.Failf("Unrecognized object kind %s", kind)
		}
		check(r.String(), err)
		framework.Logf("Object %s deletion verified", r)
	}
	dynamicClient := dynamic.NewForConfigOrDie(t.config)
	var dynClient dynamic.ResourceInterface
	for _, u := range unstructuredDeletes {
		framework.Logf("Checking for object %s", u)
		gvr := schema.GroupVersionResource{
			Group:    u.group,
			Version:  u.version,
			Resource: u.resource,
		}
		if len(u.namespace) == 0 {
			dynClient = dynamicClient.Resource(gvr)
		} else {
			dynClient = dynamicClient.Resource(gvr).Namespace(u.namespace)
		}
		_, err := dynClient.Get(ctx, u.name, metav1.GetOptions{})
		check(u.String(), err)
		framework.Logf("Object %s deletion verified", u)
	}
	framework.Logf("All deletes verified")
}

func check(obj string, err error) {
	if err == nil {
		framework.Failf("Object %s should have been deleted but was not", obj)
	} else if !apierrors.IsNotFound(err) {
		framework.Failf("Could not check object %s, err=%v", obj, err)
	}
}

// Teardown cleans up any remaining objects.
func (t *UpgradeTest) Teardown(ctx context.Context, f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

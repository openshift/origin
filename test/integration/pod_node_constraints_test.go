package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"

	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/openshift/origin/pkg/oc/cli/admin/policy"
	pluginapi "github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

func TestPodNodeConstraintsAdmissionPluginSetNodeNameClusterAdmin(t *testing.T) {
	oclient, kclientset, fn := setupClusterAdminPodNodeConstraintsTest(t, &pluginapi.PodNodeConstraintsConfig{})
	defer fn()
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node name, cluster admin", kclientset, oclient, "nodename.example.com", nil, false)
}

func TestPodNodeConstraintsAdmissionPluginSetNodeNameNonAdmin(t *testing.T) {
	config := &pluginapi.PodNodeConstraintsConfig{}
	oclient, kclientset, fn := setupUserPodNodeConstraintsTest(t, config, "derples")
	defer fn()
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node name, regular user", kclientset, oclient, "nodename.example.com", nil, true)
}

func TestPodNodeConstraintsAdmissionPluginSetNodeSelectorClusterAdmin(t *testing.T) {
	config := &pluginapi.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: []string{"hostname"},
	}
	oclient, kclientset, fn := setupClusterAdminPodNodeConstraintsTest(t, config)
	defer fn()
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node selector, cluster admin", kclientset, oclient, "", map[string]string{"hostname": "foo"}, false)
}

func TestPodNodeConstraintsAdmissionPluginSetNodeSelectorNonAdmin(t *testing.T) {
	config := &pluginapi.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: []string{"hostname"},
	}
	oclient, kclientset, fn := setupUserPodNodeConstraintsTest(t, config, "derples")
	defer fn()
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node selector, regular user", kclientset, oclient, "", map[string]string{"hostname": "foo"}, true)
}

func setupClusterAdminPodNodeConstraintsTest(t *testing.T, pluginConfig *pluginapi.PodNodeConstraintsConfig) (appsclient.Interface, kubernetes.Interface, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	cfg := map[string]*configapi.AdmissionPluginConfig{
		"scheduling.openshift.io/PodNodeConstraints": {
			Configuration: pluginConfig,
		},
	}
	masterConfig.AdmissionConfig.PluginConfig = cfg

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	ns := &corev1.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClientset.CoreV1().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return appsclient.NewForConfigOrDie(clusterAdminClientConfig), kubeClientset, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

func setupUserPodNodeConstraintsTest(t *testing.T, pluginConfig *pluginapi.PodNodeConstraintsConfig, user string) (appsclient.Interface, kubernetes.Interface, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	cfg := map[string]*configapi.AdmissionPluginConfig{
		"scheduling.openshift.io/PodNodeConstraints": {
			Configuration: pluginConfig,
		},
	}
	masterConfig.AdmissionConfig.PluginConfig = cfg
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userkubeClientset, userClientConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, user)
	if err != nil {
		t.Fatalf("error getting user/kube client: %v", err)
	}
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting kube client: %v", err)
	}
	ns := &corev1.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClientset.CoreV1().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForServiceAccounts(kubeClientset, testutil.Namespace(), []string{"default"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	addUser := &policy.RoleModificationOptions{
		RoleName:   "admin",
		RoleKind:   "ClusterRole",
		RbacClient: rbacv1client.NewForConfigOrDie(clusterAdminClientConfig),
		Users:      []string{user},
		PrintFlags: genericclioptions.NewPrintFlags(""),
		ToPrinter:  func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addUser.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return appsclient.NewForConfigOrDie(userClientConfig), userkubeClientset, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

func testPodNodeConstraintsPodSpec(nodeName string, nodeSelector map[string]string) corev1.PodSpec {
	spec := corev1.PodSpec{}
	spec.RestartPolicy = corev1.RestartPolicyAlways
	spec.NodeName = nodeName
	spec.NodeSelector = nodeSelector
	spec.Containers = []corev1.Container{
		{
			Name:  "container",
			Image: "test/image",
		},
	}
	return spec
}

func testPodNodeConstraintsPod(nodeName string, nodeSelector map[string]string) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = "testpod"
	pod.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	return pod
}

// testPodNodeConstraintsObjectCreationWithPodTemplate attempts to create different object types that contain pod templates
// using the passed in nodeName and nodeSelector. It will use the expectError flag to determine if an error should be returned or not
func testPodNodeConstraintsObjectCreationWithPodTemplate(t *testing.T, name string, kclientset kubernetes.Interface, appsClient appsclient.Interface, nodeName string, nodeSelector map[string]string, expectError bool) {
	checkForbiddenErr := func(objType string, err error) {
		if err == nil && expectError {
			t.Errorf("%s (%s): expected forbidden error but did not receive one", name, objType)
			return
		}
		if err != nil && !expectError {
			t.Errorf("%s (%s): got error but did not expect one: %v", name, objType, err)
			return
		}
		if err != nil && expectError && !kapierrors.IsForbidden(err) {
			t.Errorf("%s (%s): did not get an expected forbidden error: %v", name, objType, err)
			return
		}
	}

	// Pod
	pod := testPodNodeConstraintsPod(nodeName, nodeSelector)
	_, err := kclientset.CoreV1().Pods(testutil.Namespace()).Create(pod)
	checkForbiddenErr("pod", err)
}

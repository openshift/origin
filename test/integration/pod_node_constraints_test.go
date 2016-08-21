package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	pluginapi "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPodNodeConstraintsAdmissionPluginSetNodeNameClusterAdmin(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	oclient, kclient := setupClusterAdminPodNodeConstraintsTest(t, &pluginapi.PodNodeConstraintsConfig{})
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node name, cluster admin", kclient, oclient, "nodename.example.com", nil, false)
}

func TestPodNodeConstraintsAdmissionPluginSetNodeNameNonAdmin(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	config := &pluginapi.PodNodeConstraintsConfig{}
	oclient, kclient := setupUserPodNodeConstraintsTest(t, config, "derples")
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node name, regular user", kclient, oclient, "nodename.example.com", nil, true)
}

func TestPodNodeConstraintsAdmissionPluginSetNodeSelectorClusterAdmin(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	config := &pluginapi.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: []string{"hostname"},
	}
	oclient, kclient := setupClusterAdminPodNodeConstraintsTest(t, config)
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node selector, cluster admin", kclient, oclient, "", map[string]string{"hostname": "foo"}, false)
}

func TestPodNodeConstraintsAdmissionPluginSetNodeSelectorNonAdmin(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	config := &pluginapi.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: []string{"hostname"},
	}
	oclient, kclient := setupUserPodNodeConstraintsTest(t, config, "derples")
	testPodNodeConstraintsObjectCreationWithPodTemplate(t, "set node selector, regular user", kclient, oclient, "", map[string]string{"hostname": "foo"}, true)
}

func setupClusterAdminPodNodeConstraintsTest(t *testing.T, pluginConfig *pluginapi.PodNodeConstraintsConfig) (*client.Client, *kclient.Client) {
	testutil.RequireEtcd(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	cfg := map[string]configapi.AdmissionPluginConfig{
		"PodNodeConstraints": {
			Configuration: pluginConfig,
		},
	}
	masterConfig.AdmissionConfig.PluginConfig = cfg
	masterConfig.KubernetesMasterConfig.AdmissionConfig.PluginConfig = cfg

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	openShiftClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	ns := &kapi.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClient.Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClient, testutil.Namespace()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return openShiftClient, kubeClient
}

func setupUserPodNodeConstraintsTest(t *testing.T, pluginConfig *pluginapi.PodNodeConstraintsConfig, user string) (*client.Client, *kclient.Client) {
	testutil.RequireEtcd(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	cfg := map[string]configapi.AdmissionPluginConfig{
		"PodNodeConstraints": {
			Configuration: pluginConfig,
		},
	}
	masterConfig.AdmissionConfig.PluginConfig = cfg
	masterConfig.KubernetesMasterConfig.AdmissionConfig.PluginConfig = cfg
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userClient, userkubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, user)
	if err != nil {
		t.Fatalf("error getting user/kube client: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting kube client: %v", err)
	}
	ns := &kapi.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClient.Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForServiceAccounts(kubeClient, testutil.Namespace(), []string{bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	addUser := &policy.RoleModificationOptions{
		RoleNamespace:       ns.Name,
		RoleName:            bootstrappolicy.AdminRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(clusterAdminClient),
		Users:               []string{user},
	}
	if err := addUser.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return userClient, userkubeClient
}

func testPodNodeConstraintsPodSpec(nodeName string, nodeSelector map[string]string) kapi.PodSpec {
	spec := kapi.PodSpec{}
	spec.RestartPolicy = kapi.RestartPolicyAlways
	spec.NodeName = nodeName
	spec.NodeSelector = nodeSelector
	spec.Containers = []kapi.Container{
		{
			Name:  "container",
			Image: "test/image",
		},
	}
	return spec
}

func testPodNodeConstraintsPod(nodeName string, nodeSelector map[string]string) *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Name = "testpod"
	pod.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	return pod
}

func testPodNodeConstraintsReplicationController(nodeName string, nodeSelector map[string]string) *kapi.ReplicationController {
	rc := &kapi.ReplicationController{}
	rc.Name = "testrc"
	rc.Spec.Replicas = 1
	rc.Spec.Selector = map[string]string{"foo": "bar"}
	rc.Spec.Template = &kapi.PodTemplateSpec{}
	rc.Spec.Template.Labels = map[string]string{"foo": "bar"}
	rc.Spec.Template.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	return rc
}

func testPodNodeConstraintsDeployment(nodeName string, nodeSelector map[string]string) *extensions.Deployment {
	d := &extensions.Deployment{}
	d.Name = "testdeployment"
	d.Spec.Replicas = 1
	d.Spec.Template.Labels = map[string]string{"foo": "bar"}
	d.Spec.Template.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	d.Spec.Selector = &unversioned.LabelSelector{
		MatchLabels: map[string]string{"foo": "bar"},
	}
	return d
}

func testPodNodeConstraintsReplicaSet(nodeName string, nodeSelector map[string]string) *extensions.ReplicaSet {
	rs := &extensions.ReplicaSet{}
	rs.Name = "testrs"
	rs.Spec.Replicas = 1
	rs.Spec.Template = kapi.PodTemplateSpec{}
	rs.Spec.Template.Labels = map[string]string{"foo": "bar"}
	rs.Spec.Template.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	rs.Spec.Selector = &unversioned.LabelSelector{
		MatchLabels: map[string]string{"foo": "bar"},
	}
	return rs
}

func testPodNodeConstraintsJob(nodeName string, nodeSelector map[string]string) *batch.Job {
	job := &batch.Job{}
	job.Name = "testjob"
	job.Spec.Template.Labels = map[string]string{"foo": "bar"}
	job.Spec.Template.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	job.Spec.Template.Spec.RestartPolicy = kapi.RestartPolicyNever
	// Matching selector is now generated automatically
	// job.Spec.Selector = ...
	return job
}

func testPodNodeConstraintsDeploymentConfig(nodeName string, nodeSelector map[string]string) *deployapi.DeploymentConfig {
	dc := &deployapi.DeploymentConfig{}
	dc.Name = "testdc"
	dc.Spec.Replicas = 1
	dc.Spec.Template = &kapi.PodTemplateSpec{}
	dc.Spec.Template.Labels = map[string]string{"foo": "bar"}
	dc.Spec.Template.Spec = testPodNodeConstraintsPodSpec(nodeName, nodeSelector)
	dc.Spec.Selector = map[string]string{"foo": "bar"}
	return dc
}

// testPodNodeConstraintsObjectCreationWithPodTemplate attemps to create different object types that contain pod templates
// using the passed in nodeName and nodeSelector. It will use the expectError flag to determine if an error should be returned or not
func testPodNodeConstraintsObjectCreationWithPodTemplate(t *testing.T, name string, kclient kclient.Interface, client client.Interface, nodeName string, nodeSelector map[string]string, expectError bool) {

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
	_, err := kclient.Pods(testutil.Namespace()).Create(pod)
	checkForbiddenErr("pod", err)

	// ReplicationController
	rc := testPodNodeConstraintsReplicationController(nodeName, nodeSelector)
	_, err = kclient.ReplicationControllers(testutil.Namespace()).Create(rc)
	checkForbiddenErr("rc", err)

	// TODO: Enable when the deployments endpoint is supported in Origin
	// Deployment
	// d := testPodNodeConstraintsDeployment(nodeName, nodeSelector)
	// _, err = kclient.Extensions().Deployments(testutil.Namespace()).Create(d)
	// checkForbiddenErr("deployment", err)

	// ReplicaSet
	rs := testPodNodeConstraintsReplicaSet(nodeName, nodeSelector)
	_, err = kclient.Extensions().ReplicaSets(testutil.Namespace()).Create(rs)
	checkForbiddenErr("replicaset", err)

	// Job
	job := testPodNodeConstraintsJob(nodeName, nodeSelector)
	_, err = kclient.Extensions().Jobs(testutil.Namespace()).Create(job)
	checkForbiddenErr("job", err)

	// DeploymentConfig
	dc := testPodNodeConstraintsDeploymentConfig(nodeName, nodeSelector)
	_, err = client.DeploymentConfigs(testutil.Namespace()).Create(dc)
	checkForbiddenErr("dc", err)
}

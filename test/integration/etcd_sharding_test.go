// +build integration,etcd

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"

	kapi "k8s.io/kubernetes/pkg/api"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/cmd/server/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestEtcdSharding(t *testing.T) {
	// Start a separate etcd server
	kubeEtcdServer := etcdtesting.NewEtcdTestClientServer(t)
	defer kubeEtcdServer.Terminate(t)
	kubeEtcdClient := etcd.NewClient(kubeEtcdServer.ClientURLs.StringSlice())

	// Generate openshift configuration
	openShiftMasterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Configure openshift to write kubernetes data to a separate etcd
	openShiftMasterConfig.KubernetesMasterConfig.EtcdClientInfo = &api.EtcdConnectionInfo{
		URLs: kubeEtcdServer.ClientURLs.StringSlice(),
	}

	// Start the server
	openShiftOptions := testserver.TestOptions{DeleteAllEtcdKeys: true, EnableControllers: true}
	adminConfigFile, err := testserver.StartConfiguredMasterWithOptions(openShiftMasterConfig, openShiftOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	adminConfig, err := testutil.GetClusterAdminClientConfig(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create an openshift client
	osClient, err := testutil.GetClusterAdminClient(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create an etcd client to talk to the openshift etcd
	osEtcdClient := etcd.NewClient(openShiftMasterConfig.EtcdClientInfo.URLs)
	if osEtcdClient == nil {
		t.Fatalf("failed to connect to openshift etcd: %v", err)
	}

	// Create a project
	project := &projectapi.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "shard-project",
		},
	}
	projectResult, err := osClient.Projects().Create(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Login as a user
	loginOptions := newLoginOptions(adminConfig.Host, "shard", "shard", true)
	if err := loginOptions.GatherInfo(); err != nil {
		t.Fatalf("error trying to determine server info: %v", err)
	}
	if projectResult.Status.Phase != kapi.NamespaceActive {
		t.Fatalf("project status is: %s", projectResult.Status.Phase)
	}

	// Create a pod
	pod := &kapi.Pod{}
	pod.Name = "shard-pod"
	pod.Namespace = kapi.NamespaceDefault

	container := kapi.Container{}
	container.Name = "shard-container"
	container.Image = "openshift/hello-openshift"
	pod.Spec.Containers = []kapi.Container{container}

	kubeClient, err := testutil.GetClusterAdminKubeClient(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = wait.Poll(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := kubeClient.Pods(kapi.NamespaceDefault).Create(pod); err != nil {
			if strings.Contains(err.Error(), "no API token found for service account") {
				return true, nil
			}

			t.Log(err)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("error submitting a pod: %v", err)
	}

	// Verify data is placed in the correct etcd instances
	kubeStoragePrefix := openShiftMasterConfig.EtcdStorageConfig.KubernetesStoragePrefix
	osStoragePrefix := openShiftMasterConfig.EtcdStorageConfig.OpenShiftStoragePrefix
	_, err = kubeEtcdClient.Get(fmt.Sprintf("/%s", kubeStoragePrefix), false, false)
	if err != nil {
		t.Fatalf("failed to find kuberenetes prefix '%s' in kubernetes etcd instance: %v", kubeStoragePrefix, err)
	}
	_, err = kubeEtcdClient.Get(fmt.Sprintf("/%s", osStoragePrefix), false, false)
	if err == nil {
		t.Fatalf("found openshift prefix '%s' in kubernetes etcd instance: %v", osStoragePrefix, err)
	}
	_, err = osEtcdClient.Get(fmt.Sprintf("/%s", osStoragePrefix), false, false)
	if err != nil {
		t.Fatalf("failed to find openshift prefix '%s' in openshift etcd instance: %v", osStoragePrefix, err)
	}
	_, err = osEtcdClient.Get(fmt.Sprintf("/%s", kubeStoragePrefix), false, false)
	if err == nil {
		t.Fatalf("found kuberenetes prefix '%s' in openshift etcd instance: %v", kubeStoragePrefix, err)
	}
}

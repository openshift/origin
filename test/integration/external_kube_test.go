// +build integration,etcd

package integration

import (
	"io"
	"net/url"
	"os"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/probe"
	httpprobe "k8s.io/kubernetes/pkg/probe/http"
	"k8s.io/kubernetes/pkg/watch"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
)

func TestExternalKube(t *testing.T) {
	// Start one OpenShift master as "cluster1" to play the external kube server
	cluster1MasterConfig, cluster1AdminConfigFile, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cluster1AdminKubeClient, err := testutil.GetClusterAdminKubeClient(cluster1AdminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Copy admin.kubeconfig with a name-change top stop from over-writing it later
	persistentCluster1AdminConfigFile := cluster1AdminConfigFile + "old"
	err = copyFile(cluster1AdminConfigFile, persistentCluster1AdminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set up cluster 2 to run against cluster 1 as external kubernetes
	cluster2MasterConfig, err := testutil.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Don't start kubernetes in process
	cluster2MasterConfig.KubernetesMasterConfig = nil
	// Connect to cluster1 using the service account credentials
	cluster2MasterConfig.MasterClients.ExternalKubernetesKubeConfig = persistentCluster1AdminConfigFile
	// Don't start etcd
	cluster2MasterConfig.EtcdConfig = nil
	// Use the same credentials as cluster1 to connect to existing etcd
	cluster2MasterConfig.EtcdClientInfo = cluster1MasterConfig.EtcdClientInfo
	// Set a custom etcd prefix to make sure data is getting sent to cluster1
	cluster2MasterConfig.EtcdStorageConfig.KubernetesStoragePrefix += "2"
	cluster2MasterConfig.EtcdStorageConfig.OpenShiftStoragePrefix += "2"
	// Don't manage any names in cluster2
	cluster2MasterConfig.ServiceAccountConfig.ManagedNames = []string{}
	// Don't create any service account tokens in cluster2
	cluster2MasterConfig.ServiceAccountConfig.PrivateKeyFile = ""
	// Use the same public keys to validate tokens as cluster1
	cluster2MasterConfig.ServiceAccountConfig.PublicKeyFiles = cluster1MasterConfig.ServiceAccountConfig.PublicKeyFiles
	// Don't run controllers in the second cluster
	cluster2MasterConfig.PauseControllers = true
	// don't try to start second dns server
	cluster2MasterConfig.DNSConfig = nil

	// Start cluster 2 (without clearing etcd) and get admin client configs and clients
	cluster2Options := testutil.TestOptions{DeleteAllEtcdKeys: false}
	cluster2AdminConfigFile, err := testutil.StartConfiguredMasterWithOptions(cluster2MasterConfig, cluster2Options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cluster2AdminKubeClient, err := testutil.GetClusterAdminKubeClient(cluster2AdminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	healthzProxyTest(cluster2MasterConfig, t)

	watchProxyTest(cluster1AdminKubeClient, cluster2AdminKubeClient, t)

}

func healthzProxyTest(masterConfig *configapi.MasterConfig, t *testing.T) {
	// Ping the healthz endpoint on the second OpenShift cluster
	url, err := url.Parse(masterConfig.MasterPublicURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	url.Path = "/healthz"
	response, body, err := httpprobe.New().Probe(url, 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != probe.Success {
		t.Fatalf("Server reported unhealthy: %v", body)
	}
}

func watchProxyTest(cluster1AdminKubeClient, cluster2AdminKubeClient *kclient.Client, t *testing.T) {
	// list namespaces in order to determine correct resourceVersion
	namespaces, err := cluster1AdminKubeClient.Namespaces().List(labels.Everything(), fields.Everything())

	// open a watch on Cluster 2 for namespaces starting with latest resourceVersion
	namespaceWatch, err := cluster2AdminKubeClient.Namespaces().Watch(labels.Everything(), fields.Everything(), namespaces.ResourceVersion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer namespaceWatch.Stop()

	// add namespace in Cluster 2
	namespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: "test-namespace"},
	}
	createdNamespace, err := cluster2AdminKubeClient.Namespaces().Create(namespace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// consume watch output and record it if it's the event we want to see
	select {
	case e := <-namespaceWatch.ResultChan():
		// check that the watch shows the new namespace
		if e.Type != watch.Added {
			t.Fatalf("expected an Added event but got: %v", e)
		}
		addedNamespace, ok := e.Object.(*kapi.Namespace)
		if !ok {
			t.Fatalf("unexpected cast error from event Object to Namespace")
		}
		if addedNamespace.ObjectMeta.Name != createdNamespace.Name {
			t.Fatalf("namespace returned from Watch is not the same ast that created: got %v, wanted %v", createdNamespace, addedNamespace)
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for watch")
	}
}

func copyFile(oldFile, newFile string) (err error) {
	in, err := os.Open(oldFile)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(newFile)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if cerr == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

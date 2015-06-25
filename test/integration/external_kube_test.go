// +build integration,!no-etcd

package integration

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	testutil "github.com/openshift/origin/test/util"
)

func TestExternalKube(t *testing.T) {
	// Start one OpenShift master as "cluster1" to play the external kube server
	cluster1MasterConfig, cluster1AdminConfigFile, err := testutil.StartTestMaster()
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

	// Start cluster 2 (without clearing etcd) and get admin client configs and clients
	cluster2Options := testutil.TestOptions{DeleteAllEtcdKeys: false}
	_, err = testutil.StartConfiguredMasterWithOptions(cluster2MasterConfig, cluster2Options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ping the healthz endpoint on the second OpenShift cluster
	url, err := url.Parse(cluster2MasterConfig.MasterPublicURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	url.Path = "/healthz"
	response, err := doHTTPSProbe(url, 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only valid "healthy" response from server is 200 - OK
	if response.StatusCode != http.StatusOK {
		t.Fatalf("OpenShift reported unhealthy: %v", response)
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

// TODO(skuznets): Use Kube HTTPSProbe once HTTPS support is added and OpenShift GoDeps are updated
func doHTTPSProbe(url *url.URL, timeout time.Duration) (result *http.Response, err error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Timeout: timeout, Transport: transport}
	result, err = client.Get(url.String())
	return
}

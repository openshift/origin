package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

const (
	clusterNetworkCIDR = "10.128.0.0/14"
	serviceNetworkCIDR = "172.30.0.0/16"
)

var exampleAddresses = map[string]string{
	"cluster":  "10.128.0.2",
	"service":  "172.30.0.2",
	"external": "1.2.3.4",
}

func testOne(t *testing.T, client kubernetes.Interface, namespace, addrType string, success bool) *corev1.Endpoints {
	testEndpoint := &corev1.Endpoints{}
	testEndpoint.GenerateName = "test"
	testEndpoint.Subsets = []corev1.EndpointSubset{
		{
			Addresses: []corev1.EndpointAddress{
				{
					IP: exampleAddresses[addrType],
				},
			},
			Ports: []corev1.EndpointPort{
				{
					Port:     9999,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	ep, err := client.CoreV1().Endpoints(namespace).Create(testEndpoint)
	if err != nil && success {
		t.Fatalf("unexpected error creating %s network endpoint: %v", addrType, err)
	} else if err == nil && !success {
		t.Fatalf("unexpected success creating %s network endpoint", addrType)
	}
	return ep
}

func TestEndpointAdmission(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterNetworkConfig := []configapi.ClusterNetworkEntry{
		{
			CIDR: clusterNetworkCIDR,
		},
	}
	masterConfig.NetworkConfig.ClusterNetworks = clusterNetworkConfig
	masterConfig.NetworkConfig.ServiceNetworkCIDR = serviceNetworkCIDR

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting kube client: %v", err)
	}
	clientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	// Cluster admin
	testOne(t, clusterAdminKubeClient, "default", "cluster", true)
	testOne(t, clusterAdminKubeClient, "default", "service", true)
	testOne(t, clusterAdminKubeClient, "default", "external", true)

	// Endpoint controller service account
	serviceAccountClient, _, err := testutil.GetClientForServiceAccount(clusterAdminKubeClient, *clientConfig, "kube-system", "endpoint-controller")
	if err != nil {
		t.Fatalf("error getting endpoint controller service account: %v", err)
	}
	testOne(t, serviceAccountClient, "default", "cluster", true)
	testOne(t, serviceAccountClient, "default", "service", true)
	testOne(t, serviceAccountClient, "default", "external", true)

	// Project admin
	_, _, err = testserver.CreateNewProject(clientConfig, "myproject", "myadmin")
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}
	projectAdminClient, _, err := testutil.GetClientForUser(clientConfig, "myadmin")
	if err != nil {
		t.Fatalf("error getting project admin client: %v", err)
	}

	testOne(t, projectAdminClient, "myproject", "cluster", false)
	testOne(t, projectAdminClient, "myproject", "service", false)
	testOne(t, projectAdminClient, "myproject", "external", true)

	// User without restricted endpoint permission can't modify IPs but can still do other modifications
	ep := testOne(t, clusterAdminKubeClient, "myproject", "cluster", true)
	ep.Annotations = map[string]string{"foo": "bar"}
	ep, err = projectAdminClient.CoreV1().Endpoints("myproject").Update(ep)
	if err != nil {
		t.Fatalf("unexpected error updating endpoint annotation: %v", err)
	}
	ep.Subsets[0].Addresses[0].IP = exampleAddresses["service"]
	ep, err = projectAdminClient.CoreV1().Endpoints("myproject").Update(ep)
	if err == nil {
		t.Fatalf("unexpected success modifying endpoint")
	}
}

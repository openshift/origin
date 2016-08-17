package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
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

func testOne(t *testing.T, client *kclient.Client, namespace, addrType string, success bool) *kapi.Endpoints {
	testEndpoint := &kapi.Endpoints{}
	testEndpoint.GenerateName = "test"
	testEndpoint.Subsets = []kapi.EndpointSubset{
		{
			Addresses: []kapi.EndpointAddress{
				{
					IP: exampleAddresses[addrType],
				},
			},
			Ports: []kapi.EndpointPort{
				{
					Port:     9999,
					Protocol: kapi.ProtocolTCP,
				},
			},
		},
	}

	ep, err := client.Endpoints(namespace).Create(testEndpoint)
	if err != nil && success {
		t.Fatalf("unexpected error creating %s network endpoint: %v", addrType, err)
	} else if err == nil && !success {
		t.Fatalf("unexpected success creating %s network endpoint", addrType)
	}
	return ep
}

func TestEndpointAdmission(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.KubernetesMasterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
		serviceadmit.RestrictedEndpointsPluginName: {
			Configuration: &configapi.DefaultAdmissionConfig{},
		},
	}
	masterConfig.NetworkConfig.ClusterNetworkCIDR = clusterNetworkCIDR
	masterConfig.NetworkConfig.ServiceNetworkCIDR = serviceNetworkCIDR

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting kube client: %v", err)
	}
	clusterAdminOSClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
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
	_, serviceAccountClient, _, err := testutil.GetClientForServiceAccount(clusterAdminKubeClient, *clientConfig, bootstrappolicy.DefaultOpenShiftInfraNamespace, bootstrappolicy.InfraEndpointControllerServiceAccountName)
	if err != nil {
		t.Fatalf("error getting endpoint controller service account: %v", err)
	}
	testOne(t, serviceAccountClient, "default", "cluster", true)
	testOne(t, serviceAccountClient, "default", "service", true)
	testOne(t, serviceAccountClient, "default", "external", true)

	// Project admin
	_, err = testserver.CreateNewProject(clusterAdminOSClient, *clientConfig, "myproject", "myadmin")
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}
	_, projectAdminClient, _, err := testutil.GetClientForUser(*clientConfig, "myadmin")
	if err != nil {
		t.Fatalf("error getting project admin client: %v", err)
	}

	testOne(t, projectAdminClient, "myproject", "cluster", false)
	testOne(t, projectAdminClient, "myproject", "service", false)
	testOne(t, projectAdminClient, "myproject", "external", true)

	// User without restricted endpoint permission can't modify IPs but can still do other modifications
	ep := testOne(t, clusterAdminKubeClient, "myproject", "cluster", true)
	ep.Annotations = map[string]string{"foo": "bar"}
	ep, err = projectAdminClient.Endpoints("myproject").Update(ep)
	if err != nil {
		t.Fatalf("unexpected error updating endpoint annotation: %v", err)
	}
	ep.Subsets[0].Addresses[0].IP = exampleAddresses["service"]
	ep, err = projectAdminClient.Endpoints("myproject").Update(ep)
	if err == nil {
		t.Fatalf("unexpected success modifying endpoint")
	}
}

package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestAggregator(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	// Set up the aggregator ca and proxy cert
	caDir, err := ioutil.TempDir("", "aggregator-ca")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Remove(caDir)
	}()
	signerOptions := &admin.CreateSignerCertOptions{
		Name:       "aggregator-proxy-ca",
		CertFile:   filepath.Join(caDir, "aggregator-proxy-ca.crt"),
		KeyFile:    filepath.Join(caDir, "aggregator-proxy-ca.key"),
		SerialFile: filepath.Join(caDir, "aggregator-proxy-ca.serial"),
		Output:     ioutil.Discard,
	}
	if _, err := signerOptions.CreateSignerCert(); err != nil {
		t.Fatal(err)
	}
	proxyClientOptions := &admin.CreateClientCertOptions{
		SignerCertOptions: &admin.SignerCertOptions{
			CertFile:   signerOptions.CertFile,
			KeyFile:    signerOptions.KeyFile,
			SerialFile: signerOptions.SerialFile,
		},
		CertFile: filepath.Join(caDir, "aggregator-proxy.crt"),
		KeyFile:  filepath.Join(caDir, "aggregator-proxy.key"),
		User:     "aggregator-proxy",
	}
	if _, err := proxyClientOptions.CreateClientCert(); err != nil {
		t.Fatal(err)
	}

	// Configure the aggregator and auth config
	masterConfig.AggregatorConfig.ProxyClientInfo.CertFile = proxyClientOptions.CertFile
	masterConfig.AggregatorConfig.ProxyClientInfo.KeyFile = proxyClientOptions.KeyFile
	masterConfig.AuthConfig.RequestHeader = &configapi.RequestHeaderAuthenticationOptions{
		ClientCA:            signerOptions.CertFile,
		ClientCommonNames:   []string{proxyClientOptions.User},
		UsernameHeaders:     []string{"X-Remote-User"},
		GroupHeaders:        []string{"X-Remote-Group"},
		ExtraHeaderPrefixes: []string{"X-Remote-Extra-"},
	}

	// Get clients
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	openshiftProjectClient, err := projectclientset.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatal(err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	apiregistrationClient, err := apiregistrationclientset.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Get resources
	// Kube resource
	if _, err := kubeClient.Core().Namespaces().Get("default", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	// Legacy openshift resource
	if _, err := openshiftProjectClient.Project().Projects().Get("default", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	// Groupified openshift resource
	if _, err := openshiftProjectClient.Project().Projects().Get("default", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	// Get aggregator resources
	// Legacy group
	if _, err := apiregistrationClient.ApiregistrationV1beta1().APIServices().Get("v1.", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	// Openshift group
	if _, err := apiregistrationClient.ApiregistrationV1beta1().APIServices().Get("v1.project.openshift.io", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	// Kube group
	if _, err := apiregistrationClient.ApiregistrationV1beta1().APIServices().Get("v1beta1.rbac.authorization.k8s.io", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
}

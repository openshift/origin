package api

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

func GetKubeClient(kubeConfigFile string) (*kclient.Client, *kclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{CommandLinePath: kubeConfigFile}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	kubeClient, err := kclient.New(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return kubeClient, kubeConfig, nil
}

func GetOpenShiftClient(kubeConfigFile string) (*client.Client, *kclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{CommandLinePath: kubeConfigFile}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	openshiftClient, err := client.New(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return openshiftClient, kubeConfig, nil
}

func UseTLS(servingInfo ServingInfo) bool {
	return len(servingInfo.ServerCert.CertFile) > 0
}

// GetAPIClientCertCAPool returns the cert pool used to validate client certificates to the API server
func GetAPIClientCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	certs, err := getAPIClientCertCAs(options)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	for _, root := range certs {
		roots.AddCert(root)
	}
	return roots, nil
}

// GetClientCertCAPool returns a cert pool containing all client CAs that could be presented (union of API and OAuth)
func GetClientCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	roots := x509.NewCertPool()

	// Add CAs for OAuth
	certs, err := getOAuthClientCertCAs(options)
	if err != nil {
		return nil, err
	}
	for _, root := range certs {
		roots.AddCert(root)
	}

	// Add CAs for API
	certs, err = getAPIClientCertCAs(options)
	if err != nil {
		return nil, err
	}
	for _, root := range certs {
		roots.AddCert(root)
	}

	return roots, nil
}

// GetAPIServerCertCAPool returns the cert pool containing the roots for the API server cert
func GetAPIServerCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	caRoots, err := crypto.GetTLSCARoots(options.ServingInfo.ClientCA)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	for _, root := range caRoots.Roots {
		roots.AddCert(root)
	}
	return roots, nil
}

func getOAuthClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	caFile := options.OAuthConfig.ProxyCA
	if len(caFile) == 0 {
		return nil, nil
	}
	caPEMBlock, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	certs, err := crypto.CertsFromPEM(caPEMBlock)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s", caFile, err)
	}
	return certs, nil
}

func getAPIClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	apiClientCertCAs, err := crypto.GetTLSCARoots(options.ServingInfo.ClientCA)
	if err != nil {
		return nil, err
	}

	return apiClientCertCAs.Roots, nil
}

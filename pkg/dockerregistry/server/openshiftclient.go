package server

import (
	"errors"
	"fmt"
	"os"

	osclient "github.com/openshift/origin/pkg/client"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

// NewUserOpenShiftClients returns clients for Kubernetes and OpenShift master api servers with given bearer
// token.
func NewUserOpenShiftClients(bearerToken string) (*kclient.Client, *osclient.Client, error) {
	config, err := openShiftClientConfig()
	if err != nil {
		return nil, nil, err
	}
	config.BearerToken = bearerToken
	return getClientsForConfig(config)
}

// NewRegistryOpenShiftClients returns clients for Kubernetes and OpenShift master api servers.
func NewRegistryOpenShiftClients() (*kclient.Client, *osclient.Client, error) {
	config, err := openShiftClientConfig()
	if err != nil {
		return nil, nil, err
	}
	if !config.Insecure {
		certData := os.Getenv("OPENSHIFT_CERT_DATA")
		if len(certData) == 0 {
			return nil, nil, errors.New("OPENSHIFT_CERT_DATA is required")
		}
		certKeyData := os.Getenv("OPENSHIFT_KEY_DATA")
		if len(certKeyData) == 0 {
			return nil, nil, errors.New("OPENSHIFT_KEY_DATA is required")
		}
		config.TLSClientConfig.CertData = []byte(certData)
		config.TLSClientConfig.KeyData = []byte(certKeyData)
	}
	return getClientsForConfig(config)
}

// openShiftClientConfig creates a config for instantiation of clients out of environment variables.
func openShiftClientConfig() (*kclient.Config, error) {
	openshiftAddr := os.Getenv("OPENSHIFT_MASTER")
	if len(openshiftAddr) == 0 {
		return nil, errors.New("OPENSHIFT_MASTER is required")
	}

	insecure := os.Getenv("OPENSHIFT_INSECURE") == "true"
	var tlsClientConfig kclient.TLSClientConfig
	if !insecure {
		caData := os.Getenv("OPENSHIFT_CA_DATA")
		if len(caData) == 0 {
			return nil, errors.New("OPENSHIFT_CA_DATA is required")
		}
		tlsClientConfig = kclient.TLSClientConfig{
			CAData: []byte(caData),
		}
	}

	return &kclient.Config{
		Host:            openshiftAddr,
		TLSClientConfig: tlsClientConfig,
		Insecure:        insecure,
	}, nil
}

// getClientsForConfig creates Kubernetes and OpenShift master api clients from given config.
func getClientsForConfig(config *kclient.Config) (*kclient.Client, *osclient.Client, error) {
	kClient, err := kclient.New(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Kube client: %s", err)
	}
	osClient, err := osclient.New(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Origin client: %s", err)
	}
	return kClient, osClient, nil
}

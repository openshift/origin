package server

import (
	"errors"
	"fmt"
	"os"

	osclient "github.com/openshift/origin/pkg/client"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

func NewUserOpenShiftClient(bearerToken string) (*osclient.Client, error) {
	config, err := openShiftClientConfig()
	if err != nil {
		return nil, err
	}
	config.BearerToken = bearerToken
	client, err := osclient.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Origin client: %s", err)
	}
	return client, nil
}

// NewRegistryOpenShiftClient returns an client interface for OpenShift
// resources built from environment variables.
func NewRegistryOpenShiftClient() (osclient.Interface, error) {
	config, err := openShiftClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := osclient.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Origin client: %s", err)
	}
	return client, nil
}

// NewRegistryOpenShiftClient returns an client interface for Kubernetes
// resources built from environment variables.
func NewRegistryKubernetesClient() (kclient.Interface, error) {
	config, err := openShiftClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := kclient.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Origin client: %s", err)
	}
	return client, nil
}

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
		tlsClientConfig.CAData = []byte(caData)

		certData := os.Getenv("OPENSHIFT_CERT_DATA")
		if len(certData) == 0 {
			return nil, errors.New("OPENSHIFT_CERT_DATA is required")
		}
		tlsClientConfig.CertData = []byte(certData)

		certKeyData := os.Getenv("OPENSHIFT_KEY_DATA")
		if len(certKeyData) == 0 {
			return nil, errors.New("OPENSHIFT_KEY_DATA is required")
		}
		tlsClientConfig.KeyData = []byte(certKeyData)
	}

	return &kclient.Config{
		Host:            openshiftAddr,
		TLSClientConfig: tlsClientConfig,
		Insecure:        insecure,
	}, nil
}

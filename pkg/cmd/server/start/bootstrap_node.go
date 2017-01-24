package start

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/certificates"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	utilcert "k8s.io/kubernetes/pkg/util/cert"
	"k8s.io/kubernetes/pkg/util/wait"

	"crypto/rsa"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

// hasCSRCondition returns the first matching condition with the given type or nil.
func hasCSRCondition(conditions []certificates.CertificateSigningRequestCondition, t certificates.RequestConditionType) *certificates.CertificateSigningRequestCondition {
	for i := range conditions {
		if conditions[i].Type == t {
			return &conditions[i]
		}
	}
	return nil
}

// readOrCreatePrivateKey attempts to read an rsa private key from the provided path,
// or if that fails, to generate a new private key.
func readOrCreatePrivateKey(path string) (*rsa.PrivateKey, error) {
	if data, err := ioutil.ReadFile(path); err == nil {
		if key, err := utilcert.ParsePrivateKeyPEM(data); err == nil {
			if pkey, ok := key.(*rsa.PrivateKey); ok {
				return pkey, nil
			}
		}
	}
	return utilcert.NewPrivateKey()
}

// loadBootstrapClientCertificate attempts to read a node.kubeconfig file from the config dir,
// and otherwise tries to request a client certificate as a node (system:node:NODE_NAME). It will
// reuse a private key if one exists, and exit with an error if the CSR is not completed within
// timeout or if the current CSR does not validate against the local private key.
func (o NodeOptions) loadBootstrapClientCertificate(nodeConfigDir string, c kclientset.Interface, timeout time.Duration) (kclientset.Interface, error) {
	nodeConfigPath := filepath.Join(nodeConfigDir, "node.kubeconfig")

	// if the node config exists, try to use it or fail
	if _, err := os.Stat(nodeConfigPath); err == nil {
		kubeClientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: nodeConfigPath}, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, err
		}
		return kclientset.NewForConfig(kubeClientConfig)
	}

	clientCertPath := filepath.Join(nodeConfigDir, "master-client.crt")
	clientKeyPath := filepath.Join(nodeConfigDir, "master-client.key")

	// create and sign a client cert
	privateKey, err := readOrCreatePrivateKey(clientKeyPath)
	if err != nil {
		return nil, err
	}
	privateKeyData := utilcert.EncodePrivateKeyPEM(privateKey)
	csrData, err := utilcert.MakeCSR(privateKey, &pkix.Name{
		Organization: []string{"system:nodes"},
		CommonName:   fmt.Sprintf("system:node:%s", o.NodeArgs.NodeName),
		// TODO: indicate usage for client
	}, nil, nil)
	if err != nil {
		return nil, err
	}

	signingRequest := &certificates.CertificateSigningRequest{
		ObjectMeta: kapi.ObjectMeta{
			Name: fmt.Sprintf("node-bootstrapper-client-%s", safeSecretName(o.NodeArgs.NodeName)),
		},
		Spec: certificates.CertificateSigningRequestSpec{
			Request: csrData,
		},
	}

	csr, err := c.Certificates().CertificateSigningRequests().Create(signingRequest)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return nil, err
		}
		glog.V(3).Infof("Bootstrap client certificate already exists at %s", signingRequest.Name)
		csr, err = c.Certificates().CertificateSigningRequests().Get(signingRequest.Name)
		if err != nil {
			return nil, err
		}
	}
	if err := ioutil.WriteFile(clientKeyPath, privateKeyData, 0600); err != nil {
		return nil, err
	}

	err = wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if deny := hasCSRCondition(csr.Status.Conditions, certificates.CertificateDenied); deny != nil {
			glog.V(2).Infof("Bootstrap signing rejected (%s): %s", deny.Reason, deny.Message)
			return false, fmt.Errorf("certificate signing request rejected (%s): %s", deny.Reason, deny.Message)
		}
		if approved := hasCSRCondition(csr.Status.Conditions, certificates.CertificateApproved); approved != nil {
			glog.V(2).Infof("Bootstrap client cert approved")
			return true, nil
		}
		csr, err = c.Certificates().CertificateSigningRequests().Get(csr.Name)
		return false, err
	})
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(clientCertPath, csr.Status.Certificate, 0600); err != nil {
		return nil, err
	}

	if _, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath); err != nil {
		return nil, fmt.Errorf("bootstrap client certificate does not match private key, you may need to delete the client CSR: %v", err)
	}

	// write a kube config file for the node that contains the client cert
	cfg, err := o.NodeArgs.KubeConnectionArgs.ClientConfig.RawConfig()
	if err != nil {
		return nil, err
	}
	if err := clientcmdapi.MinifyConfig(&cfg); err != nil {
		return nil, err
	}
	ctx := cfg.Contexts[cfg.CurrentContext]
	if len(ctx.AuthInfo) == 0 {
		ctx.AuthInfo = "bootstrap"
	}
	cfg.AuthInfos = map[string]*clientcmdapi.AuthInfo{
		ctx.AuthInfo: {
			ClientCertificateData: csr.Status.Certificate,
			ClientKeyData:         privateKeyData,
		},
	}
	if err := clientcmd.WriteToFile(cfg, nodeConfigPath); err != nil {
		return nil, err
	}

	kubeClientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: nodeConfigPath}, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}

	return kclientset.NewForConfig(kubeClientConfig)
}

// loadBootstrapServerCertificate attempts to read a server certificate file from the config dir,
// and otherwise tries to request a server certificate for its registered addresses. It will
// reuse a private key if one exists, and exit with an error if the CSR is not completed within
// timeout or if the current CSR does not validate against the local private key.
func (o NodeOptions) loadBootstrapServerCertificate(nodeConfigDir string, hostnames []string, c kclientset.Interface, timeout time.Duration) error {
	serverCertPath := filepath.Join(nodeConfigDir, "server.crt")
	serverKeyPath := filepath.Join(nodeConfigDir, "server.key")

	if _, err := os.Stat(serverCertPath); err == nil {
		if _, err := os.Stat(serverKeyPath); err == nil {
			if _, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath); err != nil {
				return fmt.Errorf("bootstrap server certificate does not match private key: %v", err)
			}
			// continue
			return nil
		}
	}

	privateKey, err := readOrCreatePrivateKey(serverKeyPath)
	if err != nil {
		return err
	}
	privateKeyData := utilcert.EncodePrivateKeyPEM(privateKey)
	ipAddresses, dnsNames := crypto.IPAddressesDNSNames(hostnames)
	csrData, err := utilcert.MakeCSR(privateKey, &pkix.Name{
		CommonName: dnsNames[0],
		// TODO: indicate usage for server
	}, dnsNames, ipAddresses)
	if err != nil {
		return err
	}

	signingRequest := &certificates.CertificateSigningRequest{
		ObjectMeta: kapi.ObjectMeta{
			Name: fmt.Sprintf("node-bootstrapper-server-%s", safeSecretName(o.NodeArgs.NodeName)),
		},
		Spec: certificates.CertificateSigningRequestSpec{
			Request: csrData,
		},
	}

	csr, err := c.Certificates().CertificateSigningRequests().Create(signingRequest)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return err
		}
		glog.V(3).Infof("Bootstrap server certificate already exists at %s", signingRequest.Name)
		csr, err = c.Certificates().CertificateSigningRequests().Get(signingRequest.Name)
		if err != nil {
			return err
		}
	}
	if err := ioutil.WriteFile(serverKeyPath, privateKeyData, 0600); err != nil {
		return err
	}

	err = wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		if deny := hasCSRCondition(csr.Status.Conditions, certificates.CertificateDenied); deny != nil {
			glog.V(2).Infof("Bootstrap signing rejected (%s): %s", deny.Reason, deny.Message)
			return false, fmt.Errorf("certificate signing request rejected (%s): %s", deny.Reason, deny.Message)
		}
		if approved := hasCSRCondition(csr.Status.Conditions, certificates.CertificateApproved); approved != nil {
			glog.V(2).Infof("Bootstrap serving cert approved")
			return true, nil
		}
		csr, err = c.Certificates().CertificateSigningRequests().Get(csr.Name)
		return false, err
	})
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(serverCertPath, csr.Status.Certificate, 0600); err != nil {
		return err
	}
	if _, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath); err != nil {
		return fmt.Errorf("bootstrap server certificate does not match private key, you may need to delete the server CSR: %v", err)
	}

	return nil
}

// loadBootstrap attempts to ensure a bootstrap configuration exists inside the node config dir
// by contacting the server and requesting a client and server certificate. If successful, it
// will attempt to download a node config file from namespace openshift-infra from the node-config
// ConfigMap. If no configuration is found, a new node config will be generated from the arguments
// and used instead. If no error is returned, nodeConfigDir can be used as a valid node configuration.
func (o NodeOptions) loadBootstrap(hostnames []string, nodeConfigDir string) error {
	if err := os.MkdirAll(nodeConfigDir, 0700); err != nil {
		return err
	}

	kubeClientConfig, err := o.NodeArgs.KubeConnectionArgs.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	var c kclientset.Interface
	c, err = kclientset.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	glog.Infof("Bootstrapping from API server %s (experimental)", kubeClientConfig.Host)

	c, err = o.loadBootstrapClientCertificate(nodeConfigDir, c, 1*time.Minute)
	if err != nil {
		return err
	}
	if err := o.loadBootstrapServerCertificate(nodeConfigDir, hostnames, c, 1*time.Minute); err != nil {
		return err
	}

	nodeClientCAPath := filepath.Join(nodeConfigDir, "node-client-ca.crt")
	if len(kubeClientConfig.CAData) > 0 {
		if err := ioutil.WriteFile(nodeClientCAPath, []byte(kubeClientConfig.CAData), 0600); err != nil {
			return err
		}
	}

	// try to refresh the latest node-config.yaml
	o.ConfigFile = filepath.Join(nodeConfigDir, "node-config.yaml")
	config, err := c.Core().ConfigMaps("openshift-infra").Get("node-config")
	if err == nil {
		// skip all the config we generated ourselves
		skipConfig := map[string]struct{}{"server.crt": {}, "server.key": {}, "master-client.crt": {}, "master-client.key": {}, "node.kubeconfig": {}}
		for k, v := range config.Data {
			if _, ok := skipConfig[k]; ok {
				continue
			}
			b := []byte(v)
			// if a node config is provided, override the setup.
			if k == "node-config.yaml" {
				if err := ioutil.WriteFile(o.ConfigFile, []byte(v), 0600); err != nil {
					return err
				}
				nodeConfig, err := configapilatest.ReadNodeConfig(o.ConfigFile)
				if err != nil {
					return err
				}
				if err := o.NodeArgs.MergeSerializeableNodeConfig(nodeConfig); err != nil {
					return err
				}
				nodeConfig.ServingInfo.ServerCert.CertFile = filepath.Join(nodeConfigDir, "server.crt")
				nodeConfig.ServingInfo.ServerCert.KeyFile = filepath.Join(nodeConfigDir, "server.key")
				nodeConfig.ServingInfo.ClientCA = nodeClientCAPath
				nodeConfig.MasterKubeConfig = filepath.Join(nodeConfigDir, "node.kubeconfig")
				b, err = configapilatest.WriteYAML(nodeConfig)
				if err != nil {
					return err
				}
			}

			if err := ioutil.WriteFile(filepath.Join(nodeConfigDir, k), b, 0600); err != nil {
				return err
			}
		}
		glog.V(3).Infof("Received %d bootstrap files into %s", len(config.Data), nodeConfigDir)
	}

	// if we had a previous node-config.yaml, continue using it
	if _, err2 := os.Stat(o.ConfigFile); err2 == nil {
		if err == nil {
			glog.V(2).Infof("Unable to load node configuration from the server: %v", err)
		}
		return nil
	}

	// if there is no node-config.yaml and no server config map, generate one
	if kerrors.IsNotFound(err) {
		glog.V(2).Infof("Generating a local configuration since no server config available")
		nodeConfig, err := o.NodeArgs.BuildSerializeableNodeConfig()
		if err != nil {
			return err
		}
		if err := o.NodeArgs.MergeSerializeableNodeConfig(nodeConfig); err != nil {
			return err
		}
		nodeConfig.ServingInfo.ServerCert.CertFile = "server.crt"
		nodeConfig.ServingInfo.ServerCert.KeyFile = "server.key"
		nodeConfig.ServingInfo.ClientCA = "node-client-ca.crt"
		nodeConfig.MasterKubeConfig = "node.kubeconfig"
		b, err := configapilatest.WriteYAML(nodeConfig)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(o.ConfigFile, b, 0600); err != nil {
			return err
		}
		return nil
	}

	return err
}

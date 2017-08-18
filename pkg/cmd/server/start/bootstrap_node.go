package start

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
	utilcert "k8s.io/client-go/util/cert"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/apis/certificates"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	clientcertificates "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/certificates/internalversion"

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

// requestCertificate will create a certificate signing request using the PEM
// encoded CSR and send it to API server, then it will watch the object's
// status, once approved by API server, it will return the API server's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func requestCertificate(client clientcertificates.CertificateSigningRequestInterface, csrData []byte, usages []certificates.KeyUsage) (certData []byte, err error) {
	req, err := client.Create(&certificates.CertificateSigningRequest{
		// Username, UID, Groups will be injected by API server.
		ObjectMeta: metav1.ObjectMeta{GenerateName: "csr-"},

		Spec: certificates.CertificateSigningRequestSpec{
			Request: csrData,
			Usages:  usages,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create certificate signing request: %v", err)
	}

	// Make a default timeout = 3600s.
	var defaultTimeoutSeconds int64 = 3600
	certWatch, err := client.Watch(metav1.ListOptions{
		Watch:          true,
		TimeoutSeconds: &defaultTimeoutSeconds,
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", req.Name).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot watch on the certificate signing request: %v", err)
	}

	var certificateData []byte
	_, err = watch.Until(0, certWatch, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && event.Type != watch.Added {
			return false, nil
		}
		if event.Object.(*certificates.CertificateSigningRequest).UID != req.UID {
			return false, nil
		}

		status := event.Object.(*certificates.CertificateSigningRequest).Status
		for _, c := range status.Conditions {
			if c.Type == certificates.CertificateDenied {
				return false, fmt.Errorf("certificate signing request is not approved, reason: %v, message: %v", c.Reason, c.Message)
			}
			if c.Type == certificates.CertificateApproved && status.Certificate != nil {
				certificateData = status.Certificate
				return true, nil
			}
		}
		return false, nil
	})
	return certificateData, err
}

// loadBootstrapServerCertificate attempts to read a server certificate file from the config dir,
// and otherwise tries to request a server certificate for its registered addresses. It will
// reuse a private key if one exists, and exit with an error if the CSR is not completed within
// timeout or if the current CSR does not validate against the local private key.
func loadBootstrapServerCertificate(nodeConfigDir string, hostnames []string, c kclientset.Interface) error {
	glog.V(2).Info("Using node kubeconfig to generate TLS server cert and key file")

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

	serverCertData, err := requestCertificate(
		c.Certificates().CertificateSigningRequests(),
		csrData,
		[]certificates.KeyUsage{
			// https://tools.ietf.org/html/rfc5280#section-4.2.1.3
			//
			// Digital signature allows the certificate to be used to verify
			// digital signatures used during TLS negotiation.
			certificates.UsageDigitalSignature,
			// KeyEncipherment allows the cert/key pair to be used to encrypt
			// keys, including the symetric keys negotiated during TLS setup
			// and used for data transfer.
			certificates.UsageKeyEncipherment,
			// ServerAuth allows the cert to be used by a TLS server to
			// authenticate itself to a TLS client.
			certificates.UsageServerAuth,
		},
	)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(serverKeyPath, privateKeyData, 0600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(serverCertPath, serverCertData, 0600); err != nil {
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

	nodeKubeconfig := filepath.Join(nodeConfigDir, "node.kubeconfig")

	if err := kubeletapp.BootstrapClientCert(
		nodeKubeconfig,
		o.NodeArgs.KubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath,
		nodeConfigDir,
		types.NodeName(o.NodeArgs.NodeName),
	); err != nil {
		return err
	}

	kubeClientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: nodeKubeconfig}, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}
	c, err := kclientset.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	if err := loadBootstrapServerCertificate(nodeConfigDir, hostnames, c); err != nil {
		return err
	}

	nodeClientCAPath := filepath.Join(nodeConfigDir, "node-client-ca.crt")
	if err := utilcert.WriteCert(nodeClientCAPath, kubeClientConfig.CAData); err != nil {
		return err
	}

	// try to refresh the latest node-config.yaml
	o.ConfigFile = filepath.Join(nodeConfigDir, "node-config.yaml")
	config, err := c.Core().ConfigMaps("kube-system").Get("node-config", metav1.GetOptions{})
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

package start

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	utilcert "k8s.io/client-go/util/cert"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubelet/certificate/bootstrap"
	nodeutil "k8s.io/kubernetes/pkg/util/node"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// loadBootstrap attempts to ensure a bootstrap configuration exists inside the node config dir
// by contacting the server and requesting a client certificate. If successful, it
// will attempt to download a node config file from namespace openshift-node from the node-config
// ConfigMap. If no error is returned, nodeConfigDir can be used as a valid node configuration.
// The actions of this method are intended to emulate the behavior of the kubelet during startup
// and will eventually be replaced by a pure Kubelet bootstrap. Bootstrap mode *requires* server
// certificate rotation to be enabled because it generates no server certificate.
func (o NodeOptions) loadBootstrap(nodeConfigDir string) error {
	if err := os.MkdirAll(nodeConfigDir, 0700); err != nil {
		return err
	}

	// Emulate Kubelet bootstrapping - this codepath will be removed in a future release
	// when we adopt dynamic config in the Kubelet.
	bootstrapKubeconfig := o.NodeArgs.KubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath
	nodeKubeconfig := filepath.Join(nodeConfigDir, "node.kubeconfig")
	certDir := filepath.Join(nodeConfigDir, "certificates")
	if err := bootstrap.LoadClientCert(
		nodeKubeconfig,
		bootstrapKubeconfig,
		certDir,
		types.NodeName(o.NodeArgs.NodeName),
	); err != nil {
		return err
	}
	if err := os.MkdirAll(certDir, 0600); err != nil {
		return fmt.Errorf("unable to create kubelet certificate directory: %v", err)
	}
	kubeClientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: nodeKubeconfig}, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}
	// clear the current client from the cert dir to ensure that the next rotation captures a new state
	if err := os.Remove(filepath.Join(certDir, "kubelet-client-current.pem")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to remove current client pem for bootstrapping: %v", err)
	}

	c, err := kclientset.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	nodeClientCAPath := filepath.Join(nodeConfigDir, "node-client-ca.crt")
	if err := utilcert.WriteCert(nodeClientCAPath, kubeClientConfig.CAData); err != nil {
		return err
	}

	// try to refresh the latest node-config.yaml
	o.ConfigFile = filepath.Join(nodeConfigDir, "node-config.yaml")
	config, err := c.Core().ConfigMaps(o.NodeArgs.BootstrapConfigNamespace).Get(o.NodeArgs.BootstrapConfigName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsForbidden(err) {
			glog.Warningf("Node is not authorized to access master, treating bootstrap configuration as invalid and exiting: %v", err)
			if err := os.Remove(nodeKubeconfig); err != nil && !os.IsNotExist(err) {
				glog.Warningf("Unable to remove bootstrap client configuration: %v", err)
			}
			return err
		}
		glog.Warningf("Node is unable to access config, exiting: %v", err)
		return err
	}

	glog.V(2).Infof("Loading node configuration from %s/%s (rv=%s, uid=%s)", config.Namespace, config.Name, config.ResourceVersion, config.UID)
	// skip all the config we generated ourselves
	var loaded []string
	skipConfig := map[string]struct{}{
		"server.crt":        {},
		"server.key":        {},
		"master-client.crt": {},
		"master-client.key": {},
		"node.kubeconfig":   {},
	}
	for k, v := range config.Data {
		if _, ok := skipConfig[k]; ok {
			glog.V(2).Infof("Skipping key %q from config map", k)
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

			overrideNodeConfigForBootstrap(nodeConfig, bootstrapKubeconfig)

			b, err = configapilatest.WriteYAML(nodeConfig)
			if err != nil {
				return err
			}
		}
		loaded = append(loaded, k)
		if err := ioutil.WriteFile(filepath.Join(nodeConfigDir, k), b, 0600); err != nil {
			return err
		}
	}
	glog.V(3).Infof("Received bootstrap files into %s: %s", nodeConfigDir, strings.Join(loaded, ", "))
	return nil
}

// overrideNodeConfigForBootstrap sets certain bootstrap overrides.
func overrideNodeConfigForBootstrap(nodeConfig *configapi.NodeConfig, bootstrapKubeconfig string) {
	if nodeConfig.KubeletArguments == nil {
		nodeConfig.KubeletArguments = configapi.ExtendedArguments{}
	}

	// Set impliict defaults the same as the kubelet (until this entire code path is removed)
	nodeConfig.NodeName = nodeutil.GetHostname(nodeConfig.NodeName)
	if nodeConfig.DNSIP == "0.0.0.0" {
		nodeConfig.DNSIP = nodeConfig.NodeIP
		// TODO: the Kubelet should do this defaulting (to the IP it recognizes)
		if len(nodeConfig.DNSIP) == 0 {
			if ip, err := cmdutil.DefaultLocalIP4(); err == nil {
				nodeConfig.DNSIP = ip.String()
			}
		}
	}

	// Created during bootstrapping
	nodeConfig.ServingInfo.ClientCA = "node-client-ca.crt"

	// We will use cert-dir instead and bootstrapping
	nodeConfig.ServingInfo.ServerCert.CertFile = ""
	nodeConfig.ServingInfo.ServerCert.KeyFile = ""
	nodeConfig.KubeletArguments["bootstrap-kubeconfig"] = []string{bootstrapKubeconfig}
	nodeConfig.KubeletArguments["rotate-certificates"] = []string{"true"}

	// Default a valid certificate directory to store bootstrap certs
	if _, ok := nodeConfig.KubeletArguments["cert-dir"]; !ok {
		nodeConfig.KubeletArguments["cert-dir"] = []string{"./certificates"}
	}
	// Enable both client and server rotation when bootstrapping
	if _, ok := nodeConfig.KubeletArguments["feature-gates"]; !ok {
		nodeConfig.KubeletArguments["feature-gates"] = []string{
			"RotateKubeletClientCertificate=true,RotateKubeletServerCertificate=true",
		}
	}
}

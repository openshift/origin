package kubelet

import (
	"io"
	"io/ioutil"
	"path"
	"strings"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"k8s.io/apimachinery/pkg/runtime"
)

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func MutateKubeDNSConfig(nodeConfigDir string, out io.Writer) (string, error) {
	// update DNS resolution to point at the master (for now).  Do this by grabbing the local and prepending to it.
	// this is probably broken somewhere for some reason and a bad idea of other reasons, but it gets us moving
	if existingResolveConf, err := ioutil.ReadFile("/etc/resolv.conf"); err == nil {
		if err := ioutil.WriteFile(path.Join(nodeConfigDir, "resolv.conf"), existingResolveConf, 0644); err != nil {
			return "", err
		}

	} else {
		// TODO this may not be fatal after it sort of works.
		return "", err
	}

	// update some listen information to include starting the DNS server
	nodeConfigFilename := path.Join(nodeConfigDir, "node-config.yaml")
	originalBytes, err := ioutil.ReadFile(nodeConfigFilename)
	if err != nil {
		return "", err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, originalBytes)
	if err != nil {
		return "", err
	}
	nodeConfig := configObj.(*configapi.NodeConfig)
	nodeConfig.DNSBindAddress = "0.0.0.0:53"
	nodeConfig.DNSRecursiveResolvConf = "resolv.conf"
	configBytes, err := runtime.Encode(configapilatest.Codec, nodeConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(nodeConfigFilename, configBytes, 0644); err != nil {
		return "", err
	}

	// update the node kubeconfig file to point to the IP of the master.
	// TODO figure out where this comes from
	kubeconfigFilename := path.Join(nodeConfigDir, "node.kubeconfig")
	originalKubeconfigBytes, err := ioutil.ReadFile(kubeconfigFilename)
	newKubeconfigBytes := substitute(string(originalKubeconfigBytes), map[string]string{"https://localhost:8443": "https://172.30.0.1"})
	if err := ioutil.WriteFile(kubeconfigFilename, []byte(newKubeconfigBytes), 0600); err != nil {
		return "", err
	}

	return nodeConfigDir, nil
}

func substitute(in string, replacements map[string]string) string {
	curr := in
	for oldVal, newVal := range replacements {
		curr = strings.Replace(curr, oldVal, newVal, -1)
	}

	return curr
}

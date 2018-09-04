package kubelet

import (
	"io/ioutil"
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
)

func MakeKubeDNSConfig(existingNodeConfig string, basedir string, hostIP string) (string, error) {
	configDir := path.Join(basedir, KubeDNSDirName)
	glog.V(1).Infof("Copying kubelet config to local directory %s", configDir)
	if err := tmpformac.CopyDirectory(existingNodeConfig, configDir); err != nil {
		return "", err
	}

	// update DNS resolution to point at the master (for now).  Do this by grabbing the local and prepending to it.
	// this is probably broken somewhere for some reason and a bad idea of other reasons, but it gets us moving
	if existingResolveConf, err := ioutil.ReadFile("/etc/resolv.conf"); err == nil {
		// Transform references to localhost for users running dnsmasq locally or actually running DNS there
		existingResolveConf = []byte(substitute(string(existingResolveConf), map[string]string{"127.0.0.1": hostIP, "localhost": hostIP}))
		if err := ioutil.WriteFile(path.Join(configDir, "resolv.conf"), existingResolveConf, 0644); err != nil {
			return "", err
		}

	} else {
		// TODO this may not be fatal after it sort of works.
		return "", err
	}

	// update some listen information to include starting the DNS server
	nodeConfigFilename := path.Join(configDir, "node-config.yaml")
	nodeConfig, err := componentinstall.ReadNodeConfig(nodeConfigFilename)
	if err != nil {
		return "", err
	}
	nodeConfig.DNSBindAddress = "0.0.0.0:53"
	nodeConfig.DNSRecursiveResolvConf = "resolv.conf"

	if err := componentinstall.WriteNodeConfig(nodeConfigFilename, nodeConfig); err != nil {
		return "", err
	}

	// update the node kubeconfig file to point to the IP of the master.
	// TODO figure out where this comes from
	kubeconfigFilename := path.Join(configDir, "node.kubeconfig")
	originalKubeconfigBytes, err := ioutil.ReadFile(kubeconfigFilename)
	newKubeconfigBytes := substitute(string(originalKubeconfigBytes), map[string]string{"https://localhost:8443": "https://172.30.0.1"})
	if err := ioutil.WriteFile(kubeconfigFilename, []byte(newKubeconfigBytes), 0600); err != nil {
		return "", err
	}

	return configDir, nil
}

func substitute(in string, replacements map[string]string) string {
	curr := in
	for oldVal, newVal := range replacements {
		curr = strings.Replace(curr, oldVal, newVal, -1)
	}

	return curr
}

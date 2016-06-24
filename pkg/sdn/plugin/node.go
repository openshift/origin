package plugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/plugin/api"
	"github.com/openshift/origin/pkg/util/netutils"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubeletcni "k8s.io/kubernetes/pkg/kubelet/network/cni"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"

	cniinvoke "github.com/containernetworking/cni/pkg/invoke"
	cnitypes "github.com/containernetworking/cni/pkg/types"
)

type OsdnNode struct {
	multitenant        bool
	registry           *Registry
	localIP            string
	hostName           string
	podNetworkReady    chan struct{}
	vnids              *nodeVNIDMap
	iptablesSyncPeriod time.Duration
	mtu                uint32
	egressPolicies     map[uint32][]*osapi.EgressNetworkPolicy
	masterKubeConfig   string
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration, mtu uint32, masterKubeConfig string) (*OsdnNode, error) {
	if !IsOpenShiftNetworkPlugin(pluginName) {
		return nil, nil
	}

	log.Infof("Initializing SDN node of type %q with configured hostname %q (IP %q), iptables sync period %q", pluginName, hostname, selfIP, iptablesSyncPeriod.String())
	if hostname == "" {
		output, err := kexec.New().Command("uname", "-n").CombinedOutput()
		if err != nil {
			return nil, err
		}
		hostname = strings.TrimSpace(string(output))
		log.Infof("Resolved hostname to %q", hostname)
	}
	if selfIP == "" {
		var err error
		selfIP, err = netutils.GetNodeIP(hostname)
		if err != nil {
			log.V(5).Infof("Failed to determine node address from hostname %s; using default interface (%v)", hostname, err)
			var defaultIP net.IP
			defaultIP, err = kubeutilnet.ChooseHostInterface()
			if err != nil {
				return nil, err
			}
			selfIP = defaultIP.String()
			log.Infof("Resolved IP address to %q", selfIP)
		}
	}

	if err := writeCNIConfig(masterKubeConfig, hostname, mtu); err != nil {
		return nil, err
	}

	plugin := &OsdnNode{
		multitenant:        IsOpenShiftMultitenantNetworkPlugin(pluginName),
		registry:           newRegistry(osClient, kClient),
		localIP:            selfIP,
		hostName:           hostname,
		vnids:              newNodeVNIDMap(),
		podNetworkReady:    make(chan struct{}),
		iptablesSyncPeriod: iptablesSyncPeriod,
		masterKubeConfig:   masterKubeConfig,
		mtu:                mtu,
		egressPolicies:     make(map[uint32][]*osapi.EgressNetworkPolicy),
	}
	return plugin, nil
}

func getCNIConfig(masterKubeConfig string, hostname string, mtu uint32) ([]byte, error) {
	return json.Marshal(&api.CNINetConfig{
		NetConf: cnitypes.NetConf{
			Name: "openshift-sdn",
			Type: "openshift-sdn",
		},
		MasterKubeConfig: masterKubeConfig,
		NodeName:         hostname,
		MTU:              mtu,
	})
}

const cniConfigFile string = "80-openshift-sdn.conf"

func writeCNIConfig(masterKubeConfig string, hostname string, mtu uint32) error {
	cniConfig, err := getCNIConfig(masterKubeConfig, hostname, mtu)
	if err != nil {
		return err
	}

	cniConfigPath := filepath.Join(kubeletcni.DefaultNetDir, cniConfigFile)
	if err := os.MkdirAll(path.Dir(cniConfigPath), 0700); err != nil {
		return fmt.Errorf("failed to create CNI config directory: %v", err)
	}

	if err := ioutil.WriteFile(cniConfigPath, cniConfig, 0700); err != nil {
		return fmt.Errorf("failed to create CNI config file: %v", err)
	}

	return nil
}

func (node *OsdnNode) Start() error {
	ni, err := node.registry.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("Failed to get network information: %v", err)
	}

	nodeIPTables := newNodeIPTables(ni.ClusterNetwork.String(), node.iptablesSyncPeriod)
	if err = nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("Failed to set up iptables: %v", err)
	}

	var networkChanged bool
	networkChanged, err = node.SubnetStartNode(node.mtu)
	if err != nil {
		return err
	}

	if node.multitenant {
		if err = node.VnidStartNode(); err != nil {
			return err
		}
		if err = node.SetupEgressNetworkPolicy(); err != nil {
			return err
		}
	}

	if networkChanged {
		var pods []kapi.Pod
		pods, err = node.GetLocalPods(kapi.NamespaceAll)
		if err != nil {
			return err
		}
		for _, p := range pods {
			containerID := getPodContainerID(&p)
			err = node.UpdatePod(p.Namespace, p.Name, kubeletTypes.ContainerID{ID: containerID})
			if err != nil {
				log.Warningf("Could not update pod %q (%s): %s", p.Name, containerID, err)
			}
		}
	}

	node.markPodNetworkReady()

	return nil
}

// FIXME: this should eventually go into kubelet via a CNI UPDATE/CHANGE action
// See https://github.com/containernetworking/cni/issues/89
func (node *OsdnNode) UpdatePod(namespace string, name string, id kubeletTypes.ContainerID) error {
	pluginPath, err := cniinvoke.FindInPath("openshift-sdn", []string{kubeletcni.DefaultCNIDir})
	if err != nil {
		return err
	}

	cniConfig, err := getCNIConfig(node.masterKubeConfig, node.hostName, node.mtu)
	if err != nil {
		return err
	}

	args := &cniinvoke.Args{
		Command:     "UPDATE",
		ContainerID: id.String(),
		NetNS:       "/blahblah/foobar", // plugin finds out namespace itself
		PluginArgs: [][2]string{
			{"K8S_POD_NAMESPACE", namespace},
			{"K8S_POD_NAME", name},
			{"K8S_POD_INFRA_CONTAINER_ID", id.String()},
			{"OPENSHIFT_ACTION", "UPDATE"},
		},
		IfName: knetwork.DefaultInterfaceName,
		Path:   filepath.Dir(pluginPath),
	}

	if _, err := cniinvoke.ExecPluginWithResult(pluginPath, cniConfig, args); err != nil {
		return fmt.Errorf("failed to update pod network: %v", err)
	}

	return nil
}

func (node *OsdnNode) GetLocalPods(namespace string) ([]kapi.Pod, error) {
	return node.registry.GetRunningPods(node.hostName, namespace)
}

func (node *OsdnNode) markPodNetworkReady() {
	close(node.podNetworkReady)
}

func (node *OsdnNode) WaitForPodNetworkReady() error {
	logInterval := 10 * time.Second
	numIntervals := 12 // timeout: 2 mins

	for i := 0; i < numIntervals; i++ {
		select {
		// Wait for StartNode() to finish SDN setup
		case <-node.podNetworkReady:
			return nil
		case <-time.After(logInterval):
			log.Infof("Waiting for SDN pod network to be ready...")
		}
	}
	return fmt.Errorf("SDN pod network is not ready(timeout: 2 mins)")
}

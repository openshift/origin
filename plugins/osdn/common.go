package osdn

import (
	"fmt"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"
)

type OsdnNode struct {
	multitenant        bool
	Registry           *Registry
	localIP            string
	localSubnet        *osapi.HostSubnet
	HostName           string
	podNetworkReady    chan struct{}
	vnids              vnidMap
	iptablesSyncPeriod time.Duration
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration) (api.OsdnNodePlugin, error) {
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
			defaultIP, err := kubeutilnet.ChooseHostInterface()
			if err != nil {
				return nil, err
			}
			selfIP = defaultIP.String()
			log.Infof("Resolved IP address to %q", selfIP)
		}
	}

	plugin := &OsdnNode{
		multitenant:        IsOpenShiftMultitenantNetworkPlugin(pluginName),
		Registry:           NewRegistry(osClient, kClient),
		localIP:            selfIP,
		HostName:           hostname,
		vnids:              newVnidMap(),
		podNetworkReady:    make(chan struct{}),
		iptablesSyncPeriod: iptablesSyncPeriod,
	}
	return plugin, nil
}

func (oc *OsdnNode) Start(mtu uint) error {
	// Assume we are working with IPv4
	ni, err := oc.Registry.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("Failed to get network information: %v", err)
	}

	nodeIPTables := NewNodeIPTables(ni.ClusterNetwork.String(), oc.iptablesSyncPeriod)
	if err := nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("Failed to set up iptables: %v", err)
	}

	networkChanged, err := oc.SubnetStartNode(mtu)
	if err != nil {
		return err
	}

	if oc.multitenant {
		if err := oc.VnidStartNode(); err != nil {
			return err
		}
	}

	if networkChanged {
		pods, err := oc.GetLocalPods(kapi.NamespaceAll)
		if err != nil {
			return err
		}
		for _, p := range pods {
			containerID := GetPodContainerID(&p)
			err = oc.UpdatePod(p.Namespace, p.Name, kubeletTypes.DockerID(containerID))
			if err != nil {
				log.Warningf("Could not update pod %q (%s): %s", p.Name, containerID, err)
			}
		}
	}

	oc.markPodNetworkReady()

	return nil
}

func (oc *OsdnNode) GetLocalPods(namespace string) ([]kapi.Pod, error) {
	return oc.Registry.GetRunningPods(oc.HostName, namespace)
}

func (oc *OsdnNode) markPodNetworkReady() {
	close(oc.podNetworkReady)
}

func (oc *OsdnNode) WaitForPodNetworkReady() error {
	logInterval := 10 * time.Second
	numIntervals := 12 // timeout: 2 mins

	for i := 0; i < numIntervals; i++ {
		select {
		// Wait for StartNode() to finish SDN setup
		case <-oc.podNetworkReady:
			return nil
		case <-time.After(logInterval):
			log.Infof("Waiting for SDN pod network to be ready...")
		}
	}
	return fmt.Errorf("SDN pod network is not ready(timeout: 2 mins)")
}

func GetNodeIP(node *kapi.Node) (string, error) {
	if len(node.Status.Addresses) > 0 && node.Status.Addresses[0].Address != "" {
		return node.Status.Addresses[0].Address, nil
	} else {
		return netutils.GetNodeIP(node.Name)
	}
}

func GetPodContainerID(pod *kapi.Pod) string {
	if len(pod.Status.ContainerStatuses) > 0 {
		// Extract only container ID, pod.Status.ContainerStatuses[0].ContainerID is of the format: docker://<containerID>
		if parts := strings.Split(pod.Status.ContainerStatuses[0].ContainerID, "://"); len(parts) > 1 {
			return parts[1]
		}
	}
	return ""
}

func HostSubnetToString(subnet *osapi.HostSubnet) string {
	return fmt.Sprintf("%s (host: %q, ip: %q, subnet: %q)", subnet.Name, subnet.Host, subnet.HostIP, subnet.Subnet)
}

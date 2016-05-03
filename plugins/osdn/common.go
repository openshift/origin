package osdn

import (
	"fmt"
	"net"
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
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"
)

type OsdnController struct {
	pluginName         string
	multitenant        bool
	Registry           *Registry
	localIP            string
	localSubnet        *osapi.HostSubnet
	HostName           string
	subnetAllocator    *netutils.SubnetAllocator
	podNetworkReady    chan struct{}
	vnids              vnidMap
	netIDManager       *netutils.NetIDAllocator
	adminNamespaces    []string
	iptablesSyncPeriod time.Duration
}

// Called by higher layers to create the plugin SDN master instance
func NewMasterPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client) (api.OsdnPlugin, error) {
	return createPlugin(osClient, kClient, pluginName, "", "", 0)
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration) (api.OsdnPlugin, error) {
	return createPlugin(osClient, kClient, pluginName, hostname, selfIP, iptablesSyncPeriod)
}

func createPlugin(osClient *osclient.Client, kClient *kclient.Client, pluginName string, hostname string, selfIP string, iptablesSyncPeriod time.Duration) (api.OsdnPlugin, error) {
	if !IsOpenShiftNetworkPlugin(pluginName) {
		return nil, nil
	}

	log.Infof("Starting with configured hostname %q (IP %q), iptables sync period %q", hostname, selfIP, iptablesSyncPeriod.String())

	if hostname == "" {
		output, err := kexec.New().Command("uname", "-n").CombinedOutput()
		if err != nil {
			return nil, err
		}
		hostname = strings.TrimSpace(string(output))
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
		}
	}

	log.Infof("Initializing %s plugin for %s (%s)", pluginName, hostname, selfIP)
	plugin := &OsdnController{
		pluginName:         pluginName,
		multitenant:        IsOpenShiftMultitenantNetworkPlugin(pluginName),
		Registry:           NewRegistry(osClient, kClient),
		localIP:            selfIP,
		HostName:           hostname,
		vnids:              newVnidMap(),
		podNetworkReady:    make(chan struct{}),
		adminNamespaces:    make([]string, 0),
		iptablesSyncPeriod: iptablesSyncPeriod,
	}
	return plugin, nil
}

func (oc *OsdnController) validateNetworkConfig(ni *NetworkInfo) error {
	hostIPNets, err := netutils.GetHostIPNetworks([]string{TUN, LBR})
	if err != nil {
		return err
	}

	errList := []error{}

	// Ensure cluster and service network don't overlap with host networks
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(ni.ClusterNetwork.IP) {
			errList = append(errList, fmt.Errorf("Error: Cluster IP: %s conflicts with host network: %s", ni.ClusterNetwork.IP.String(), ipNet.String()))
		}
		if ni.ClusterNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with cluster network: %s", ipNet.IP.String(), ni.ClusterNetwork.String()))
		}
		if ipNet.Contains(ni.ServiceNetwork.IP) {
			errList = append(errList, fmt.Errorf("Error: Service IP: %s conflicts with host network: %s", ni.ServiceNetwork.String(), ipNet.String()))
		}
		if ni.ServiceNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with service network: %s", ipNet.IP.String(), ni.ServiceNetwork.String()))
		}
	}

	// Ensure each host subnet is within the cluster network
	subnets, err := oc.Registry.GetSubnets()
	if err != nil {
		return fmt.Errorf("Error in initializing/fetching subnets: %v", err)
	}
	for _, sub := range subnets {
		subnetIP, _, err := net.ParseCIDR(sub.Subnet)
		if err != nil {
			errList = append(errList, fmt.Errorf("Failed to parse network address: %s", sub.Subnet))
			continue
		}
		if !ni.ClusterNetwork.Contains(subnetIP) {
			errList = append(errList, fmt.Errorf("Error: Existing node subnet: %s is not part of cluster network: %s", sub.Subnet, ni.ClusterNetwork.String()))
		}
	}

	// Ensure each service is within the services network
	services, err := oc.Registry.GetServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		if !ni.ServiceNetwork.Contains(net.ParseIP(svc.Spec.ClusterIP)) {
			errList = append(errList, fmt.Errorf("Error: Existing service with IP: %s is not part of service network: %s", svc.Spec.ClusterIP, ni.ServiceNetwork.String()))
		}
	}

	return kerrors.NewAggregate(errList)
}

func (oc *OsdnController) isClusterNetworkChanged(curNetwork *NetworkInfo) (bool, error) {
	oldNetwork, err := oc.Registry.GetNetworkInfo()
	if err != nil {
		return false, err
	}

	if curNetwork.ClusterNetwork.String() != oldNetwork.ClusterNetwork.String() ||
		curNetwork.HostSubnetLength != oldNetwork.HostSubnetLength ||
		curNetwork.ServiceNetwork.String() != oldNetwork.ServiceNetwork.String() ||
		curNetwork.PluginName != oldNetwork.PluginName {
		return true, nil
	}
	return false, nil
}

func (oc *OsdnController) StartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	// Validate command-line/config parameters
	ni, err := ValidateClusterNetwork(clusterNetworkCIDR, int(clusterBitsPerSubnet), serviceNetworkCIDR, oc.pluginName)
	if err != nil {
		return err
	}

	changed, net_err := oc.isClusterNetworkChanged(ni)
	if changed {
		if err := oc.validateNetworkConfig(ni); err != nil {
			return err
		}
		if err := oc.Registry.UpdateClusterNetwork(ni); err != nil {
			return err
		}
	} else if net_err != nil {
		if err := oc.Registry.CreateClusterNetwork(ni); err != nil {
			return err
		}
	}

	if err := oc.SubnetStartMaster(ni.ClusterNetwork, clusterBitsPerSubnet); err != nil {
		return err
	}

	if oc.multitenant {
		if err := oc.VnidStartMaster(); err != nil {
			return err
		}
	}

	return nil
}

func (oc *OsdnController) StartNode(mtu uint) error {
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

func (oc *OsdnController) GetLocalPods(namespace string) ([]kapi.Pod, error) {
	return oc.Registry.GetRunningPods(oc.HostName, namespace)
}

func (oc *OsdnController) markPodNetworkReady() {
	close(oc.podNetworkReady)
}

func (oc *OsdnController) WaitForPodNetworkReady() error {
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

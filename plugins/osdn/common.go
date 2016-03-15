package osdn

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	kubetypes "k8s.io/kubernetes/pkg/kubelet/container"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"
)

type PluginHooks interface {
	PluginStartMaster(clusterNetwork *net.IPNet, hostSubnetLength uint) error
	PluginStartNode(mtu uint) error
	UpdatePod(namespace string, name string, id kubetypes.DockerID) error
}

type OvsController struct {
	pluginHooks     PluginHooks
	Registry        *Registry
	localIP         string
	localSubnet     *api.Subnet
	HostName        string
	subnetAllocator *netutils.SubnetAllocator
	sig             chan struct{}
	podNetworkReady chan struct{}
	flowController  FlowController
	VNIDMap         map[string]uint
	netIDManager    *netutils.NetIDAllocator
	adminNamespaces []string
	services        map[string]api.Service
}

type FlowController interface {
	Setup(localSubnetCIDR, clusterNetworkCIDR, serviceNetworkCIDR string, mtu uint) (bool, error)

	AddOFRules(nodeIP, nodeSubnetCIDR, localIP string) error
	DelOFRules(nodeIP, localIP string) error

	AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error
	DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error
}

// Called by plug factory functions to initialize the generic plugin instance
func (oc *OvsController) BaseInit(registry *Registry, flowController FlowController, pluginHooks PluginHooks, hostname string, selfIP string) error {

	if hostname == "" {
		output, err := kexec.New().Command("uname", "-n").CombinedOutput()
		if err != nil {
			return err
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
				return err
			}
			selfIP = defaultIP.String()
		}
	}
	log.Infof("Self IP: %s.", selfIP)

	oc.pluginHooks = pluginHooks
	oc.Registry = registry
	oc.flowController = flowController
	oc.localIP = selfIP
	oc.HostName = hostname
	oc.VNIDMap = make(map[string]uint)
	oc.sig = make(chan struct{})
	oc.podNetworkReady = make(chan struct{})
	oc.adminNamespaces = make([]string, 0)
	oc.services = make(map[string]api.Service)

	return nil
}

func (oc *OvsController) validateNetworkConfig(clusterNetwork, serviceNetwork *net.IPNet) error {
	// TODO: Instead of hardcoding 'tun0' and 'lbr0', get it from common place.
	// This will ensure both the kube/multitenant scripts and master validations use the same name.
	hostIPNets, err := netutils.GetHostIPNetworks([]string{"tun0", "lbr0"})
	if err != nil {
		return err
	}

	errList := []error{}

	// Ensure cluster and service network don't overlap with host networks
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(clusterNetwork.IP) {
			errList = append(errList, fmt.Errorf("Error: Cluster IP: %s conflicts with host network: %s", clusterNetwork.IP.String(), ipNet.String()))
		}
		if clusterNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with cluster network: %s", ipNet.IP.String(), clusterNetwork.String()))
		}
		if ipNet.Contains(serviceNetwork.IP) {
			errList = append(errList, fmt.Errorf("Error: Service IP: %s conflicts with host network: %s", serviceNetwork.String(), ipNet.String()))
		}
		if serviceNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with service network: %s", ipNet.IP.String(), serviceNetwork.String()))
		}
	}

	// Ensure each host subnet is within the cluster network
	subnets, _, err := oc.Registry.GetSubnets()
	if err != nil {
		return fmt.Errorf("Error in initializing/fetching subnets: %v", err)
	}
	for _, sub := range subnets {
		subnetIP, _, err := net.ParseCIDR(sub.SubnetCIDR)
		if err != nil {
			errList = append(errList, fmt.Errorf("Failed to parse network address: %s", sub.SubnetCIDR))
			continue
		}
		if !clusterNetwork.Contains(subnetIP) {
			errList = append(errList, fmt.Errorf("Error: Existing node subnet: %s is not part of cluster network: %s", sub.SubnetCIDR, clusterNetwork.String()))
		}
	}

	// Ensure each service is within the services network
	services, _, err := oc.Registry.GetServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		if !serviceNetwork.Contains(net.ParseIP(svc.IP)) {
			errList = append(errList, fmt.Errorf("Error: Existing service with IP: %s is not part of service network: %s", svc.IP, serviceNetwork.String()))
		}
	}

	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) isClusterNetworkChanged(clusterNetworkCIDR string, hostBitsPerSubnet int, serviceNetworkCIDR string) (bool, error) {
	clusterNetwork, hostSubnetLength, serviceNetwork, err := oc.Registry.GetNetworkInfo()
	if err != nil {
		return false, err
	}
	if clusterNetworkCIDR != clusterNetwork.String() ||
		hostSubnetLength != hostBitsPerSubnet ||
		serviceNetworkCIDR != serviceNetwork.String() {
		return true, nil
	}
	return false, nil
}

func (oc *OvsController) StartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	// Validate command-line/config parameters
	hostBitsPerSubnet := int(clusterBitsPerSubnet)
	clusterNetwork, _, serviceNetwork, err := ValidateClusterNetwork(clusterNetworkCIDR, hostBitsPerSubnet, serviceNetworkCIDR)
	if err != nil {
		return err
	}

	changed, net_err := oc.isClusterNetworkChanged(clusterNetworkCIDR, hostBitsPerSubnet, serviceNetworkCIDR)
	if changed {
		if err := oc.validateNetworkConfig(clusterNetwork, serviceNetwork); err != nil {
			return err
		}
		if err := oc.Registry.UpdateClusterNetwork(clusterNetwork, hostBitsPerSubnet, serviceNetwork); err != nil {
			return err
		}
	} else if net_err != nil {
		if err := oc.Registry.CreateClusterNetwork(clusterNetwork, hostBitsPerSubnet, serviceNetwork); err != nil {
			return err
		}
	}

	if err := oc.pluginHooks.PluginStartMaster(clusterNetwork, clusterBitsPerSubnet); err != nil {
		return fmt.Errorf("Failed to start plugin: %v", err)
	}
	return nil
}

func (oc *OvsController) StartNode(mtu uint) error {
	// Assume we are working with IPv4
	clusterNetwork, err := oc.Registry.GetClusterNetwork()
	if err != nil {
		log.Errorf("Failed to obtain ClusterNetwork: %v", err)
		return err
	}

	ipt := iptables.New(kexec.New(), utildbus.New(), iptables.ProtocolIpv4)
	if err := SetupIptables(ipt, clusterNetwork.String()); err != nil {
		return fmt.Errorf("Failed to set up iptables: %v", err)
	}

	ipt.AddReloadFunc(func() {
		err := SetupIptables(ipt, clusterNetwork.String())
		if err != nil {
			log.Errorf("Error reloading iptables: %v\n", err)
		}
	})

	if err := oc.pluginHooks.PluginStartNode(mtu); err != nil {
		return fmt.Errorf("Failed to start plugin: %v", err)
	}

	oc.markPodNetworkReady()

	return nil
}

func (oc *OvsController) GetLocalPods(namespace string) ([]api.Pod, error) {
	return oc.Registry.GetRunningPods(oc.HostName, namespace)
}

func (oc *OvsController) markPodNetworkReady() {
	close(oc.podNetworkReady)
}

func (oc *OvsController) WaitForPodNetworkReady() error {
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

func (oc *OvsController) Stop() {
	close(oc.sig)
}

// Wait for ready signal from Watch interface for the given resource
// Closes the ready channel as we don't need it anymore after this point
func waitForWatchReadiness(ready chan bool, resourceName string) {
	timeout := time.Minute
	select {
	case <-ready:
		close(ready)
	case <-time.After(timeout):
		log.Fatalf("Watch for resource %s is not ready(timeout: %v)", resourceName, timeout)
	}
	return
}

type watchWatcher func(oc *OvsController, ready chan<- bool, start <-chan string)
type watchGetter func(registry *Registry) (interface{}, string, error)

// watchAndGetResource will fetch current items in etcd and watch for any new
// changes for the given resource.
// Supported resources: nodes, subnets, namespaces, services, netnamespaces, and pods.
//
// To avoid any potential race conditions during this process, these steps are followed:
// 1. Initiator(master/node): Watch for a resource as an async op, lets say WatchProcess
// 2. WatchProcess: When ready for watching, send ready signal to initiator
// 3. Initiator: Wait for watch resource to be ready
//    This is needed as step-1 is an asynchronous operation
// 4. WatchProcess: Collect new changes in the queue but wait for initiator
//    to indicate which version to start from
// 5. Initiator: Get existing items with their latest version for the resource
// 6. Initiator: Send version from step-5 to WatchProcess
// 7. WatchProcess: Ignore any items with version <= start version got from initiator on step-6
// 8. WatchProcess: Handle new changes
func (oc *OvsController) watchAndGetResource(resourceName string, watcher watchWatcher, getter watchGetter) (interface{}, error) {
	ready := make(chan bool)
	start := make(chan string)

	go watcher(oc, ready, start)
	waitForWatchReadiness(ready, strings.ToLower(resourceName))
	getOutput, version, err := getter(oc.Registry)
	if err != nil {
		return nil, err
	}

	start <- version

	return getOutput, nil
}

type FirewallRule struct {
	table string
	chain string
	args  []string
}

func SetupIptables(ipt iptables.Interface, clusterNetworkCIDR string) error {
	rules := []FirewallRule{
		{"nat", "POSTROUTING", []string{"-s", clusterNetworkCIDR, "!", "-d", clusterNetworkCIDR, "-j", "MASQUERADE"}},
		{"filter", "INPUT", []string{"-p", "udp", "-m", "multiport", "--dports", "4789", "-m", "comment", "--comment", "001 vxlan incoming", "-j", "ACCEPT"}},
		{"filter", "INPUT", []string{"-i", "tun0", "-m", "comment", "--comment", "traffic from docker for internet", "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-d", clusterNetworkCIDR, "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-s", clusterNetworkCIDR, "-j", "ACCEPT"}},
	}

	for _, rule := range rules {
		_, err := ipt.EnsureRule(iptables.Prepend, iptables.Table(rule.table), iptables.Chain(rule.chain), rule.args...)
		if err != nil {
			return err
		}
	}

	return nil
}

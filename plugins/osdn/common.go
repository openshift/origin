package osdn

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
)

type PluginHooks interface {
	PluginStartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error
	PluginStartNode(mtu uint) error
	UpdatePod(namespace string, name string, id kubetypes.DockerID) error
}

type OvsController struct {
	pluginHooks     PluginHooks
	Registry        *Registry
	localIP         string
	localSubnet     *api.Subnet
	hostName        string
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
	Setup(localSubnetCIDR, clusterNetworkCIDR, serviceNetworkCIDR string, mtu uint) error

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
			return err
		}
	}
	log.Infof("Self IP: %s.", selfIP)

	oc.pluginHooks = pluginHooks
	oc.Registry = registry
	oc.flowController = flowController
	oc.localIP = selfIP
	oc.hostName = hostname
	oc.VNIDMap = make(map[string]uint)
	oc.sig = make(chan struct{})
	oc.podNetworkReady = make(chan struct{})
	oc.adminNamespaces = make([]string, 0)
	oc.services = make(map[string]api.Service)

	return nil
}

func (oc *OvsController) validateClusterNetwork(networkCIDR string, subnetsInUse []string, hostIPNets []*net.IPNet) error {
	clusterIP, clusterIPNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return fmt.Errorf("Failed to parse network address: %s", networkCIDR)
	}

	errList := []error{}
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(clusterIP) {
			errList = append(errList, fmt.Errorf("Error: Cluster IP: %s conflicts with host network: %s", clusterIP.String(), ipNet.String()))
		}
		if clusterIPNet.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with cluster network: %s", ipNet.IP.String(), networkCIDR))
		}
	}

	for _, netStr := range subnetsInUse {
		subnetIP, _, err := net.ParseCIDR(netStr)
		if err != nil {
			errList = append(errList, fmt.Errorf("Failed to parse network address: %s", netStr))
			continue
		}
		if !clusterIPNet.Contains(subnetIP) {
			errList = append(errList, fmt.Errorf("Error: Existing node subnet: %s is not part of cluster network: %s", netStr, networkCIDR))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) validateServiceNetwork(networkCIDR string, hostIPNets []*net.IPNet) error {
	serviceIP, serviceIPNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return fmt.Errorf("Failed to parse network address: %s", networkCIDR)
	}

	errList := []error{}
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(serviceIP) {
			errList = append(errList, fmt.Errorf("Error: Service IP: %s conflicts with host network: %s", ipNet.String(), networkCIDR))
		}
		if serviceIPNet.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with service network: %s", ipNet.IP.String(), networkCIDR))
		}
	}

	services, _, err := oc.Registry.GetServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		if !serviceIPNet.Contains(net.ParseIP(svc.IP)) {
			errList = append(errList, fmt.Errorf("Error: Existing service with IP: %s is not part of service network: %s", svc.IP, networkCIDR))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) validateNetworkConfig(clusterNetworkCIDR, serviceNetworkCIDR string, subnetsInUse []string) error {
	// TODO: Instead of hardcoding 'tun0' and 'lbr0', get it from common place.
	// This will ensure both the kube/multitenant scripts and master validations use the same name.
	hostIPNets, err := netutils.GetHostIPNetworks([]string{"tun0", "lbr0"})
	if err != nil {
		return err
	}

	errList := []error{}
	if err := oc.validateClusterNetwork(clusterNetworkCIDR, subnetsInUse, hostIPNets); err != nil {
		errList = append(errList, err)
	}
	if err := oc.validateServiceNetwork(serviceNetworkCIDR, hostIPNets); err != nil {
		errList = append(errList, err)
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) StartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	// Any mismatch in cluster/service network is handled by WriteNetworkConfig
	// For any new cluster/service network, ensure existing node subnets belong
	// to the given cluster network and service IPs belong to the given service network
	if _, err := oc.Registry.GetClusterNetworkCIDR(); err != nil {
		subrange := make([]string, 0)
		subnets, _, err := oc.Registry.GetSubnets()
		if err != nil {
			log.Errorf("Error in initializing/fetching subnets: %v", err)
			return err
		}
		for _, sub := range subnets {
			subrange = append(subrange, sub.SubnetCIDR)
		}

		err = oc.validateNetworkConfig(clusterNetworkCIDR, serviceNetworkCIDR, subrange)
		if err != nil {
			return err
		}
	}

	if err := oc.Registry.WriteNetworkConfig(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR); err != nil {
		return err
	}

	if err := oc.pluginHooks.PluginStartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR); err != nil {
		return fmt.Errorf("Failed to start plugin: %v", err)
	}

	return nil
}

func (oc *OvsController) StartNode(mtu uint) error {
	// Assume we are working with IPv4
	clusterNetworkCIDR, err := oc.Registry.GetClusterNetworkCIDR()
	if err != nil {
		log.Errorf("Failed to obtain ClusterNetwork: %v", err)
		return err
	}

	ipt := iptables.New(kexec.New(), utildbus.New(), iptables.ProtocolIpv4)
	if err := SetupIptables(ipt, clusterNetworkCIDR); err != nil {
		return fmt.Errorf("Failed to set up iptables: %v", err)
	}

	ipt.AddReloadFunc(func() {
		err := SetupIptables(ipt, clusterNetworkCIDR)
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

// +build linux

package node

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/golang/glog"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	"github.com/openshift/origin/pkg/util/netutils"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
	"k8s.io/kubernetes/pkg/util/sysctl"

	"github.com/vishvananda/netlink"
)

func (plugin *OsdnNode) getLocalSubnet() (string, error) {
	var subnet *networkapi.HostSubnet
	// If the HostSubnet doesn't already exist, it will be created by the SDN master in
	// response to the kubelet registering itself with the master (which should be
	// happening in another goroutine in parallel with this). Sometimes this takes
	// unexpectedly long though, so give it plenty of time before returning an error
	// (since that will cause the node process to exit).
	backoff := utilwait.Backoff{
		// A bit over 1 minute total
		Duration: time.Second,
		Factor:   1.5,
		Steps:    8,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		subnet, err = plugin.networkClient.Network().HostSubnets().Get(plugin.hostName, metav1.GetOptions{})
		if err == nil {
			return true, nil
		} else if kapierrors.IsNotFound(err) {
			glog.Warningf("Could not find an allocated subnet for node: %s, Waiting...", plugin.hostName)
			return false, nil
		} else {
			return false, err
		}
	})
	if err != nil {
		return "", fmt.Errorf("failed to get subnet for this host: %s, error: %v", plugin.hostName, err)
	}

	if err = plugin.networkInfo.ValidateNodeIP(subnet.HostIP); err != nil {
		return "", fmt.Errorf("failed to validate own HostSubnet: %v", err)
	}

	return subnet.Subnet, nil
}

func (plugin *OsdnNode) alreadySetUp(localSubnetGatewayCIDR string, clusterNetworkCIDR []string) bool {
	var found bool

	l, err := netlink.LinkByName(Tun0)
	if err != nil {
		return false
	}

	addrs, err := netlink.AddrList(l, syscall.AF_INET)
	if err != nil {
		return false
	}
	found = false
	for _, addr := range addrs {
		if addr.IPNet.String() == localSubnetGatewayCIDR {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	routes, err := netlink.RouteList(l, syscall.AF_INET)
	if err != nil {
		return false
	}
	for _, clusterCIDR := range clusterNetworkCIDR {
		found = false
		for _, route := range routes {
			if route.Dst != nil && route.Dst.String() == clusterCIDR {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if !plugin.oc.AlreadySetUp() {
		return false
	}

	return true
}

func deleteLocalSubnetRoute(device, localSubnetCIDR string) {
	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.25,
		Steps:    6,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		l, err := netlink.LinkByName(device)
		if err != nil {
			return false, fmt.Errorf("could not get interface %s: %v", device, err)
		}
		routes, err := netlink.RouteList(l, syscall.AF_INET)
		if err != nil {
			return false, fmt.Errorf("could not get routes: %v", err)
		}
		for _, route := range routes {
			if route.Dst != nil && route.Dst.String() == localSubnetCIDR {
				err = netlink.RouteDel(&route)
				if err != nil {
					return false, fmt.Errorf("could not delete route: %v", err)
				}
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		glog.Errorf("Error removing %s route from dev %s: %v; if the route appears later it will not be deleted.", localSubnetCIDR, device, err)
	}
}

func (plugin *OsdnNode) SetupSDN() (bool, error) {
	// Make sure IPv4 forwarding state is 1
	sysctl := sysctl.New()
	val, err := sysctl.GetSysctl("net/ipv4/ip_forward")
	if err != nil {
		return false, fmt.Errorf("could not get IPv4 forwarding state: %s", err)
	}
	if val != 1 {
		return false, fmt.Errorf("net/ipv4/ip_forward=0, it must be set to 1")
	}

	var clusterNetworkCIDRs []string
	for _, cn := range plugin.networkInfo.ClusterNetworks {
		clusterNetworkCIDRs = append(clusterNetworkCIDRs, cn.ClusterCIDR.String())
	}

	localSubnetCIDR := plugin.localSubnetCIDR
	_, ipnet, err := net.ParseCIDR(localSubnetCIDR)
	if err != nil {
		return false, fmt.Errorf("invalid local subnet CIDR: %v", err)
	}
	localSubnetMaskLength, _ := ipnet.Mask.Size()
	localSubnetGateway := netutils.GenerateDefaultGateway(ipnet).String()

	glog.V(5).Infof("[SDN setup] node pod subnet %s gateway %s", ipnet.String(), localSubnetGateway)

	exec := kexec.New()

	if plugin.clearLbr0IptablesRule {
		// Delete docker's left-over lbr0 rule; cannot do this from
		// NewNodePlugin (where docker is cleaned up) because we need
		// localSubnetCIDR which is only valid after plugin start
		ipt := iptables.New(exec, utildbus.New(), iptables.ProtocolIpv4)
		ipt.DeleteRule(iptables.TableNAT, iptables.ChainPostrouting, "-s", localSubnetCIDR, "!", "-o", "lbr0", "-j", "MASQUERADE")
	}

	gwCIDR := fmt.Sprintf("%s/%d", localSubnetGateway, localSubnetMaskLength)

	if err := waitForOVS(ovsDialDefaultNetwork, ovsDialDefaultAddress); err != nil {
		return false, err
	}

	var changed bool
	if plugin.alreadySetUp(gwCIDR, clusterNetworkCIDRs) {
		glog.V(5).Infof("[SDN setup] no SDN setup required")
	} else {
		glog.Infof("[SDN setup] full SDN setup required")
		if err := plugin.setup(clusterNetworkCIDRs, localSubnetCIDR, localSubnetGateway, gwCIDR); err != nil {
			return false, err
		}
		changed = true
	}

	// TODO: make it possible to safely reestablish node configuration after restart
	// If OVS goes down and fails the health check, restart the entire process
	healthFn := func() bool { return plugin.alreadySetUp(gwCIDR, clusterNetworkCIDRs) }
	runOVSHealthCheck(ovsDialDefaultNetwork, ovsDialDefaultAddress, healthFn)

	return changed, nil
}

func (plugin *OsdnNode) setup(clusterNetworkCIDRs []string, localSubnetCIDR, localSubnetGateway, gwCIDR string) error {
	serviceNetworkCIDR := plugin.networkInfo.ServiceNetwork.String()

	if err := plugin.oc.SetupOVS(clusterNetworkCIDRs, serviceNetworkCIDR, localSubnetCIDR, localSubnetGateway); err != nil {
		return err
	}

	l, err := netlink.LinkByName(Tun0)
	if err == nil {
		gwIP, _ := netlink.ParseIPNet(gwCIDR)
		err = netlink.AddrAdd(l, &netlink.Addr{IPNet: gwIP})
		if err == nil {
			defer deleteLocalSubnetRoute(Tun0, localSubnetCIDR)
		}
	}
	if err == nil {
		err = netlink.LinkSetMTU(l, int(plugin.mtu))
	}
	if err == nil {
		err = netlink.LinkSetUp(l)
	}
	if err == nil {
		for _, clusterNetwork := range plugin.networkInfo.ClusterNetworks {
			route := &netlink.Route{
				LinkIndex: l.Attrs().Index,
				Scope:     netlink.SCOPE_LINK,
				Dst:       clusterNetwork.ClusterCIDR,
			}
			if err = netlink.RouteAdd(route); err != nil {
				return err
			}
		}
	}
	if err == nil {
		route := &netlink.Route{
			LinkIndex: l.Attrs().Index,
			Dst:       plugin.networkInfo.ServiceNetwork,
		}
		err = netlink.RouteAdd(route)
	}
	if err != nil {
		return err
	}

	return nil
}

func (plugin *OsdnNode) updateEgressNetworkPolicyRules(vnid uint32) {
	policies := plugin.egressPolicies[vnid]
	namespaces := plugin.policy.GetNamespaces(vnid)
	if err := plugin.oc.UpdateEgressNetworkPolicyRules(policies, vnid, namespaces, plugin.egressDNS); err != nil {
		glog.Errorf("Error updating OVS flows for EgressNetworkPolicy: %v", err)
	}
}

func (plugin *OsdnNode) AddHostSubnetRules(subnet *networkapi.HostSubnet) {
	glog.Infof("AddHostSubnetRules for %s", common.HostSubnetToString(subnet))
	if err := plugin.oc.AddHostSubnetRules(subnet); err != nil {
		glog.Errorf("Error adding OVS flows for subnet %q: %v", subnet.Subnet, err)
	}
}

func (plugin *OsdnNode) DeleteHostSubnetRules(subnet *networkapi.HostSubnet) {
	glog.Infof("DeleteHostSubnetRules for %s", common.HostSubnetToString(subnet))
	if err := plugin.oc.DeleteHostSubnetRules(subnet); err != nil {
		glog.Errorf("Error deleting OVS flows for subnet %q: %v", subnet.Subnet, err)
	}
}

func (plugin *OsdnNode) AddServiceRules(service *kapi.Service, netID uint32) {
	glog.V(5).Infof("AddServiceRules for %v", service)
	if err := plugin.oc.AddServiceRules(service, netID); err != nil {
		glog.Errorf("Error adding OVS flows for service %v, netid %d: %v", service, netID, err)
	}
}

func (plugin *OsdnNode) DeleteServiceRules(service *kapi.Service) {
	glog.V(5).Infof("DeleteServiceRules for %v", service)
	if err := plugin.oc.DeleteServiceRules(service); err != nil {
		glog.Errorf("Error deleting OVS flows for service %v: %v", service, err)
	}
}

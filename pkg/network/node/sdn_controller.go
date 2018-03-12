// +build linux

package node

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/golang/glog"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	"github.com/openshift/origin/pkg/util/netutils"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/util/sysctl"

	"github.com/vishvananda/netlink"
)

func (plugin *OsdnNode) alreadySetUp(localSubnetGatewayCIDR string, clusterNetworkCIDR []string) error {
	var found bool

	l, err := netlink.LinkByName(Tun0)
	if err != nil {
		return err
	}

	addrs, err := netlink.AddrList(l, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	found = false
	for _, addr := range addrs {
		if addr.IPNet.String() == localSubnetGatewayCIDR {
			found = true
			break
		}
	}
	if !found {
		return errors.New("local subnet gateway CIDR not found")
	}

	routes, err := netlink.RouteList(l, netlink.FAMILY_V4)
	if err != nil {
		return err
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
			return errors.New("cluster CIDR not found")
		}
	}

	if !plugin.oc.AlreadySetUp() {
		return errors.New("plugin is not setup")
	}

	return nil
}

func deleteLocalSubnetRoute(device, localSubnetCIDR string) {
	// ~1 sec total
	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.25,
		Steps:    7,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		l, err := netlink.LinkByName(device)
		if err != nil {
			return false, fmt.Errorf("could not get interface %s: %v", device, err)
		}
		routes, err := netlink.RouteList(l, netlink.FAMILY_V4)
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
		utilruntime.HandleError(fmt.Errorf("Error removing %s route from dev %s: %v; if the route appears later it will not be deleted.", localSubnetCIDR, device, err))
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

	gwCIDR := fmt.Sprintf("%s/%d", localSubnetGateway, localSubnetMaskLength)

	if err := waitForOVS(ovsDialDefaultNetwork, ovsDialDefaultAddress); err != nil {
		return false, err
	}

	var changed bool
	if err := plugin.alreadySetUp(gwCIDR, clusterNetworkCIDRs); err == nil {
		glog.V(5).Infof("[SDN setup] no SDN setup required")
	} else {
		glog.Infof("[SDN setup] full SDN setup required (%v)", err)
		if err := plugin.setup(clusterNetworkCIDRs, localSubnetCIDR, localSubnetGateway, gwCIDR); err != nil {
			return false, err
		}
		changed = true
	}

	// TODO: make it possible to safely reestablish node configuration after restart
	// If OVS goes down and fails the health check, restart the entire process
	healthFn := func() error { return plugin.alreadySetUp(gwCIDR, clusterNetworkCIDRs) }
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
		utilruntime.HandleError(fmt.Errorf("Error updating OVS flows for EgressNetworkPolicy: %v", err))
	}
}

func (plugin *OsdnNode) AddHostSubnetRules(subnet *networkapi.HostSubnet) {
	glog.Infof("AddHostSubnetRules for %s", common.HostSubnetToString(subnet))
	if err := plugin.oc.AddHostSubnetRules(subnet); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error adding OVS flows for subnet %q: %v", subnet.Subnet, err))
	}
}

func (plugin *OsdnNode) DeleteHostSubnetRules(subnet *networkapi.HostSubnet) {
	glog.Infof("DeleteHostSubnetRules for %s", common.HostSubnetToString(subnet))
	if err := plugin.oc.DeleteHostSubnetRules(subnet); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error deleting OVS flows for subnet %q: %v", subnet.Subnet, err))
	}
}

func (plugin *OsdnNode) AddServiceRules(service *kapi.Service, netID uint32) {
	glog.V(5).Infof("AddServiceRules for %v", service)
	if err := plugin.oc.AddServiceRules(service, netID); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error adding OVS flows for service %v, netid %d: %v", service, netID, err))
	}
}

func (plugin *OsdnNode) DeleteServiceRules(service *kapi.Service) {
	glog.V(5).Infof("DeleteServiceRules for %v", service)
	if err := plugin.oc.DeleteServiceRules(service); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error deleting OVS flows for service %v: %v", service, err))
	}
}

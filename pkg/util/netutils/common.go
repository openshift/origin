package netutils

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/golang/glog"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
)

func IPToUint32(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip.To4())
}

func Uint32ToIP(u uint32) net.IP {
	ip := make([]byte, 4)
	binary.BigEndian.PutUint32(ip, u)
	return net.IPv4(ip[0], ip[1], ip[2], ip[3])
}

// Generate the default gateway IP Address for a subnet
func GenerateDefaultGateway(sna *net.IPNet) net.IP {
	ip := sna.IP.To4()
	return net.IPv4(ip[0], ip[1], ip[2], ip[3]|0x1)
}

// Return Host IP Networks
// Ignores provided interfaces and filters loopback and non IPv4 addrs.
func GetHostIPNetworks(skipInterfaces []string) ([]*net.IPNet, []net.IP, error) {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}

	skipInterfaceMap := make(map[string]bool)
	for _, ifaceName := range skipInterfaces {
		skipInterfaceMap[ifaceName] = true
	}

	errList := []error{}
	var hostIPNets []*net.IPNet
	var hostIPs []net.IP
	for _, iface := range hostInterfaces {
		if skipInterfaceMap[iface.Name] {
			continue
		}

		ifAddrs, err := iface.Addrs()
		if err != nil {
			errList = append(errList, err)
			continue
		}
		for _, addr := range ifAddrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil {
				errList = append(errList, err)
				continue
			}

			// Skip loopback and non IPv4 addrs
			if !ip.IsLoopback() && ip.To4() != nil {
				hostIPNets = append(hostIPNets, ipNet)
				hostIPs = append(hostIPs, ip)
			}
		}
	}
	return hostIPNets, hostIPs, kerrors.NewAggregate(errList)
}

func GetNodeIP(nodeName string) (string, error) {
	ip := net.ParseIP(nodeName)
	if ip == nil {
		addrs, err := net.LookupIP(nodeName)
		if err != nil {
			return "", fmt.Errorf("Failed to lookup IP address for node %s: %v", nodeName, err)
		}
		for _, addr := range addrs {
			// Skip loopback and non IPv4 addrs
			if addr.IsLoopback() || addr.To4() == nil {
				glog.V(5).Infof("Skipping loopback/non-IPv4 addr: %q for node %s", addr.String(), nodeName)
				continue
			}
			ip = addr
			break
		}
	} else if ip.IsLoopback() || ip.To4() == nil {
		glog.V(5).Infof("Skipping loopback/non-IPv4 addr: %q for node %s", ip.String(), nodeName)
		ip = nil
	}

	if ip == nil || len(ip.String()) == 0 {
		return "", fmt.Errorf("Failed to obtain IP address from node name: %s", nodeName)
	}
	return ip.String(), nil
}

package netutils

import (
	"encoding/binary"
	"net"

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

func GetHostIPNetworks(skipInterfaces []string) ([]*net.IPNet, error) {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	errList := []error{}
	var hostIPNets []*net.IPNet

CheckValidInterfaces:
	for _, iface := range hostInterfaces {
		for _, skipIface := range skipInterfaces {
			if skipIface == iface.Name {
				continue CheckValidInterfaces
			}
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
			// Skip IP addrs that doesn't belong to IPv4
			if ip.To4() != nil {
				hostIPNets = append(hostIPNets, ipNet)
			}
		}
	}
	return hostIPNets, kerrors.NewAggregate(errList)
}

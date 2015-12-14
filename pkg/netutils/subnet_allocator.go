package netutils

import (
	"fmt"
	"net"
)

type SubnetAllocator struct {
	network  *net.IPNet
	hostBits uint
	next     uint32
	allocMap map[string]bool
}

func NewSubnetAllocator(network string, hostBits uint, inUse []string) (*SubnetAllocator, error) {
	_, netIP, err := net.ParseCIDR(network)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse network address: %q", network)
	}

	netMaskSize, _ := netIP.Mask.Size()
	if hostBits > (32 - uint(netMaskSize)) {
		return nil, fmt.Errorf("Subnet capacity cannot be larger than number of networks available.")
	}

	amap := make(map[string]bool)
	for _, netStr := range inUse {
		_, nIp, err := net.ParseCIDR(netStr)
		if err != nil {
			fmt.Println("Failed to parse network address: ", netStr)
			continue
		}
		if !netIP.Contains(nIp.IP) {
			fmt.Println("Provided subnet doesn't belong to network: ", nIp)
			continue
		}
		amap[nIp.String()] = true
	}
	return &SubnetAllocator{network: netIP, hostBits: hostBits, next: 0, allocMap: amap}, nil
}

func (sna *SubnetAllocator) GetNetwork() (*net.IPNet, error) {
	var (
		numSubnets    uint32
		numSubnetBits uint
	)
	baseipu := IPToUint32(sna.network.IP)
	netMaskSize, _ := sna.network.Mask.Size()
	numSubnetBits = 32 - uint(netMaskSize) - sna.hostBits
	numSubnets = 1 << numSubnetBits

	var i uint32
	for i = 0; i < numSubnets; i++ {
		n := (i + sna.next) % numSubnets
		shifted := n << sna.hostBits
		ipu := baseipu | shifted
		genIp := Uint32ToIP(ipu)
		genSubnet := &net.IPNet{IP: genIp, Mask: net.CIDRMask(int(numSubnetBits)+netMaskSize, 32)}
		if !sna.allocMap[genSubnet.String()] {
			sna.allocMap[genSubnet.String()] = true
			sna.next = n + 1
			return genSubnet, nil
		}
	}

	sna.next = 0
	return nil, fmt.Errorf("No subnets available.")
}

func (sna *SubnetAllocator) ReleaseNetwork(ipnet *net.IPNet) error {
	if !sna.network.Contains(ipnet.IP) {
		return fmt.Errorf("Provided subnet %v doesn't belong to the network %v.", ipnet, sna.network)
	}

	ipnetStr := ipnet.String()
	if !sna.allocMap[ipnetStr] {
		return fmt.Errorf("Provided subnet %v is already available.", ipnet)
	}

	sna.allocMap[ipnetStr] = false

	return nil
}

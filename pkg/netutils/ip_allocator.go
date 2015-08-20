package netutils

import (
	"fmt"
	"net"

	log "github.com/golang/glog"
)

type IPAllocator struct {
	network  *net.IPNet
	allocMap map[string]bool
}

func NewIPAllocator(network string, inUse []string) (*IPAllocator, error) {
	_, netIP, err := net.ParseCIDR(network)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse network address: %q", network)
	}

	amap := make(map[string]bool)
	for _, netStr := range inUse {
		_, nIp, err := net.ParseCIDR(netStr)
		if err != nil {
			log.Errorf("Failed to parse network address: %s", netStr)
			continue
		}
		if !netIP.Contains(nIp.IP) {
			log.Errorf("Provided subnet doesn't belong to network: %s", nIp)
			continue
		}
		amap[netStr] = true
	}

	// Add the network address to the map
	amap[netIP.String()] = true
	return &IPAllocator{network: netIP, allocMap: amap}, nil
}

func (ipa *IPAllocator) GetIP() (*net.IPNet, error) {
	var (
		numIPs    uint32
		numIPBits uint
	)
	baseipu := IPToUint32(ipa.network.IP)
	netMaskSize, _ := ipa.network.Mask.Size()
	numIPBits = 32 - uint(netMaskSize)
	numIPs = 1 << numIPBits

	var i uint32
	// We exclude the last address as it is reserved for broadcast
	for i = 0; i < numIPs-1; i++ {
		ipu := baseipu | i
		genIP := &net.IPNet{IP: Uint32ToIP(ipu), Mask: net.CIDRMask(netMaskSize, 32)}
		if !ipa.allocMap[genIP.String()] {
			ipa.allocMap[genIP.String()] = true
			return genIP, nil
		}
	}

	return nil, fmt.Errorf("No IPs available")
}

func (ipa *IPAllocator) ReleaseIP(ip *net.IPNet) error {
	if !ipa.network.Contains(ip.IP) {
		return fmt.Errorf("Provided IP %v doesn't belong to the network %v", ip, ipa.network)
	}

	ipStr := ip.String()
	if !ipa.allocMap[ipStr] {
		return fmt.Errorf("Provided IP %v is already available", ip)
	}

	ipa.allocMap[ipStr] = false

	return nil
}

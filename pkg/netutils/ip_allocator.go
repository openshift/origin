package netutils

import (
	"fmt"
	"net"
)

type IPAllocator struct {
	network  *net.IPNet
	allocMap map[string]bool
}

func NewIPAllocator(network string) (*IPAllocator, error) {
	_, netIP, err := net.ParseCIDR(network)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse network address: %q", network)
	}

	amap := make(map[string]bool)
	return &IPAllocator{network: netIP, allocMap: amap}, nil
}

func (ipa *IPAllocator) GetIP() (net.IP, error) {
	fmt.Println(ipa.network.IP)
	return ipa.network.IP, nil
}

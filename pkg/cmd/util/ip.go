package util

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/golang/glog"
)

// ErrorNoDefaultIP is returned when no suitable non-loopback address can be found.
var ErrorNoDefaultIP = errors.New("no suitable IP address")

// DefaultLocalIP4 returns an IPv4 address that this host can be reached
// on. Will return NoDefaultIP if no suitable address can be found.
func DefaultLocalIP4() (net.IP, error) {
	devices, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, dev := range devices {
		if (dev.Flags&net.FlagUp != 0) && (dev.Flags&net.FlagLoopback == 0) {
			addrs, err := dev.Addrs()
			if err != nil {
				continue
			}
			for i := range addrs {
				if ip, ok := addrs[i].(*net.IPNet); ok {
					if ip.IP.To4() != nil {
						return ip.IP, nil
					}
				}
			}
		}
	}
	return nil, ErrorNoDefaultIP
}

// AllLocalIP4 returns all the IPv4 addresses that this host can be reached
// on.
func AllLocalIP4() ([]net.IP, error) {
	devices, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ret := []net.IP{}
	for _, dev := range devices {
		if dev.Flags&net.FlagUp != 0 {
			addrs, err := dev.Addrs()
			if err != nil {
				continue
			}
			for i := range addrs {
				if ip, ok := addrs[i].(*net.IPNet); ok {
					if ip.IP.To4() != nil {
						ret = append(ret, ip.IP)
					}
				}
			}
		}
	}
	return ret, nil
}

// Validate given node IP belongs to the current host
// Copied from "k8s.io/kubernetes/pkg/kubelet" as this was an internal method.
func ValidateNodeIP(nodeIP net.IP) error {
	if nodeIP.To4() == nil && nodeIP.To16() == nil {
		return fmt.Errorf("nodeIP must be a valid IP address")
	}
	if nodeIP.IsLoopback() {
		return fmt.Errorf("nodeIP can't be loopback address")
	}
	if nodeIP.IsMulticast() {
		return fmt.Errorf("nodeIP can't be a multicast address")
	}
	if nodeIP.IsLinkLocalUnicast() {
		return fmt.Errorf("nodeIP can't be a link-local unicast address")
	}
	if nodeIP.IsUnspecified() {
		return fmt.Errorf("nodeIP can't be an all zeros address")
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && ip.Equal(nodeIP) {
			return nil
		}
	}
	return fmt.Errorf("nodeIP %q not found in the host's network interfaces", nodeIP.String())
}

// GetHostname returns OS's hostname if 'hostnameOverride' is empty; otherwise, return 'hostnameOverride'.
// Copied from "k8s.io/kubernetes/pkg/util/node" instead of import as that will significantly increase
// the size of the openshift-node-config binary.
func GetHostname(hostnameOverride string) string {
	hostname := hostnameOverride
	if hostname == "" {
		nodename, err := os.Hostname()
		if err != nil {
			glog.Fatalf("couldn't determine hostname: %v", err)
		}
		hostname = nodename
	}
	return strings.ToLower(strings.TrimSpace(hostname))
}

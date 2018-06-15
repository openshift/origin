package dockerhelper

import (
	"net"

	"github.com/docker/docker/api/types"
	"github.com/golang/glog"
)

// RegistryConfig contains useful Docker registry information.
// Built by NewRegistryConfig constructor
type RegistryConfig struct {
	ipv4Ranges IPV4RangeList
}

// NewRegistryConfig creates a new registry configuration
func NewRegistryConfig(dockerInfo *types.Info) *RegistryConfig {
	ipv4Ranges := make(IPV4RangeList, 0)
	for _, cidr := range dockerInfo.RegistryConfig.InsecureRegistryCIDRs {
		ipv4Ranges = append(ipv4Ranges, fromCIDR((*net.IPNet)(cidr)))
	}
	return &RegistryConfig{ipv4Ranges}
}

// HasCustomInsecureRegistryCIDRs returns whether the user has any CIDRs configured
// within the Docker insecure registry configuration.
// This omits the inferred loopback entry in place implicitly by Docker.
func (config *RegistryConfig) HasCustomInsecureRegistryCIDRs() bool {
	count := len(config.ipv4Ranges)
	glog.V(5).Infof("Contains %d --insecure-registry entries", count)
	//Docker always includes 127.0.0.0/8
	return count > 1
}

// ContainsInsecureRegistryCIDR returns whether a given CIDR is contained within the bounds of at least one
// CIDR configured in the Docker insecure registry configuration.
func (config *RegistryConfig) ContainsInsecureRegistryCIDR(cidr string) (bool, error) {
	glog.V(5).Infof("Looking if any %#v contains CIDR %q", config.ipv4Ranges, cidr)
	_, candidateCIDR, err := net.ParseCIDR(cidr)
	if err != nil {
		glog.V(2).Infof("Issue parsing CIDR %q", cidr)
		return false, err
	}
	candidateIPRange := fromCIDR(candidateCIDR)
	if config.ipv4Ranges.Contains(candidateIPRange) {
		return true, nil
	}
	glog.V(5).Infof("CIDR %q did not fit in the registry CIDR range", cidr)
	return false, nil
}

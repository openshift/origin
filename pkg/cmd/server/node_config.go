package server

import (
	"net"

	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

func (cfg Config) BuildKubernetesNodeConfig() (*kubernetes.NodeConfig, error) {
	kubernetesAddr, err := cfg.GetKubernetesAddress()
	if err != nil {
		return nil, err
	}
	kubeClient, _, err := cfg.GetKubeClient()
	if err != nil {
		return nil, err
	}

	dnsDomain := env("OPENSHIFT_DNS_DOMAIN", "local")
	dnsIP := cfg.ClusterDNS
	if clusterDNS := env("OPENSHIFT_DNS_ADDR", ""); len(clusterDNS) > 0 {
		dnsIP = net.ParseIP(clusterDNS)
	}

	// define a function for resolving components to names
	imageResolverFn := cfg.ImageTemplate.ExpandOrDie

	nodeConfig := &kubernetes.NodeConfig{
		BindHost:   cfg.BindAddr.Host,
		NodeHost:   cfg.Hostname,
		MasterHost: kubernetesAddr.String(),

		ClusterDomain: dnsDomain,
		ClusterDNS:    dnsIP,

		VolumeDir: cfg.VolumeDir,

		NetworkContainerImage: imageResolverFn("pod"),

		AllowDisabledDocker: cfg.StartNode && cfg.StartMaster,

		Client: kubeClient,
	}

	return nodeConfig, nil
}

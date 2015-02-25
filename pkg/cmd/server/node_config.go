package server

import (
	_ "net/http/pprof"

	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

func (cfg Config) BuildKubernetesNodeConfig() (*kubernetes.NodeConfig, error) {
	masterAddr, err := cfg.GetMasterAddress()
	if err != nil {
		return nil, err
	}
	kubeClient, _, err := cfg.GetKubeClient()
	if err != nil {
		return nil, err
	}

	// define a function for resolving components to names
	imageResolverFn := cfg.ImageTemplate.ExpandOrDie

	nodeConfig := &kubernetes.NodeConfig{
		BindHost:   cfg.BindAddr.Host,
		NodeHost:   cfg.Hostname,
		MasterHost: masterAddr.String(),

		VolumeDir: cfg.VolumeDir,

		NetworkContainerImage: imageResolverFn("pod"),

		AllowDisabledDocker: cfg.StartNode && cfg.StartMaster,

		Client: kubeClient,
	}

	return nodeConfig, nil
}

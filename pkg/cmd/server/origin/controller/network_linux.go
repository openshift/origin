package controller

import (
	"fmt"

	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
)

type SDNControllerConfig struct {
	NetworkConfig configapi.MasterNetworkConfig
}

func (c *SDNControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	// TODO: Switch SDN to use client.Interface
	clientConfig, err := ctx.ClientBuilder.Config(bootstrappolicy.InfraSDNControllerServiceAccountName)
	if err != nil {
		return false, err
	}
	osClient, err := osclient.New(clientConfig)
	if err != nil {
		return false, err
	}
	kClient, err := kclientsetinternal.NewForConfig(clientConfig)
	if err != nil {
		return false, err
	}
	err = sdnplugin.StartMaster(
		c.NetworkConfig,
		osClient,
		kClient,
		ctx.InternalKubeInformers,
	)
	if err != nil {
		return false, fmt.Errorf("failed to start SDN plugin controller: %v", err)
	}
	return true, nil
}

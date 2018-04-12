package controller

import (
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	sccallocation "github.com/openshift/origin/pkg/security/controller"
	"github.com/openshift/origin/pkg/security/uid"
)

type NamespaceSecurityAllocationConfig struct {
	RequiredUIDRange *uid.Range
	MCS              sccallocation.MCSAllocationFunc
}

func (c *NamespaceSecurityAllocationConfig) RunController(ctx ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(bootstrappolicy.InfraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}
	securityClient, err := ctx.ClientBuilder.OpenshiftV1SecurityClient(bootstrappolicy.InfraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	controller := sccallocation.NewNamespaceSCCAllocationController(
		ctx.ExternalKubeInformers.Core().V1().Namespaces(),
		kubeClient.CoreV1().Namespaces(),
		securityClient.SecurityV1(),
		c.RequiredUIDRange,
		c.MCS,
	)
	go controller.Run(ctx.Stop)

	return true, nil
}

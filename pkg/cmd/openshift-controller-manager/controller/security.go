package controller

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	sccallocation "github.com/openshift/origin/pkg/security/controller"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
)

func RunNamespaceSecurityAllocationController(ctx *ControllerContext) (bool, error) {
	uidRange, err := uid.ParseRange(ctx.OpenshiftControllerConfig.SecurityAllocator.UIDAllocatorRange)
	if err != nil {
		return true, fmt.Errorf("unable to describe UID range: %v", err)
	}
	mcsRange, err := mcs.ParseRange(ctx.OpenshiftControllerConfig.SecurityAllocator.MCSAllocatorRange)
	if err != nil {
		return true, fmt.Errorf("unable to describe MCS category range: %v", err)
	}

	kubeClient, err := ctx.ClientBuilder.Client(bootstrappolicy.InfraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}
	securityClient, err := ctx.ClientBuilder.OpenshiftSecurityClient(bootstrappolicy.InfraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	controller := sccallocation.NewNamespaceSCCAllocationController(
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		kubeClient.CoreV1().Namespaces(),
		securityClient.SecurityV1(),
		uidRange,
		sccallocation.DefaultMCSAllocation(uidRange, mcsRange, ctx.OpenshiftControllerConfig.SecurityAllocator.MCSLabelsPerProject),
	)
	go controller.Run(ctx.Stop)

	return true, nil
}

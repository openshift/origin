package controller

import (
	"github.com/openshift/origin/pkg/authorization/controller/defaultrolebindings"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func RunDefaultRoleBindingController(ctx ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(bootstrappolicy.InfraDefaultRoleBindingsControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	go defaultrolebindings.NewDefaultRoleBindingsController(
		ctx.ExternalKubeInformers.Rbac().V1().RoleBindings(),
		ctx.ExternalKubeInformers.Core().V1().Namespaces(),
		kubeClient.RbacV1(),
	).Run(5, ctx.Stop)

	return true, nil
}

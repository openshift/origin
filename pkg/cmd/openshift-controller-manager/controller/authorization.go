package controller

import (
	"github.com/openshift/origin/pkg/authorization/controller/defaultrolebindings"
)

func RunDefaultRoleBindingController(ctx *ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(infraDefaultRoleBindingsControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	go defaultrolebindings.NewDefaultRoleBindingsController(
		ctx.KubernetesInformers.Rbac().V1().RoleBindings(),
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		kubeClient.RbacV1(),
	).Run(5, ctx.Stop)

	return true, nil
}

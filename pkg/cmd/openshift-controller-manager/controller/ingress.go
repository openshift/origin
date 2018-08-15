package controller

import (
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"

	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/route/controller/ingress"
)

func RunIngressToRouteController(ctx *ControllerContext) (bool, error) {
	clientConfig := ctx.ClientBuilder.ConfigOrDie(bootstrappolicy.InfraIngressToRouteControllerServiceAccountName)
	coreClient, err := coreclient.NewForConfig(clientConfig)
	if err != nil {
		return false, err
	}
	routeClient, err := routeclient.NewForConfig(clientConfig)
	if err != nil {
		return false, err
	}

	controller := ingress.NewController(
		coreClient,
		routeClient,
		ctx.KubernetesInformers.Extensions().V1beta1().Ingresses(),
		ctx.KubernetesInformers.Core().V1().Secrets(),
		ctx.KubernetesInformers.Core().V1().Services(),
		ctx.InternalRouteInformers.Route().V1().Routes(),
	)

	go controller.Run(5, ctx.Stop)

	return true, nil
}

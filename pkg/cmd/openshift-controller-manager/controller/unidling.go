package controller

import (
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/scale"

	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	unidlingcontroller "github.com/openshift/origin/pkg/unidling/controller"
)

func RunUnidlingController(ctx *ControllerContext) (bool, error) {
	// TODO this should be configurable
	resyncPeriod := 2 * time.Hour

	clientConfig := ctx.ClientBuilder.ConfigOrDie(infraUnidlingControllerServiceAccountName)
	appsClient, err := appsclient.NewForConfig(clientConfig)
	if err != nil {
		return false, err
	}

	// we don't use cached discovery because DiscoveryScaleKindResolver does its own caching,
	// so we want to re-fetch every time when we actually ask for it
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(appsClient.Discovery())
	scaleClient, err := scale.NewForConfig(clientConfig, ctx.RestMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return false, err
	}

	coreClient := ctx.ClientBuilder.ClientOrDie(infraUnidlingControllerServiceAccountName).CoreV1()
	controller := unidlingcontroller.NewUnidlingController(
		scaleClient,
		ctx.RestMapper,
		coreClient,
		coreClient,
		appsClient.AppsV1(),
		coreClient,
		resyncPeriod,
	)

	go controller.Run(ctx.Stop)

	return true, nil
}

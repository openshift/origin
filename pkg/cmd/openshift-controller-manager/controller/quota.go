package controller

import (
	"math/rand"
	"time"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotareconciliation"
	"github.com/openshift/origin/pkg/quota/image"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/quota/generic"
	quotainstall "k8s.io/kubernetes/pkg/quota/install"
)

func RunResourceQuotaManager(ctx ControllerContext) (bool, error) {
	concurrentResourceQuotaSyncs := int(ctx.OpenshiftControllerOptions.ResourceQuotaOptions.ConcurrentSyncs)
	resourceQuotaSyncPeriod := ctx.OpenshiftControllerOptions.ResourceQuotaOptions.SyncPeriod.Duration
	replenishmentSyncPeriodFunc := calculateResyncPeriod(ctx.OpenshiftControllerOptions.ResourceQuotaOptions.MinResyncPeriod.Duration)
	saName := "resourcequota-controller"
	listerFuncForResource := generic.ListerFuncForResourceFunc(ctx.GenericResourceInformer.ForResource)
	quotaConfiguration := quotainstall.NewQuotaConfigurationForControllers(listerFuncForResource)

	imageEvaluators := image.NewReplenishmentEvaluators(
		listerFuncForResource,
		ctx.ImageInformers.Image().InternalVersion().ImageStreams(),
		ctx.ClientBuilder.OpenshiftInternalImageClientOrDie(saName).Image())
	resourceQuotaRegistry := generic.NewRegistry(imageEvaluators)

	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		QuotaClient:               ctx.ClientBuilder.ClientOrDie(saName).Core(),
		ResourceQuotaInformer:     ctx.ExternalKubeInformers.Core().V1().ResourceQuotas(),
		ResyncPeriod:              controller.StaticResyncPeriodFunc(resourceQuotaSyncPeriod),
		Registry:                  resourceQuotaRegistry,
		ReplenishmentResyncPeriod: replenishmentSyncPeriodFunc,
		IgnoredResourcesFunc:      quotaConfiguration.IgnoredResources,
		InformersStarted:          ctx.InformersStarted,
		InformerFactory:           ctx.GenericResourceInformer,
	}
	controller, err := kresourcequota.NewResourceQuotaController(resourceQuotaControllerOptions)
	if err != nil {
		return true, err
	}
	go controller.Run(concurrentResourceQuotaSyncs, ctx.Stop)

	return true, nil
}

type ClusterQuotaReconciliationControllerConfig struct {
	DefaultResyncPeriod            time.Duration
	DefaultReplenishmentSyncPeriod time.Duration
}

func (c *ClusterQuotaReconciliationControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraClusterQuotaReconciliationControllerServiceAccountName

	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingController(
		ctx.ExternalKubeInformers.Core().V1().Namespaces(),
		ctx.QuotaInformers.Quota().InternalVersion().ClusterResourceQuotas())
	resourceQuotaControllerClient := ctx.ClientBuilder.ClientOrDie("resourcequota-controller")
	discoveryFunc := resourceQuotaControllerClient.Discovery().ServerPreferredNamespacedResources
	listerFuncForResource := generic.ListerFuncForResourceFunc(ctx.GenericResourceInformer.ForResource)
	quotaConfiguration := quotainstall.NewQuotaConfigurationForControllers(listerFuncForResource)

	// TODO make a union registry
	resourceQuotaRegistry := generic.NewRegistry(quotaConfiguration.Evaluators())
	imageEvaluators := image.NewReplenishmentEvaluators(
		listerFuncForResource,
		ctx.ImageInformers.Image().InternalVersion().ImageStreams(),
		ctx.ClientBuilder.OpenshiftInternalImageClientOrDie(saName).Image())
	for i := range imageEvaluators {
		resourceQuotaRegistry.Add(imageEvaluators[i])
	}

	options := clusterquotareconciliation.ClusterQuotaReconcilationControllerOptions{
		ClusterQuotaInformer: ctx.QuotaInformers.Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:   clusterQuotaMappingController.GetClusterQuotaMapper(),
		ClusterQuotaClient:   ctx.ClientBuilder.OpenshiftInternalQuotaClientOrDie(saName).Quota().ClusterResourceQuotas(),

		Registry:                  resourceQuotaRegistry,
		ResyncPeriod:              c.DefaultResyncPeriod,
		ReplenishmentResyncPeriod: controller.StaticResyncPeriodFunc(c.DefaultReplenishmentSyncPeriod),
		DiscoveryFunc:             discoveryFunc,
		IgnoredResourcesFunc:      quotaConfiguration.IgnoredResources,
		InformersStarted:          ctx.InformersStarted,
		InformerFactory:           ctx.GenericResourceInformer,
	}
	clusterQuotaReconciliationController, err := clusterquotareconciliation.NewClusterQuotaReconcilationController(options)
	if err != nil {
		return true, err
	}
	clusterQuotaMappingController.GetClusterQuotaMapper().AddListener(clusterQuotaReconciliationController)

	go clusterQuotaMappingController.Run(5, ctx.Stop)
	go clusterQuotaReconciliationController.Run(5, ctx.Stop)

	return true, nil
}

func calculateResyncPeriod(period time.Duration) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(period.Nanoseconds()) * factor)
	}
}

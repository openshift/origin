package controller

import (
	"math/rand"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	quotacontroller "github.com/openshift/origin/pkg/quota/controller"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotareconciliation"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"

	"github.com/openshift/origin/pkg/quota"
)

func RunResourceQuotaManager(ctx ControllerContext) (bool, error) {
	concurrentResourceQuotaSyncs := int(ctx.KubeControllerContext.Options.ConcurrentResourceQuotaSyncs)
	resourceQuotaSyncPeriod := ctx.KubeControllerContext.Options.ResourceQuotaSyncPeriod.Duration
	replenishmentSyncPeriodFunc := calculateResyncPeriod(ctx.KubeControllerContext.Options.MinResyncPeriod.Duration)
	saName := "resourcequota-controller"

	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(
		ctx.ExternalKubeInformers,
		ctx.ImageInformers.Image().InternalVersion().ImageStreams(),
		ctx.ClientBuilder.DeprecatedOpenshiftClientOrDie(saName),
		ctx.ClientBuilder.ClientOrDie(saName),
	)

	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		QuotaClient:           ctx.ClientBuilder.ClientOrDie(saName).Core(),
		ResourceQuotaInformer: ctx.ExternalKubeInformers.Core().V1().ResourceQuotas(),
		ResyncPeriod:          controller.StaticResyncPeriodFunc(resourceQuotaSyncPeriod),
		Registry:              resourceQuotaRegistry,
		GroupKindsToReplenish: quota.AllEvaluatedGroupKinds,
		ControllerFactory: quotacontroller.NewAllResourceReplenishmentControllerFactory(
			ctx.ExternalKubeInformers,
			ctx.ImageInformers.Image().InternalVersion().ImageStreams(),
			ctx.ClientBuilder.DeprecatedOpenshiftClientOrDie(saName),
		),
		ReplenishmentResyncPeriod: replenishmentSyncPeriodFunc,
	}
	go kresourcequota.NewResourceQuotaController(resourceQuotaControllerOptions).Run(concurrentResourceQuotaSyncs, ctx.Stop)

	return true, nil
}

type ClusterQuotaReconciliationControllerConfig struct {
	Mapper                         clusterquotamapping.ClusterQuotaMapper
	DefaultResyncPeriod            time.Duration
	DefaultReplenishmentSyncPeriod time.Duration
}

func (c *ClusterQuotaReconciliationControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraClusterQuotaReconciliationControllerServiceAccountName
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(
		ctx.ExternalKubeInformers,
		ctx.ImageInformers.Image().InternalVersion().ImageStreams(),
		ctx.ClientBuilder.DeprecatedOpenshiftClientOrDie(saName),
		ctx.ClientBuilder.ClientOrDie(saName),
	)
	groupKindsToReplenish := quota.AllEvaluatedGroupKinds

	options := clusterquotareconciliation.ClusterQuotaReconcilationControllerOptions{
		ClusterQuotaInformer: ctx.QuotaInformers.Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:   c.Mapper,
		ClusterQuotaClient:   ctx.ClientBuilder.DeprecatedOpenshiftClientOrDie(saName),

		Registry:     resourceQuotaRegistry,
		ResyncPeriod: c.DefaultResyncPeriod,
		ControllerFactory: quotacontroller.NewAllResourceReplenishmentControllerFactory(
			ctx.ExternalKubeInformers,
			ctx.ImageInformers.Image().InternalVersion().ImageStreams(),
			ctx.ClientBuilder.DeprecatedOpenshiftClientOrDie(saName),
		),
		ReplenishmentResyncPeriod: controller.StaticResyncPeriodFunc(c.DefaultReplenishmentSyncPeriod),
		GroupKindsToReplenish:     groupKindsToReplenish,
	}

	controller := clusterquotareconciliation.NewClusterQuotaReconcilationController(options)
	c.Mapper.AddListener(controller)
	go controller.Run(5, ctx.Stop)

	return true, nil
}

type ClusterQuotaMappingControllerConfig struct {
	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController
}

func (c *ClusterQuotaMappingControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	var syncOnce sync.Once
	syncOnce.Do(func() {
		go c.ClusterQuotaMappingController.Run(5, ctx.Stop)
	})
	return true, nil
}

func calculateResyncPeriod(period time.Duration) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(period.Nanoseconds()) * factor)
	}
}

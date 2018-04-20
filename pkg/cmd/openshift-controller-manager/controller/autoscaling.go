package controller

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/scale"
	hpacontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	hpametrics "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// NB: this is funky -- it's actually a Kubernetes controller, but we run it as an OpenShift controller in order
// to get a handle on OpenShift clients, so that our delegating scales getter can work.

// TODO this goes away with a truly generic autoscaler
func RunHorizontalPodAutoscalerController(originCtx ControllerContext) (bool, error) {
	heapsterNamespace := bootstrappolicy.DefaultOpenShiftInfraNamespace

	hpaClientConfig, err := originCtx.ClientBuilder.Config(bootstrappolicy.InfraHorizontalPodAutoscalerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	hpaClient, err := kubeclientset.NewForConfig(hpaClientConfig)
	if err != nil {
		return false, err
	}

	metricsClient := hpametrics.NewHeapsterMetricsClient(
		hpaClient,
		heapsterNamespace,
		"https",
		"heapster",
		"",
	)
	replicaCalc := hpacontroller.NewReplicaCalculator(
		metricsClient,
		hpaClient.CoreV1(),
		0.1, // this is the default
	)

	// TODO: we need something like deferred discovery REST mapper that calls invalidate
	// on cache misses.
	cachedDiscovery := discocache.NewMemCacheClient(hpaClient.Discovery())
	restMapper := discovery.NewDeferredDiscoveryRESTMapper(cachedDiscovery, apimeta.InterfacesForUnstructured)
	restMapper.Reset()
	// we don't use cached discovery because DiscoveryScaleKindResolver does its own caching,
	// so we want to re-fetch every time when we actually ask for it
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(hpaClient.Discovery())
	scaleClient, err := scale.NewForConfig(hpaClientConfig, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return false, err
	}

	go hpacontroller.NewHorizontalController(
		hpaClient.CoreV1(),
		scaleClient,
		hpaClient.AutoscalingV1(),
		restMapper,
		replicaCalc,
		originCtx.ExternalKubeInformers.Autoscaling().V1().HorizontalPodAutoscalers(),
		originCtx.OpenshiftControllerConfig.HPA.SyncPeriod.Duration,
		originCtx.OpenshiftControllerConfig.HPA.UpscaleForbiddenWindow.Duration,
		originCtx.OpenshiftControllerConfig.HPA.DownscaleForbiddenWindow.Duration,
	).Run(originCtx.Stop)

	return true, nil
}

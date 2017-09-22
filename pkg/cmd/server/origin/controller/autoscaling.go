package controller

import (
	clientgoclientset "k8s.io/client-go/kubernetes"
	kubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	hpacontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	hpametrics "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"

	appsv1client "github.com/openshift/origin/pkg/apps/client/v1"
	appstypedclient "github.com/openshift/origin/pkg/apps/generated/clientset/typed/apps/v1"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// NB: this is funky -- it's actually a Kubernetes controller, but we run it as an OpenShift controller in order
// to get a handle on OpenShift clients, so that our delegating scales getter can work.

type HorizontalPodAutoscalerControllerConfig struct {
	HeapsterNamespace string
}

func (c *HorizontalPodAutoscalerControllerConfig) RunController(originCtx ControllerContext) (bool, error) {
	ctx := originCtx.KubeControllerContext

	hpaClientConfig := ctx.ClientBuilder.ConfigOrDie(bootstrappolicy.InfraHorizontalPodAutoscalerControllerServiceAccountName)

	hpaClient, err := kubeclientset.NewForConfig(hpaClientConfig)
	if err != nil {
		return false, err
	}
	appsClient, err := appstypedclient.NewForConfig(hpaClientConfig)
	if err != nil {
		return false, err
	}
	hpaEventsClient, err := clientgoclientset.NewForConfig(hpaClientConfig)
	if err != nil {
		return false, err
	}

	metricsClient := hpametrics.NewHeapsterMetricsClient(
		hpaClient,
		c.HeapsterNamespace,
		"https",
		"heapster",
		"",
	)
	replicaCalc := hpacontroller.NewReplicaCalculator(metricsClient, hpaClient.Core())

	delegatingScalesGetter := appsv1client.NewDelegatingScaleNamespacer(appsClient, hpaClient.ExtensionsV1beta1())

	go hpacontroller.NewHorizontalController(
		hpaEventsClient.Core(),
		delegatingScalesGetter,
		hpaClient.Autoscaling(),
		replicaCalc,
		ctx.InformerFactory.Autoscaling().V1().HorizontalPodAutoscalers(),
		ctx.Options.HorizontalPodAutoscalerSyncPeriod.Duration,
		ctx.Options.HorizontalPodAutoscalerUpscaleForbiddenWindow.Duration,
		ctx.Options.HorizontalPodAutoscalerDownscaleForbiddenWindow.Duration,
	).Run(ctx.Stop)

	return true, nil
}

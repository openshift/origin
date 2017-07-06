package controller

import (
	hpacontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	hpametrics "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	clientgoclientset "k8s.io/client-go/kubernetes"
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
)

// NB: this is funky -- it's actually a Kubernetes controller, but we run it as an OpenShift controller in order
// to get a handle on OpenShift clients, so that our delegating scales getter can work.

type HorizontalPodAutoscalerControllerConfig struct {
	HeapsterNamespace string
}

func (c *HorizontalPodAutoscalerControllerConfig) RunController(ctx kubecontroller.ControllerContext) (bool, error) {
	hpaClientConfig := ctx.ClientBuilder.ConfigOrDie(bootstrappolicy.InfraHorizontalPodAutoscalerControllerServiceAccountName)

	hpaClient, err := kubeclientset.NewForConfig(hpaClientConfig)
	if err != nil {
		return false, err
	}

	// use the Kubernetes config so that the service account is in the same name namespace for both clients
	hpaOriginClient, err := osclient.New(hpaClientConfig)
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

	delegatingScalesGetter := osclient.NewDelegatingScaleNamespacer(hpaOriginClient, hpaClient.ExtensionsV1beta1())

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

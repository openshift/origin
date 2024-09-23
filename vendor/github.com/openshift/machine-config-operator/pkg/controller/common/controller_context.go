package common

import (
	"context"
	"math/rand"
	"time"

	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	operatorinformers "github.com/openshift/client-go/operator/informers/externalversions"
	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/machine-config-operator/internal/clients"
	daemonconsts "github.com/openshift/machine-config-operator/pkg/daemon/constants"
	mcfginformers "github.com/openshift/machine-config-operator/pkg/generated/informers/externalversions"
	"github.com/openshift/machine-config-operator/pkg/version"
	apiextinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
)

const (
	minResyncPeriod = 20 * time.Minute
)

func resyncPeriod() func() time.Duration {
	return func() time.Duration {
		// Disable gosec here to avoid throwing
		// G404: Use of weak random number generator (math/rand instead of crypto/rand)
		// #nosec
		factor := rand.Float64() + 1
		return time.Duration(float64(minResyncPeriod.Nanoseconds()) * factor)
	}
}

// DefaultResyncPeriod returns a function which generates a random resync period
func DefaultResyncPeriod() func() time.Duration {
	return resyncPeriod()
}

// ControllerContext stores all the informers for a variety of kubernetes objects.
type ControllerContext struct {
	ClientBuilder *clients.Builder

	NamespacedInformerFactory                           mcfginformers.SharedInformerFactory
	InformerFactory                                     mcfginformers.SharedInformerFactory
	KubeInformerFactory                                 informers.SharedInformerFactory
	KubeNamespacedInformerFactory                       informers.SharedInformerFactory
	OpenShiftConfigKubeNamespacedInformerFactory        informers.SharedInformerFactory
	OpenShiftKubeAPIServerKubeNamespacedInformerFactory informers.SharedInformerFactory
	APIExtInformerFactory                               apiextinformers.SharedInformerFactory
	ConfigInformerFactory                               configinformers.SharedInformerFactory
	OperatorInformerFactory                             operatorinformers.SharedInformerFactory
	KubeMAOSharedInformer                               informers.SharedInformerFactory

	FeatureGateAccess featuregates.FeatureGateAccess

	AvailableResources map[schema.GroupVersionResource]bool

	Stop <-chan struct{}

	InformersStarted chan struct{}

	ResyncPeriod func() time.Duration
}

// CreateControllerContext creates the ControllerContext with the ClientBuilder.
func CreateControllerContext(ctx context.Context, cb *clients.Builder, targetNamespace string) *ControllerContext {
	client := cb.MachineConfigClientOrDie("machine-config-shared-informer")
	kubeClient := cb.KubeClientOrDie("kube-shared-informer")
	apiExtClient := cb.APIExtClientOrDie("apiext-shared-informer")
	configClient := cb.ConfigClientOrDie("config-shared-informer")
	operatorClient := cb.OperatorClientOrDie("operator-shared-informer")
	sharedInformers := mcfginformers.NewSharedInformerFactory(client, resyncPeriod()())
	sharedNamespacedInformers := mcfginformers.NewFilteredSharedInformerFactory(client, resyncPeriod()(), targetNamespace, nil)
	kubeSharedInformer := informers.NewSharedInformerFactory(kubeClient, resyncPeriod()())
	kubeNamespacedSharedInformer := informers.NewFilteredSharedInformerFactory(kubeClient, resyncPeriod()(), targetNamespace, nil)
	openShiftConfigKubeNamespacedSharedInformer := informers.NewFilteredSharedInformerFactory(kubeClient, resyncPeriod()(), "openshift-config", nil)
	openShiftKubeAPIServerKubeNamespacedSharedInformer := informers.NewFilteredSharedInformerFactory(kubeClient,
		resyncPeriod()(),
		"openshift-kube-apiserver-operator",
		func(opt *metav1.ListOptions) {
			opt.FieldSelector = fields.OneTermEqualSelector("metadata.name", "kube-apiserver-to-kubelet-client-ca").String()
		},
	)
	// this is needed to listen for changes in MAO user data secrets to re-apply the ones we define in the MCO (since we manage them)
	kubeMAOSharedInformer := informers.NewFilteredSharedInformerFactory(kubeClient, resyncPeriod()(), "openshift-machine-api", nil)

	// filter out CRDs that do not have the MCO label
	assignFilterLabels := func(opts *metav1.ListOptions) {
		labelsMap, err := labels.ConvertSelectorToLabelsMap(opts.LabelSelector)
		if err != nil {
			klog.Warningf("unable to convert selector %q to map: %v", opts.LabelSelector, err)
			return
		}
		opts.LabelSelector = labels.Merge(labelsMap, map[string]string{daemonconsts.OpenShiftOperatorManagedLabel: ""}).String()
	}
	apiExtSharedInformer := apiextinformers.NewSharedInformerFactoryWithOptions(apiExtClient, resyncPeriod()(),
		apiextinformers.WithNamespace(targetNamespace), apiextinformers.WithTweakListOptions(assignFilterLabels))
	configSharedInformer := configinformers.NewSharedInformerFactory(configClient, resyncPeriod()())
	operatorSharedInformer := operatorinformers.NewSharedInformerFactory(operatorClient, resyncPeriod()())

	desiredVersion := version.ReleaseVersion
	missingVersion := "0.0.1-snapshot"

	controllerRef, err := events.GetControllerReferenceForCurrentPod(ctx, kubeClient, targetNamespace, nil)
	if err != nil {
		klog.Warningf("unable to get owner reference (falling back to namespace): %v", err)
	}

	recorder := events.NewKubeRecorder(kubeClient.CoreV1().Events(targetNamespace), "cloud-controller-manager-operator", controllerRef)

	// By default, this will exit(0) the process if the featuregates ever change to a different set of values.
	featureGateAccessor := featuregates.NewFeatureGateAccess(
		desiredVersion, missingVersion,
		configSharedInformer.Config().V1().ClusterVersions(), configSharedInformer.Config().V1().FeatureGates(),
		recorder,
	)
	go featureGateAccessor.Run(ctx)

	return &ControllerContext{
		ClientBuilder:                                       cb,
		NamespacedInformerFactory:                           sharedNamespacedInformers,
		InformerFactory:                                     sharedInformers,
		KubeInformerFactory:                                 kubeSharedInformer,
		KubeNamespacedInformerFactory:                       kubeNamespacedSharedInformer,
		OpenShiftConfigKubeNamespacedInformerFactory:        openShiftConfigKubeNamespacedSharedInformer,
		OpenShiftKubeAPIServerKubeNamespacedInformerFactory: openShiftKubeAPIServerKubeNamespacedSharedInformer,
		APIExtInformerFactory:                               apiExtSharedInformer,
		ConfigInformerFactory:                               configSharedInformer,
		OperatorInformerFactory:                             operatorSharedInformer,
		Stop:                                                ctx.Done(),
		InformersStarted:                                    make(chan struct{}),
		ResyncPeriod:                                        resyncPeriod(),
		KubeMAOSharedInformer:                               kubeMAOSharedInformer,
		FeatureGateAccess:                                   featureGateAccessor,
	}
}

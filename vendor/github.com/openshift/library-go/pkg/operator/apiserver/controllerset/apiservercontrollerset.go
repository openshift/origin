package apiservercontrollerset

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	openshiftconfigclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	"github.com/openshift/library-go/pkg/operator/apiserver/controller/apiservice"
	"github.com/openshift/library-go/pkg/operator/apiserver/controller/nsfinalizer"
	"github.com/openshift/library-go/pkg/operator/apiserver/controller/workload"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/loglevel"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticresourcecontroller"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/apimachinery/pkg/util/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	apiregistrationinformers "k8s.io/kube-aggregator/pkg/client/informers/externalversions"
)

type preparedAPIServerControllerSet struct {
	controllers []controller
}

type controllerWrapper struct {
	emptyAllowed bool
	controller
}

type controller interface {
	Run(ctx context.Context, workers int)
}

func (cw *controllerWrapper) prepare() (controller, error) {
	if !cw.emptyAllowed && cw.controller == nil {
		return nil, fmt.Errorf("missing controller")
	}

	return cw.controller, nil
}

// APIServerControllerSet is a set of controllers that maintain a deployment of
// an API server and the namespace it's running in
//
// TODO: add workload and encryption controllers
type APIServerControllerSet struct {
	operatorClient v1helpers.OperatorClient
	eventRecorder  events.Recorder

	apiServiceController            controllerWrapper
	clusterOperatorStatusController controllerWrapper
	configUpgradableController      controllerWrapper
	logLevelController              controllerWrapper
	finalizerController             controllerWrapper
	staticResourceController        controllerWrapper
	workloadController              controllerWrapper
}

func NewAPIServerControllerSet(
	operatorClient v1helpers.OperatorClient,
	eventRecorder events.Recorder,
) *APIServerControllerSet {
	apiServerControllerSet := &APIServerControllerSet{
		operatorClient: operatorClient,
		eventRecorder:  eventRecorder,
	}

	return apiServerControllerSet
}

// WithConfigUpgradableController adds a controller for the operator to check for presence of
// unsupported configuration and to set the Upgradable condition to false if it finds any
func (cs *APIServerControllerSet) WithConfigUpgradableController() *APIServerControllerSet {
	cs.configUpgradableController.controller = unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(cs.operatorClient, cs.eventRecorder)
	return cs
}

func (cs *APIServerControllerSet) WithoutConfigUpgradableController() *APIServerControllerSet {
	cs.configUpgradableController.controller = nil
	cs.configUpgradableController.emptyAllowed = true
	return cs
}

// WithLogLevelController adds a controller that configures logging for the operator
func (cs *APIServerControllerSet) WithLogLevelController() *APIServerControllerSet {
	cs.logLevelController.controller = loglevel.NewClusterOperatorLoggingController(cs.operatorClient, cs.eventRecorder)
	return cs
}

func (cs *APIServerControllerSet) WithoutLogLevelController() *APIServerControllerSet {
	cs.logLevelController.controller = nil
	cs.logLevelController.emptyAllowed = true
	return cs
}

func (cs *APIServerControllerSet) WithClusterOperatorStatusController(
	clusterOperatorName string,
	relatedObjects []configv1.ObjectReference,
	clusterOperatorClient configv1client.ClusterOperatorsGetter,
	clusterOperatorInformer configv1informers.ClusterOperatorInformer,
	versionRecorder status.VersionGetter,
) *APIServerControllerSet {
	cs.clusterOperatorStatusController.controller = status.NewClusterOperatorStatusController(
		clusterOperatorName,
		relatedObjects,
		clusterOperatorClient,
		clusterOperatorInformer,
		cs.operatorClient,
		versionRecorder,
		cs.eventRecorder,
	)
	return cs
}

func (cs *APIServerControllerSet) WithoutClusterOperatorStatusController() *APIServerControllerSet {
	cs.clusterOperatorStatusController.controller = nil
	cs.clusterOperatorStatusController.emptyAllowed = true
	return cs
}

func (cs *APIServerControllerSet) WithAPIServiceController(
	controllerName string,
	getAPIServicesToManageFn apiservice.GetAPIServicesToMangeFunc,
	apiregistrationInformers apiregistrationinformers.SharedInformerFactory,
	apiregistrationv1Client apiregistrationv1client.ApiregistrationV1Interface,
	kubeInformersForTargetNamesace kubeinformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
) *APIServerControllerSet {
	cs.apiServiceController.controller = apiservice.NewAPIServiceController(
		controllerName,
		getAPIServicesToManageFn,
		cs.operatorClient,
		apiregistrationInformers,
		apiregistrationv1Client,
		kubeInformersForTargetNamesace,
		kubeClient,
		cs.eventRecorder,
	)
	return cs
}

func (cs *APIServerControllerSet) WithoutAPIServiceController() *APIServerControllerSet {
	cs.apiServiceController.controller = nil
	cs.apiServiceController.emptyAllowed = true
	return cs
}

func (cs *APIServerControllerSet) WithFinalizerController(
	targetNamespace string,
	kubeInformersForTargetNamespace kubeinformers.SharedInformerFactory,
	namespaceGetter corev1client.NamespacesGetter,
) *APIServerControllerSet {
	cs.finalizerController.controller = nsfinalizer.NewFinalizerController(
		targetNamespace,
		kubeInformersForTargetNamespace,
		namespaceGetter,
		cs.eventRecorder,
	)
	return cs
}

func (cs *APIServerControllerSet) WithoutFinalizerController() *APIServerControllerSet {
	cs.finalizerController.controller = nil
	cs.finalizerController.emptyAllowed = true
	return cs
}

func (cs *APIServerControllerSet) WithStaticResourcesController(
	controllerName string,
	manifests resourceapply.AssetFunc,
	files []string,
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces,
	kubeClient kubernetes.Interface,
) *APIServerControllerSet {
	cs.staticResourceController.controller = staticresourcecontroller.NewStaticResourceController(
		controllerName,
		manifests,
		files,
		resourceapply.NewKubeClientHolder(kubeClient),
		cs.operatorClient,
		cs.eventRecorder,
	).AddKubeInformers(kubeInformersForNamespaces)

	return cs
}

func (cs *APIServerControllerSet) WithoutStaticResourcesController() *APIServerControllerSet {
	cs.staticResourceController.controller = nil
	cs.staticResourceController.emptyAllowed = true
	return cs
}

func (cs *APIServerControllerSet) WithWorkloadController(
	name, operatorNamespace, targetNamespace, targetOperandVersion, operandNamePrefix, conditionsPrefix string,
	kubeClient kubernetes.Interface,
	delegate workload.Delegate,
	openshiftClusterConfigClient openshiftconfigclientv1.ClusterOperatorInterface,
	versionRecorder status.VersionGetter,
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces,
	informers ...cache.SharedIndexInformer) *APIServerControllerSet {

	workloadController := workload.NewController(
		name,
		operatorNamespace,
		targetNamespace,
		targetOperandVersion,
		operandNamePrefix,
		conditionsPrefix,
		cs.operatorClient,
		kubeClient,
		delegate,
		openshiftClusterConfigClient,
		cs.eventRecorder,
		versionRecorder)

	workloadController.AddInformer(kubeInformersForNamespaces.InformersFor(targetNamespace).Core().V1().ConfigMaps().Informer())
	workloadController.AddInformer(kubeInformersForNamespaces.InformersFor(targetNamespace).Core().V1().Secrets().Informer())
	workloadController.AddInformer(kubeInformersForNamespaces.InformersFor(targetNamespace).Apps().V1().Deployments().Informer())

	workloadController.AddNamespaceInformer(kubeInformersForNamespaces.InformersFor(targetNamespace).Core().V1().Namespaces().Informer())

	for _, informer := range informers {
		workloadController.AddInformer(informer)
	}

	cs.workloadController.controller = workloadController

	return cs
}

func (cs *APIServerControllerSet) WithoutWorkloadController() *APIServerControllerSet {
	cs.workloadController.controller = nil
	cs.workloadController.emptyAllowed = true
	return cs
}

func (cs *APIServerControllerSet) PrepareRun() (preparedAPIServerControllerSet, error) {
	prepared := []controller{}
	errs := []error{}

	for name, cw := range map[string]controllerWrapper{
		"apiServiceController":            cs.apiServiceController,
		"clusterOperatorStatusController": cs.clusterOperatorStatusController,
		"configUpgradableController":      cs.configUpgradableController,
		"logLevelController":              cs.logLevelController,
		"finalizerController":             cs.finalizerController,
		"staticResourceController":        cs.staticResourceController,
		"workloadController":              cs.workloadController,
	} {
		c, err := cw.prepare()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", name, err))
			continue
		}
		if c != nil {
			prepared = append(prepared, c)
		}
	}

	return preparedAPIServerControllerSet{controllers: prepared}, errors.NewAggregate(errs)
}

func (cs *preparedAPIServerControllerSet) Run(ctx context.Context) {
	for i := range cs.controllers {
		go cs.controllers[i].Run(ctx, 1)
	}
}

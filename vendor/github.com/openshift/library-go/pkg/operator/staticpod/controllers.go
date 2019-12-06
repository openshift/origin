package staticpod

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/loglevel"
	"github.com/openshift/library-go/pkg/operator/revisioncontroller"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/backingresource"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/installer"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/installerstate"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/monitoring"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/node"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/prune"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/staticpodstate"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

type RunnableController interface {
	Run(ctx context.Context, workers int)
}

type staticPodOperatorControllerBuilder struct {
	// clients and related
	staticPodOperatorClient v1helpers.StaticPodOperatorClient
	kubeClient              kubernetes.Interface
	kubeInformers           v1helpers.KubeInformersForNamespaces
	dynamicClient           dynamic.Interface
	eventRecorder           events.Recorder

	// resource information
	operandNamespace   string
	staticPodName      string
	revisionConfigMaps []revisioncontroller.RevisionResource
	revisionSecrets    []revisioncontroller.RevisionResource

	// cert information
	certDir        string
	certConfigMaps []revisioncontroller.RevisionResource
	certSecrets    []revisioncontroller.RevisionResource

	// versioner information
	versionRecorder   status.VersionGetter
	operatorNamespace string
	operandName       string

	// installer information
	installCommand []string

	// pruning information
	pruneCommand []string
	// TODO de-dupe this.  I think it's actually a directory name
	staticPodPrefix string

	// TODO: remove this after all operators get rid of service monitor controller
	enableServiceMonitorController bool
}

func NewBuilder(
	staticPodOperatorClient v1helpers.StaticPodOperatorClient,
	kubeClient kubernetes.Interface,
	kubeInformers v1helpers.KubeInformersForNamespaces,
) Builder {
	return &staticPodOperatorControllerBuilder{
		staticPodOperatorClient: staticPodOperatorClient,
		kubeClient:              kubeClient,
		kubeInformers:           kubeInformers,
	}
}

// Builder allows the caller to construct a set of static pod controllers in pieces
type Builder interface {
	WithEvents(eventRecorder events.Recorder) Builder
	WithServiceMonitor(dynamicClient dynamic.Interface) Builder
	WithVersioning(operatorNamespace, operandName string, versionRecorder status.VersionGetter) Builder
	WithResources(operandNamespace, staticPodName string, revisionConfigMaps, revisionSecrets []revisioncontroller.RevisionResource) Builder
	WithCerts(certDir string, certConfigMaps, certSecrets []revisioncontroller.RevisionResource) Builder
	WithInstaller(command []string) Builder
	WithPruning(command []string, staticPodPrefix string) Builder
	ToControllers() (RunnableController, error)
}

func (b *staticPodOperatorControllerBuilder) WithEvents(eventRecorder events.Recorder) Builder {
	b.eventRecorder = eventRecorder
	return b
}

// DEPRECATED: We have moved all our operators now to have this manifest with customized content.
func (b *staticPodOperatorControllerBuilder) WithServiceMonitor(dynamicClient dynamic.Interface) Builder {
	klog.Warning("DEPRECATED: MonitoringResourceController is no longer needed")
	b.enableServiceMonitorController = true
	b.dynamicClient = dynamicClient
	return b
}

func (b *staticPodOperatorControllerBuilder) WithVersioning(operatorNamespace, operandName string, versionRecorder status.VersionGetter) Builder {
	b.operatorNamespace = operatorNamespace
	b.operandName = operandName
	b.versionRecorder = versionRecorder
	return b
}

func (b *staticPodOperatorControllerBuilder) WithResources(operandNamespace, staticPodName string, revisionConfigMaps, revisionSecrets []revisioncontroller.RevisionResource) Builder {
	b.operandNamespace = operandNamespace
	b.staticPodName = staticPodName
	b.revisionConfigMaps = revisionConfigMaps
	b.revisionSecrets = revisionSecrets
	return b
}

func (b *staticPodOperatorControllerBuilder) WithCerts(certDir string, certConfigMaps, certSecrets []revisioncontroller.RevisionResource) Builder {
	b.certDir = certDir
	b.certConfigMaps = certConfigMaps
	b.certSecrets = certSecrets
	return b
}

func (b *staticPodOperatorControllerBuilder) WithInstaller(command []string) Builder {
	b.installCommand = command
	return b
}

func (b *staticPodOperatorControllerBuilder) WithPruning(command []string, staticPodPrefix string) Builder {
	b.pruneCommand = command
	b.staticPodPrefix = staticPodPrefix
	return b
}

func (b *staticPodOperatorControllerBuilder) ToControllers() (RunnableController, error) {
	controllers := &staticPodOperatorControllers{}

	eventRecorder := b.eventRecorder
	if eventRecorder == nil {
		eventRecorder = events.NewLoggingEventRecorder("static-pod-operator-controller")
	}
	versionRecorder := b.versionRecorder
	if versionRecorder == nil {
		versionRecorder = status.NewVersionGetter()
	}

	configMapClient := v1helpers.CachedConfigMapGetter(b.kubeClient.CoreV1(), b.kubeInformers)
	secretClient := v1helpers.CachedSecretGetter(b.kubeClient.CoreV1(), b.kubeInformers)
	podClient := b.kubeClient.CoreV1()
	eventsClient := b.kubeClient.CoreV1()
	operandInformers := b.kubeInformers.InformersFor(b.operandNamespace)
	clusterInformers := b.kubeInformers.InformersFor("")

	var errs []error

	if len(b.operandNamespace) > 0 {
		controllers.add(revisioncontroller.NewRevisionController(
			b.operandNamespace,
			b.revisionConfigMaps,
			b.revisionSecrets,
			operandInformers,
			revisioncontroller.StaticPodLatestRevisionClient{StaticPodOperatorClient: b.staticPodOperatorClient},
			configMapClient,
			secretClient,
			eventRecorder,
		))
	} else {
		errs = append(errs, fmt.Errorf("missing revisionController; cannot proceed"))
	}

	if len(b.installCommand) > 0 {
		controllers.add(installer.NewInstallerController(
			b.operandNamespace,
			b.staticPodName,
			b.revisionConfigMaps,
			b.revisionSecrets,
			b.installCommand,
			operandInformers,
			b.staticPodOperatorClient,
			configMapClient,
			secretClient,
			podClient,
			eventRecorder,
		).WithCerts(
			b.certDir,
			b.certConfigMaps,
			b.certSecrets,
		))

		controllers.add(installerstate.NewInstallerStateController(
			operandInformers,
			podClient,
			eventsClient,
			b.staticPodOperatorClient,
			b.operandNamespace,
			eventRecorder,
		))
	} else {
		errs = append(errs, fmt.Errorf("missing installerController; cannot proceed"))
	}

	if len(b.operandName) > 0 {
		// TODO add handling for operator configmap changes to get version-mapping changes
		controllers.add(staticpodstate.NewStaticPodStateController(
			b.operandNamespace,
			b.staticPodName,
			b.operatorNamespace,
			b.operandName,
			operandInformers,
			b.staticPodOperatorClient,
			configMapClient,
			podClient,
			versionRecorder,
			eventRecorder,
		))
	} else {
		eventRecorder.Warning("StaticPodStateControllerMissing", "not enough information provided, not all functionality is present")
	}

	if len(b.pruneCommand) > 0 {
		controllers.add(prune.NewPruneController(
			b.operandNamespace,
			b.staticPodPrefix,
			b.pruneCommand,
			configMapClient,
			secretClient,
			podClient,
			b.staticPodOperatorClient,
			eventRecorder,
		))
	} else {
		eventRecorder.Warning("PruningControllerMissing", "not enough information provided, not all functionality is present")
	}

	controllers.add(node.NewNodeController(
		b.staticPodOperatorClient,
		clusterInformers,
		eventRecorder,
	))

	controllers.add(backingresource.NewBackingResourceController(
		b.operandNamespace,
		b.staticPodOperatorClient,
		operandInformers,
		b.kubeClient,
		eventRecorder,
	))

	if b.dynamicClient != nil && b.enableServiceMonitorController {
		controllers.add(monitoring.NewMonitoringResourceController(
			b.operandNamespace,
			b.operandNamespace,
			b.staticPodOperatorClient,
			operandInformers,
			b.kubeClient,
			b.dynamicClient,
			eventRecorder,
		))
	}

	controllers.add(unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(b.staticPodOperatorClient, eventRecorder))
	controllers.add(loglevel.NewClusterOperatorLoggingController(b.staticPodOperatorClient, eventRecorder))

	return controllers, errors.NewAggregate(errs)
}

type staticPodOperatorControllers struct {
	controllers []RunnableController
}

func (o *staticPodOperatorControllers) add(controller RunnableController) {
	o.controllers = append(o.controllers, controller)
}

func (o *staticPodOperatorControllers) Run(ctx context.Context, workers int) {
	for i := range o.controllers {
		go o.controllers[i].Run(ctx, workers)
	}

	<-ctx.Done()
}

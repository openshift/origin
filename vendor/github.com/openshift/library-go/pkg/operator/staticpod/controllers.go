package staticpod

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"

	"k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/library-go/pkg/operator/status"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/backingresource"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/installer"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/monitoring"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/node"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/prune"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/staticpodstate"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

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
	revisionConfigMaps []revision.RevisionResource
	revisionSecrets    []revision.RevisionResource

	// cert information
	certDir        string
	certConfigMaps []revision.RevisionResource
	certSecrets    []revision.RevisionResource

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
	WithResources(operandNamespace, staticPodName string, revisionConfigMaps, revisionSecrets []revision.RevisionResource) Builder
	WithCerts(certDir string, certConfigMaps, certSecrets []revision.RevisionResource) Builder
	WithInstaller(command []string) Builder
	WithPruning(command []string, staticPodPrefix string) Builder
	ToControllers() (*staticPodOperatorControllers, error)
}

func (b *staticPodOperatorControllerBuilder) WithEvents(eventRecorder events.Recorder) Builder {
	b.eventRecorder = eventRecorder
	return b
}

func (b *staticPodOperatorControllerBuilder) WithServiceMonitor(dynamicClient dynamic.Interface) Builder {
	b.dynamicClient = dynamicClient
	return b
}

func (b *staticPodOperatorControllerBuilder) WithVersioning(operatorNamespace, operandName string, versionRecorder status.VersionGetter) Builder {
	b.operatorNamespace = operatorNamespace
	b.operandName = operandName
	b.versionRecorder = versionRecorder
	return b
}

func (b *staticPodOperatorControllerBuilder) WithResources(operandNamespace, staticPodName string, revisionConfigMaps, revisionSecrets []revision.RevisionResource) Builder {
	b.operandNamespace = operandNamespace
	b.staticPodName = staticPodName
	b.revisionConfigMaps = revisionConfigMaps
	b.revisionSecrets = revisionSecrets
	return b
}

func (b *staticPodOperatorControllerBuilder) WithCerts(certDir string, certConfigMaps, certSecrets []revision.RevisionResource) Builder {
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

func (b *staticPodOperatorControllerBuilder) ToControllers() (*staticPodOperatorControllers, error) {
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
	operandInformers := b.kubeInformers.InformersFor(b.operandNamespace)
	clusterInformers := b.kubeInformers.InformersFor("")

	if len(b.operandNamespace) > 0 {
		controllers.revisionController = revision.NewRevisionController(
			b.operandNamespace,
			b.revisionConfigMaps,
			b.revisionSecrets,
			operandInformers,
			b.staticPodOperatorClient,
			configMapClient,
			secretClient,
			eventRecorder,
		)
	}

	if len(b.installCommand) > 0 {
		controllers.installerController = installer.NewInstallerController(
			b.operandNamespace,
			b.staticPodName,
			b.revisionConfigMaps,
			b.revisionSecrets,
			b.installCommand,
			operandInformers,
			b.staticPodOperatorClient,
			configMapClient,
			podClient,
			eventRecorder,
		).WithCerts(
			b.certDir,
			b.certConfigMaps,
			b.certSecrets,
		)
	}

	if len(b.operandName) > 0 {
		// TODO add handling for operator configmap changes to get version-mapping changes
		controllers.staticPodStateController = staticpodstate.NewStaticPodStateController(
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
		)
	}

	if len(b.pruneCommand) > 0 {
		controllers.pruneController = prune.NewPruneController(
			b.operandNamespace,
			b.staticPodPrefix,
			b.pruneCommand,
			configMapClient,
			secretClient,
			podClient,
			b.staticPodOperatorClient,
			eventRecorder,
		)
	}

	controllers.nodeController = node.NewNodeController(
		b.staticPodOperatorClient,
		clusterInformers,
		eventRecorder,
	)

	controllers.backingResourceController = backingresource.NewBackingResourceController(
		b.operandNamespace,
		b.staticPodOperatorClient,
		operandInformers,
		b.kubeClient,
		eventRecorder,
	)

	if b.dynamicClient != nil {
		controllers.monitoringResourceController = monitoring.NewMonitoringResourceController(
			b.operandNamespace,
			b.operandNamespace,
			b.staticPodOperatorClient,
			operandInformers,
			b.kubeClient,
			b.dynamicClient,
			eventRecorder,
		)
	}

	controllers.unsupportedConfigOverridesController = unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(b.staticPodOperatorClient, eventRecorder)

	errs := []error{}
	if controllers.revisionController == nil {
		errs = append(errs, fmt.Errorf("missing revisionController; cannot proceed"))
	}
	if controllers.installerController == nil {
		errs = append(errs, fmt.Errorf("missing installerController; cannot proceed"))
	}
	if controllers.staticPodStateController == nil {
		eventRecorder.Warning("StaticPodStateControllerMissing", "not enough information provided, not all functionality is present")
	}
	if controllers.pruneController == nil {
		eventRecorder.Warning("PruningControllerMissing", "not enough information provided, not all functionality is present")
	}
	if controllers.monitoringResourceController == nil {
		eventRecorder.Warning("MonitoringResourceController", "not enough information provided, not all functionality is present")
	}

	return controllers, errors.NewAggregate(errs)
}

type staticPodOperatorControllers struct {
	revisionController                   *revision.RevisionController
	installerController                  *installer.InstallerController
	staticPodStateController             *staticpodstate.StaticPodStateController
	pruneController                      *prune.PruneController
	nodeController                       *node.NodeController
	backingResourceController            *backingresource.BackingResourceController
	monitoringResourceController         *monitoring.MonitoringResourceController
	unsupportedConfigOverridesController *unsupportedconfigoverridescontroller.UnsupportedConfigOverridesController
}

func (o *staticPodOperatorControllers) WithInstallerPodMutationFn(installerPodMutationFn installer.InstallerPodMutationFunc) *staticPodOperatorControllers {
	o.installerController.WithInstallerPodMutationFn(installerPodMutationFn)
	return o
}

func (o *staticPodOperatorControllers) Run(stopCh <-chan struct{}) {
	go o.revisionController.Run(1, stopCh)
	go o.installerController.Run(1, stopCh)
	go o.staticPodStateController.Run(1, stopCh)
	go o.pruneController.Run(1, stopCh)
	go o.nodeController.Run(1, stopCh)
	go o.backingResourceController.Run(1, stopCh)
	go o.monitoringResourceController.Run(1, stopCh)
	go o.unsupportedConfigOverridesController.Run(1, stopCh)

	<-stopCh
}

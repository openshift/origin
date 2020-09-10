package controllercmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"

	"github.com/openshift/library-go/pkg/config/client"
	"github.com/openshift/library-go/pkg/config/configdefaults"
	leaderelectionconverter "github.com/openshift/library-go/pkg/config/leaderelection"
	"github.com/openshift/library-go/pkg/config/serving"
	"github.com/openshift/library-go/pkg/controller/fileobserver"
	"github.com/openshift/library-go/pkg/operator/events"
)

// StartFunc is the function to call on leader election start
type StartFunc func(context.Context, *ControllerContext) error

type ControllerContext struct {
	ComponentConfig *unstructured.Unstructured

	// KubeConfig provides the REST config with no content type (it will default to JSON).
	// Use this config for CR resources.
	KubeConfig *rest.Config

	// ProtoKubeConfig provides the REST config with "application/vnd.kubernetes.protobuf,application/json" content type.
	// Note that this config might not be safe for CR resources, instead it should be used for other resources.
	ProtoKubeConfig *rest.Config

	// EventRecorder is used to record events in controllers.
	EventRecorder events.Recorder

	// Server is the GenericAPIServer serving healthz checks and debug info
	Server *genericapiserver.GenericAPIServer
}

// defaultObserverInterval specifies the default interval that file observer will do rehash the files it watches and react to any changes
// in those files.
var defaultObserverInterval = 5 * time.Second

// ControllerBuilder allows the construction of an controller in optional pieces.
type ControllerBuilder struct {
	kubeAPIServerConfigFile *string
	clientOverrides         *client.ClientConnectionOverrides
	leaderElection          *configv1.LeaderElection
	fileObserver            fileobserver.Observer
	fileObserverReactorFn   func(file string, action fileobserver.ActionType) error
	eventRecorderOptions    record.CorrelatorOptions

	startFunc          StartFunc
	componentName      string
	componentNamespace string
	instanceIdentity   string
	observerInterval   time.Duration

	servingInfo          *configv1.HTTPServingInfo
	authenticationConfig *operatorv1alpha1.DelegatedAuthentication
	authorizationConfig  *operatorv1alpha1.DelegatedAuthorization
	healthChecks         []healthz.HealthChecker

	versionInfo *version.Info

	// nonZeroExitFn takes a function that exit the process with non-zero code.
	// This stub exists for unit test where we can check if the graceful termination work properly.
	// Default function will klog.Warning(args) and os.Exit(1).
	nonZeroExitFn func(args ...interface{})
}

// NewController returns a builder struct for constructing the command you want to run
func NewController(componentName string, startFunc StartFunc) *ControllerBuilder {
	return &ControllerBuilder{
		startFunc:        startFunc,
		componentName:    componentName,
		observerInterval: defaultObserverInterval,
		nonZeroExitFn: func(args ...interface{}) {
			klog.Warning(args...)
			os.Exit(1)
		},
	}
}

// WithRestartOnChange will enable a file observer controller loop that observes changes into specified files. If a change to a file is detected,
// the specified channel will be closed (allowing to graceful shutdown for other channels).
func (b *ControllerBuilder) WithRestartOnChange(stopCh chan<- struct{}, startingFileContent map[string][]byte, files ...string) *ControllerBuilder {
	if len(files) == 0 {
		return b
	}
	if b.fileObserver == nil {
		observer, err := fileobserver.NewObserver(b.observerInterval)
		if err != nil {
			panic(err)
		}
		b.fileObserver = observer
	}
	var once sync.Once

	b.fileObserverReactorFn = func(filename string, action fileobserver.ActionType) error {
		once.Do(func() {
			klog.Warning(fmt.Sprintf("Restart triggered because of %s", action.String(filename)))
			close(stopCh)
		})
		return nil
	}

	b.fileObserver.AddReactor(b.fileObserverReactorFn, startingFileContent, files...)
	return b
}

func (b *ControllerBuilder) WithComponentNamespace(ns string) *ControllerBuilder {
	b.componentNamespace = ns
	return b
}

// WithLeaderElection adds leader election options
func (b *ControllerBuilder) WithLeaderElection(leaderElection configv1.LeaderElection, defaultNamespace, defaultName string) *ControllerBuilder {
	if leaderElection.Disable {
		return b
	}

	defaulted := leaderelectionconverter.LeaderElectionDefaulting(leaderElection, defaultNamespace, defaultName)
	b.leaderElection = &defaulted
	return b
}

// WithVersion accepts a getting that provide binary version information that is used to report build_info information to prometheus
func (b *ControllerBuilder) WithVersion(info version.Info) *ControllerBuilder {
	b.versionInfo = &info
	return b
}

// WithServer adds a server that provides metrics and healthz
func (b *ControllerBuilder) WithServer(servingInfo configv1.HTTPServingInfo, authenticationConfig operatorv1alpha1.DelegatedAuthentication, authorizationConfig operatorv1alpha1.DelegatedAuthorization) *ControllerBuilder {
	b.servingInfo = servingInfo.DeepCopy()
	configdefaults.SetRecommendedHTTPServingInfoDefaults(b.servingInfo)
	b.authenticationConfig = &authenticationConfig
	b.authorizationConfig = &authorizationConfig
	return b
}

// WithHealthChecks adds a list of healthchecks to the server
func (b *ControllerBuilder) WithHealthChecks(healthChecks ...healthz.HealthChecker) *ControllerBuilder {
	b.healthChecks = append(b.healthChecks, healthChecks...)
	return b
}

// WithKubeConfigFile sets an optional kubeconfig file. inclusterconfig will be used if filename is empty
func (b *ControllerBuilder) WithKubeConfigFile(kubeConfigFilename string, defaults *client.ClientConnectionOverrides) *ControllerBuilder {
	b.kubeAPIServerConfigFile = &kubeConfigFilename
	b.clientOverrides = defaults
	return b
}

// WithInstanceIdentity sets the instance identity to use if you need something special. The default is just a UID which is
// usually fine for a pod.
func (b *ControllerBuilder) WithInstanceIdentity(identity string) *ControllerBuilder {
	b.instanceIdentity = identity
	return b
}

// WithEventRecorderOptions allows to override the default Kubernetes event recorder correlator options.
// This is needed if the binary is sending a lot of events.
// Using events.DefaultOperatorEventRecorderOptions here makes a good default for normal operator binary.
func (b *ControllerBuilder) WithEventRecorderOptions(options record.CorrelatorOptions) *ControllerBuilder {
	b.eventRecorderOptions = options
	return b
}

// Run starts your controller for you.  It uses leader election if you asked, otherwise it directly calls you
func (b *ControllerBuilder) Run(ctx context.Context, config *unstructured.Unstructured) error {
	clientConfig, err := b.getClientConfig()
	if err != nil {
		return err
	}

	if b.fileObserver != nil {
		go b.fileObserver.Run(ctx.Done())
	}

	kubeClient := kubernetes.NewForConfigOrDie(clientConfig)
	namespace, err := b.getComponentNamespace()
	if err != nil {
		klog.Warningf("unable to identify the current namespace for events: %v", err)
	}
	controllerRef, err := events.GetControllerReferenceForCurrentPod(kubeClient, namespace, nil)
	if err != nil {
		klog.Warningf("unable to get owner reference (falling back to namespace): %v", err)
	}
	eventRecorder := events.NewKubeRecorderWithOptions(kubeClient.CoreV1().Events(namespace), b.eventRecorderOptions, b.componentName, controllerRef)

	utilruntime.PanicHandlers = append(utilruntime.PanicHandlers, func(r interface{}) {
		eventRecorder.Warningf(fmt.Sprintf("%sPanic", strings.Title(b.componentName)), "Panic observed: %v", r)
	})

	// if there is file observer defined for this command, add event into default reaction function.
	if b.fileObserverReactorFn != nil {
		originalFileObserverReactorFn := b.fileObserverReactorFn
		b.fileObserverReactorFn = func(file string, action fileobserver.ActionType) error {
			eventRecorder.Warningf("OperatorRestart", "Restarted because of %s", action.String(file))
			return originalFileObserverReactorFn(file, action)
		}
	}

	// report the binary version metrics to prometheus
	if b.versionInfo != nil {
		buildInfo := metrics.NewGaugeVec(
			&metrics.GaugeOpts{
				Name: strings.Replace(namespace, "-", "_", -1) + "_build_info",
				Help: "A metric with a constant '1' value labeled by major, minor, git version, git commit, git tree state, build date, Go version, " +
					"and compiler from which " + b.componentName + " was built, and platform on which it is running.",
				StabilityLevel: metrics.ALPHA,
			},
			[]string{"major", "minor", "gitVersion", "gitCommit", "gitTreeState", "buildDate", "goVersion", "compiler", "platform"},
		)
		legacyregistry.MustRegister(buildInfo)
		buildInfo.WithLabelValues(b.versionInfo.Major, b.versionInfo.Minor, b.versionInfo.GitVersion, b.versionInfo.GitCommit, b.versionInfo.GitTreeState, b.versionInfo.BuildDate, b.versionInfo.GoVersion,
			b.versionInfo.Compiler, b.versionInfo.Platform).Set(1)
		klog.Infof("%s version %s-%s", b.componentName, b.versionInfo.GitVersion, b.versionInfo.GitCommit)
	}

	kubeConfig := ""
	if b.kubeAPIServerConfigFile != nil {
		kubeConfig = *b.kubeAPIServerConfigFile
	}

	var server *genericapiserver.GenericAPIServer
	if b.servingInfo != nil {
		serverConfig, err := serving.ToServerConfig(ctx, *b.servingInfo, *b.authenticationConfig, *b.authorizationConfig, kubeConfig)
		if err != nil {
			return err
		}
		serverConfig.HealthzChecks = append(serverConfig.HealthzChecks, b.healthChecks...)

		server, err = serverConfig.Complete(nil).New(b.componentName, genericapiserver.NewEmptyDelegate())
		if err != nil {
			return err
		}

		go func() {
			if err := server.PrepareRun().Run(ctx.Done()); err != nil {
				klog.Fatal(err)
			}
			klog.Info("server exited")
		}()
	}

	protoConfig := rest.CopyConfig(clientConfig)
	protoConfig.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	protoConfig.ContentType = "application/vnd.kubernetes.protobuf"

	controllerContext := &ControllerContext{
		ComponentConfig: config,
		KubeConfig:      clientConfig,
		ProtoKubeConfig: protoConfig,
		EventRecorder:   eventRecorder,
		Server:          server,
	}

	if b.leaderElection == nil {
		if err := b.startFunc(ctx, controllerContext); err != nil {
			return err
		}
		return nil
	}

	// ensure blocking TCP connections don't block the leader election
	leaderConfig := rest.CopyConfig(protoConfig)
	leaderConfig.Timeout = b.leaderElection.RenewDeadline.Duration

	leaderElection, err := leaderelectionconverter.ToConfigMapLeaderElection(leaderConfig, *b.leaderElection, b.componentName, b.instanceIdentity)
	if err != nil {
		return err
	}

	// 10s is the graceful termination time we give the controllers to finish their workers.
	// when this time pass, we exit with non-zero code, killing all controller workers.
	// NOTE: The pod must set the termination graceful time.
	leaderElection.Callbacks.OnStartedLeading = b.getOnStartedLeadingFunc(controllerContext, 10*time.Second)

	leaderelection.RunOrDie(ctx, leaderElection)
	return nil
}

func (b ControllerBuilder) getOnStartedLeadingFunc(controllerContext *ControllerContext, gracefulTerminationDuration time.Duration) func(ctx context.Context) {
	return func(ctx context.Context) {
		stoppedCh := make(chan struct{})
		go func() {
			defer close(stoppedCh)
			if err := b.startFunc(ctx, controllerContext); err != nil {
				b.nonZeroExitFn(fmt.Sprintf("graceful termination failed, controllers failed with error: %v", err))
			}
		}()

		select {
		case <-ctx.Done(): // context closed means the process likely received signal to terminate
			controllerContext.EventRecorder.Shutdown()
		case <-stoppedCh:
			// if context was not cancelled (it is not "done"), but the startFunc terminated, it means it terminated prematurely
			// when this happen, it means the controllers terminated without error.
			if ctx.Err() == nil {
				b.nonZeroExitFn("graceful termination failed, controllers terminated prematurely")
			}
		}

		select {
		case <-time.After(gracefulTerminationDuration): // when context was closed above, give controllers extra time to terminate gracefully
			b.nonZeroExitFn(fmt.Sprintf("graceful termination failed, some controllers failed to shutdown in %s", gracefulTerminationDuration))
		case <-stoppedCh: // stoppedCh here means the controllers finished termination and we exit 0
		}
	}
}

func (b *ControllerBuilder) getComponentNamespace() (string, error) {
	if len(b.componentNamespace) > 0 {
		return b.componentNamespace, nil
	}
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "openshift-config-managed", err
	}
	return string(nsBytes), nil
}

func (b *ControllerBuilder) getClientConfig() (*rest.Config, error) {
	kubeconfig := ""
	if b.kubeAPIServerConfigFile != nil {
		kubeconfig = *b.kubeAPIServerConfigFile
	}

	return client.GetKubeConfigOrInClusterConfig(kubeconfig, b.clientOverrides)
}

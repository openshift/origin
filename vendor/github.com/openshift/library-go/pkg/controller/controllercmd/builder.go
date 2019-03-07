package controllercmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
type StartFunc func(*ControllerContext) error

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

	stopChan <-chan struct{}
}

// Done returns a channel which will close on termination.
func (c ControllerContext) Done() <-chan struct{} {
	return c.stopChan
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

	startFunc        StartFunc
	componentName    string
	instanceIdentity string
	observerInterval time.Duration

	servingInfo          *configv1.HTTPServingInfo
	authenticationConfig *operatorv1alpha1.DelegatedAuthentication
	authorizationConfig  *operatorv1alpha1.DelegatedAuthorization
	healthChecks         []healthz.HealthzChecker
}

// NewController returns a builder struct for constructing the command you want to run
func NewController(componentName string, startFunc StartFunc) *ControllerBuilder {
	return &ControllerBuilder{
		startFunc:        startFunc,
		componentName:    componentName,
		observerInterval: defaultObserverInterval,
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
			glog.Warning(fmt.Sprintf("Restart triggered because of %s", action.String(filename)))
			close(stopCh)
		})
		return nil
	}

	b.fileObserver.AddReactor(b.fileObserverReactorFn, startingFileContent, files...)
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

// WithServer adds a server that provides metrics and healthz
func (b *ControllerBuilder) WithServer(servingInfo configv1.HTTPServingInfo, authenticationConfig operatorv1alpha1.DelegatedAuthentication, authorizationConfig operatorv1alpha1.DelegatedAuthorization) *ControllerBuilder {
	b.servingInfo = servingInfo.DeepCopy()
	configdefaults.SetRecommendedHTTPServingInfoDefaults(b.servingInfo)
	b.authenticationConfig = &authenticationConfig
	b.authorizationConfig = &authorizationConfig
	return b
}

// WithHealthChecks adds a list of healthchecks to the server
func (b *ControllerBuilder) WithHealthChecks(healthChecks ...healthz.HealthzChecker) *ControllerBuilder {
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

// Run starts your controller for you.  It uses leader election if you asked, otherwise it directly calls you
func (b *ControllerBuilder) Run(config *unstructured.Unstructured, ctx context.Context) error {
	clientConfig, err := b.getClientConfig()
	if err != nil {
		return err
	}

	if b.fileObserver != nil {
		go b.fileObserver.Run(ctx.Done())
	}

	kubeClient := kubernetes.NewForConfigOrDie(clientConfig)
	namespace, err := b.getNamespace()
	if err != nil {
		panic("unable to read the namespace")
	}
	controllerRef, err := events.GetControllerReferenceForCurrentPod(kubeClient, namespace, nil)
	if err != nil {
		panic(fmt.Sprintf("unable to obtain replicaset reference for events: %v", err))
	}
	eventRecorder := events.NewKubeRecorder(kubeClient.CoreV1().Events(namespace), b.componentName, controllerRef)

	// if there is file observer defined for this command, add event into default reaction function.
	if b.fileObserverReactorFn != nil {
		originalFileObserverReactorFn := b.fileObserverReactorFn
		b.fileObserverReactorFn = func(file string, action fileobserver.ActionType) error {
			eventRecorder.Warningf("OperatorRestart", "Restarted because of %s", action.String(file))
			return originalFileObserverReactorFn(file, action)
		}
	}

	switch {
	case b.servingInfo == nil && len(b.healthChecks) > 0:
		return fmt.Errorf("healthchecks without server config won't work")

	default:
		kubeConfig := ""
		if b.kubeAPIServerConfigFile != nil {
			kubeConfig = *b.kubeAPIServerConfigFile
		}
		serverConfig, err := serving.ToServerConfig(*b.servingInfo, *b.authenticationConfig, *b.authorizationConfig, kubeConfig)
		if err != nil {
			return err
		}
		serverConfig.HealthzChecks = append(serverConfig.HealthzChecks, b.healthChecks...)

		server, err := serverConfig.Complete(nil).New(b.componentName, genericapiserver.NewEmptyDelegate())
		if err != nil {
			return err
		}

		go func() {
			if err := server.PrepareRun().Run(ctx.Done()); err != nil {
				glog.Error(err)
			}
			glog.Fatal("server exited")
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
		stopChan:        ctx.Done(),
	}

	if b.leaderElection == nil {
		if err := b.startFunc(controllerContext); err != nil {
			return err
		}
		return fmt.Errorf("exited")
	}

	leaderElection, err := leaderelectionconverter.ToConfigMapLeaderElection(clientConfig, *b.leaderElection, b.componentName, b.instanceIdentity)
	if err != nil {
		return err
	}

	leaderElection.Callbacks.OnStartedLeading = func(ctx context.Context) {
		controllerContext.stopChan = ctx.Done()
		if err := b.startFunc(controllerContext); err != nil {
			glog.Fatal(err)
		}
	}
	leaderelection.RunOrDie(ctx, leaderElection)
	return fmt.Errorf("exited")
}

func (b *ControllerBuilder) getNamespace() (string, error) {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	return string(nsBytes), err
}

func (b *ControllerBuilder) getClientConfig() (*rest.Config, error) {
	kubeconfig := ""
	if b.kubeAPIServerConfigFile != nil {
		kubeconfig = *b.kubeAPIServerConfigFile
	}

	return client.GetKubeConfigOrInClusterConfig(kubeconfig, b.clientOverrides)
}

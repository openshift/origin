package openshift_controller_manager

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoclientset "k8s.io/client-go/kubernetes"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	cmappoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/controller"

	origincontrollers "github.com/openshift/origin/pkg/cmd/openshift-controller-manager/controller"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/cm"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/version"
)

// RunOpenShiftControllerManagerCombined is only called when we're starting with the API server.  It means we can't start our own server.
func RunOpenShiftControllerManagerCombined(masterConfig *configapi.MasterConfig) error {
	return runOpenShiftControllerManager(masterConfig, false)
}

func RunOpenShiftControllerManager(masterConfig *configapi.MasterConfig) error {
	return runOpenShiftControllerManager(masterConfig, true)
}

func runOpenShiftControllerManager(masterConfig *configapi.MasterConfig, runServer bool) error {
	openshiftControllerInformers, err := origin.NewInformers(*masterConfig)
	if err != nil {
		return err
	}

	_, config, err := configapi.GetExternalKubeClient(masterConfig.MasterClients.OpenShiftLoopbackKubeConfig, masterConfig.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return err
	}
	clientGoKubeExternal, err := clientgoclientset.NewForConfig(config)
	if err != nil {
		return err
	}

	// you can't double run healthz, so only do this next bit if we aren't starting the API
	if runServer {
		glog.Infof("Starting controllers on %s (%s)", masterConfig.ServingInfo.BindAddress, version.Get().String())

		if err := origincontrollers.RunControllerServer(masterConfig.ServingInfo, clientGoKubeExternal); err != nil {
			return err
		}
	}

	kubeExternal, privilegedLoopbackConfig, err := configapi.GetExternalKubeClient(masterConfig.MasterClients.OpenShiftLoopbackKubeConfig, masterConfig.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return err
	}

	// TODO refactor this controller so that it no longer relies on direct etcd access
	// these restoptions are used to directly access small keysets on etcd that do NOT overlap with access
	// by the main API server, so we aren't paying a large cost for the separation.
	restOptsGetter, err := originrest.StorageOptions(*masterConfig)
	if err != nil {
		return err
	}
	allocationController := origin.SecurityAllocationController{
		SecurityAllocator:          masterConfig.ProjectConfig.SecurityAllocator,
		OpenshiftRESTOptionsGetter: restOptsGetter,
		ExternalKubeInformers:      openshiftControllerInformers.GetExternalKubeInformers(),
		KubeExternalClient:         kubeExternal,
	}

	originControllerManager := func(stopCh <-chan struct{}) {
		if err := waitForHealthyAPIServer(kubeExternal.Discovery().RESTClient()); err != nil {
			glog.Fatal(err)
		}

		openshiftControllerOptions, err := getOpenshiftControllerOptions(masterConfig.KubernetesMasterConfig.ControllerArguments)
		if err != nil {
			glog.Fatal(err)
		}

		informersStarted := make(chan struct{})
		controllerContext := newControllerContext(openshiftControllerOptions, masterConfig.ControllerConfig.Controllers, privilegedLoopbackConfig, kubeExternal, openshiftControllerInformers, stopCh, informersStarted)
		if err := startControllers(*masterConfig, allocationController, controllerContext); err != nil {
			glog.Fatal(err)
		}

		openshiftControllerInformers.Start(stopCh)
		close(informersStarted)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeExternal.CoreV1().RESTClient()).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-controller-manager"})
	id, err := os.Hostname()
	if err != nil {
		return err
	}
	openshiftLeaderElectionArgs, err := getLeaderElectionOptions(masterConfig.KubernetesMasterConfig.ControllerArguments)
	if err != nil {
		return err
	}
	rl, err := resourcelock.New(openshiftLeaderElectionArgs.ResourceLock,
		"kube-system",
		"openshift-master-controllers", // this matches what ansible used to set
		kubeExternal.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return err
	}
	go leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: openshiftLeaderElectionArgs.LeaseDuration.Duration,
		RenewDeadline: openshiftLeaderElectionArgs.RenewDeadline.Duration,
		RetryPeriod:   openshiftLeaderElectionArgs.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: originControllerManager,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})

	return nil
}

func newControllerContext(
	openshiftControllerOptions origincontrollers.OpenshiftControllerOptions,
	enabledControllers []string,
	privilegedLoopbackConfig *rest.Config,
	kubeExternal kclientsetexternal.Interface,
	informers origin.InformerAccess,
	stopCh <-chan struct{},
	informersStarted chan struct{},
) origincontrollers.ControllerContext {

	// divide up the QPS since it re-used separately for every client
	// TODO, eventually make this configurable individually in some way.
	if privilegedLoopbackConfig.QPS > 0 {
		privilegedLoopbackConfig.QPS = privilegedLoopbackConfig.QPS/10 + 1
	}
	if privilegedLoopbackConfig.Burst > 0 {
		privilegedLoopbackConfig.Burst = privilegedLoopbackConfig.Burst/10 + 1
	}

	openshiftControllerContext := origincontrollers.ControllerContext{
		OpenshiftControllerOptions: openshiftControllerOptions,
		EnabledControllers:         enabledControllers,

		ClientBuilder: origincontrollers.OpenshiftControllerClientBuilder{
			ControllerClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         rest.AnonymousClientConfig(privilegedLoopbackConfig),
				CoreClient:           kubeExternal.Core(),
				AuthenticationClient: kubeExternal.Authentication(),
				Namespace:            bootstrappolicy.DefaultOpenShiftInfraNamespace,
			},
		},
		InternalKubeInformers:   informers.GetInternalKubeInformers(),
		ExternalKubeInformers:   informers.GetExternalKubeInformers(),
		AppInformers:            informers.GetAppInformers(),
		AuthorizationInformers:  informers.GetAuthorizationInformers(),
		BuildInformers:          informers.GetBuildInformers(),
		ImageInformers:          informers.GetImageInformers(),
		NetworkInformers:        informers.GetNetworkInformers(),
		QuotaInformers:          informers.GetQuotaInformers(),
		SecurityInformers:       informers.GetSecurityInformers(),
		TemplateInformers:       informers.GetTemplateInformers(),
		GenericResourceInformer: informers.ToGenericInformer(),
		Stop:             stopCh,
		InformersStarted: informersStarted,
	}

	return openshiftControllerContext
}

// getOpenshiftControllerOptions parses the CLI args used by the kube-controllers (which control these options today), so that
// we can defer making the controller options structs until we have a better idea what they should look like.
// This does mean we pull in an upstream command that hopefully won't change much.
func getOpenshiftControllerOptions(args map[string][]string) (origincontrollers.OpenshiftControllerOptions, error) {
	cmserver := cmappoptions.NewCMServer()
	if err := cmdflags.Resolve(args, cm.OriginControllerManagerAddFlags(cmserver)); len(err) > 0 {
		return origincontrollers.OpenshiftControllerOptions{}, kutilerrors.NewAggregate(err)
	}

	return origincontrollers.OpenshiftControllerOptions{
		HPAControllerOptions: origincontrollers.HPAControllerOptions{
			SyncPeriod:               cmserver.KubeControllerManagerConfiguration.HorizontalPodAutoscalerSyncPeriod,
			UpscaleForbiddenWindow:   cmserver.KubeControllerManagerConfiguration.HorizontalPodAutoscalerUpscaleForbiddenWindow,
			DownscaleForbiddenWindow: cmserver.KubeControllerManagerConfiguration.HorizontalPodAutoscalerDownscaleForbiddenWindow,
		},
		ResourceQuotaOptions: origincontrollers.ResourceQuotaOptions{
			ConcurrentSyncs: cmserver.KubeControllerManagerConfiguration.ConcurrentResourceQuotaSyncs,
			SyncPeriod:      cmserver.KubeControllerManagerConfiguration.ResourceQuotaSyncPeriod,
			MinResyncPeriod: cmserver.KubeControllerManagerConfiguration.MinResyncPeriod,
		},
		ServiceAccountTokenOptions: origincontrollers.ServiceAccountTokenOptions{
			ConcurrentSyncs: cmserver.KubeControllerManagerConfiguration.ConcurrentSATokenSyncs,
		},
	}, nil
}

// getLeaderElectionOptions parses the CLI args used by the openshift controller leader election (which control these options today), so that
// we can defer making the options structs until we have a better idea what they should look like.
// This does mean we pull in an upstream command that hopefully won't change much.
func getLeaderElectionOptions(args map[string][]string) (componentconfig.LeaderElectionConfiguration, error) {
	cmserver := cmappoptions.NewCMServer()
	cmserver.LeaderElection.RetryPeriod = metav1.Duration{Duration: 3 * time.Second}

	if err := cmdflags.Resolve(args, cm.OriginControllerManagerAddFlags(cmserver)); len(err) > 0 {
		return componentconfig.LeaderElectionConfiguration{}, kutilerrors.NewAggregate(err)
	}

	leaderElection := cmserver.KubeControllerManagerConfiguration.LeaderElection
	leaderElection.LeaderElect = true
	leaderElection.ResourceLock = "configmaps"
	return leaderElection, nil
}

func waitForHealthyAPIServer(client rest.Interface) error {
	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		healthStatus := 0
		resp := client.Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			glog.Errorf("Server isn't healthy yet. Waiting a little while.")
			return false, nil
		}
		content, _ := resp.Raw()
		healthzContent = string(content)

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("server unhealthy: %v: %v", healthzContent, err)
	}

	return nil
}

// startControllers launches the controllers
// allocation controller is passed in because it wants direct etcd access.  Naughty.
func startControllers(options configapi.MasterConfig, allocationController origin.SecurityAllocationController, controllerContext origincontrollers.ControllerContext) error {
	openshiftControllerConfig, err := origincontrollers.BuildOpenshiftControllerConfig(options)
	if err != nil {
		return err
	}

	allocationController.RunSecurityAllocationController()

	openshiftControllerInitializers, err := openshiftControllerConfig.GetControllerInitializers()
	if err != nil {
		return err
	}

	for controllerName, initFn := range openshiftControllerInitializers {
		if !controllerContext.IsControllerEnabled(controllerName) {
			glog.Warningf("%q is disabled", controllerName)
			continue
		}

		glog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(controllerContext)
		if err != nil {
			glog.Fatalf("Error starting %q (%v)", controllerName, err)
			return err
		}
		if !started {
			glog.Warningf("Skipping %q", controllerName)
			continue
		}
		glog.Infof("Started %q", controllerName)
	}

	glog.Infof("Started Origin Controllers")

	return nil
}

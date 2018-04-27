package openshift_controller_manager

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/controller"

	origincontrollers "github.com/openshift/origin/pkg/cmd/openshift-controller-manager/controller"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/version"
)

func RunOpenShiftControllerManager(config *configapi.OpenshiftControllerConfig, clientConfig *rest.Config) error {
	kubeExternal, err := kclientsetexternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	openshiftControllerInformers, err := origin.NewInformers(clientConfig)
	if err != nil {
		return err
	}

	// only serve if we have serving information.
	if config.ServingInfo != nil {
		glog.Infof("Starting controllers on %s (%s)", config.ServingInfo.BindAddress, version.Get().String())

		if err := origincontrollers.RunControllerServer(*config.ServingInfo, kubeExternal); err != nil {
			return err
		}
	}

	originControllerManager := func(stopCh <-chan struct{}) {
		if err := waitForHealthyAPIServer(kubeExternal.Discovery().RESTClient()); err != nil {
			glog.Fatal(err)
		}

		informersStarted := make(chan struct{})
		controllerContext := newControllerContext(*config, clientConfig, kubeExternal, openshiftControllerInformers, stopCh, informersStarted)
		if err := startControllers(controllerContext); err != nil {
			glog.Fatal(err)
		}

		openshiftControllerInformers.Start(stopCh)
		close(informersStarted)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeExternal.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-controller-manager"})
	id, err := os.Hostname()
	if err != nil {
		return err
	}
	rl, err := resourcelock.New(
		"configmaps",
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
		LeaseDuration: config.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: config.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   config.LeaderElection.RetryPeriod.Duration,
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
	config configapi.OpenshiftControllerConfig,
	inClientConfig *rest.Config,
	kubeExternal kclientsetexternal.Interface,
	informers origin.InformerAccess,
	stopCh <-chan struct{},
	informersStarted chan struct{},
) origincontrollers.ControllerContext {

	// copy to avoid messing with original
	clientConfig := rest.CopyConfig(inClientConfig)

	// divide up the QPS since it re-used separately for every client
	// TODO, eventually make this configurable individually in some way.
	if clientConfig.QPS > 0 {
		clientConfig.QPS = clientConfig.QPS/10 + 1
	}
	if clientConfig.Burst > 0 {
		clientConfig.Burst = clientConfig.Burst/10 + 1
	}

	discoveryClient := cacheddiscovery.NewMemCacheClient(kubeExternal.Discovery())
	dynamicRestMapper := discovery.NewDeferredDiscoveryRESTMapper(discoveryClient, meta.InterfacesForUnstructured)
	dynamicRestMapper.Reset()

	go wait.Until(dynamicRestMapper.Reset, 30*time.Second, stopCh)

	openshiftControllerContext := origincontrollers.ControllerContext{
		OpenshiftControllerConfig: config,

		ClientBuilder: origincontrollers.OpenshiftControllerClientBuilder{
			ControllerClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         rest.AnonymousClientConfig(clientConfig),
				CoreClient:           kubeExternal.Core(),
				AuthenticationClient: kubeExternal.Authentication(),
				Namespace:            bootstrappolicy.DefaultOpenShiftInfraNamespace,
			},
		},
		ExternalKubeInformers:   informers.GetExternalKubeInformers(),
		AppInformers:            informers.GetAppInformers(),
		AuthorizationInformers:  informers.GetAuthorizationInformers(),
		BuildInformers:          informers.GetBuildInformers(),
		ImageInformers:          informers.GetImageInformers(),
		NetworkInformers:        informers.GetNetworkInformers(),
		QuotaInformers:          informers.GetQuotaInformers(),
		SecurityInformers:       informers.GetSecurityInformers(),
		RouteInformers:          informers.GetRouteInformers(),
		TemplateInformers:       informers.GetTemplateInformers(),
		GenericResourceInformer: informers.ToGenericInformer(),
		Stop:              stopCh,
		InformersStarted:  informersStarted,
		DynamicRestMapper: dynamicRestMapper,
	}

	return openshiftControllerContext
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
func startControllers(controllerContext origincontrollers.ControllerContext) error {
	for controllerName, initFn := range origincontrollers.ControllerInitializers {
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

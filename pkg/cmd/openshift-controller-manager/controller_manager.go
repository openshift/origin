package openshift_controller_manager

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
	origincontrollers "github.com/openshift/origin/pkg/cmd/openshift-controller-manager/controller"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/version"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

func RunOpenShiftControllerManager(config *configapi.OpenshiftControllerConfig, clientConfig *rest.Config) error {
	util.InitLogrus()
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	// only serve if we have serving information.
	if config.ServingInfo != nil {
		glog.Infof("Starting controllers on %s (%s)", config.ServingInfo.BindAddress, version.Get().String())

		if err := origincontrollers.RunControllerServer(*config.ServingInfo, kubeClient); err != nil {
			return err
		}
	}

	{
		imageTemplate := variable.NewDefaultImageTemplate()
		imageTemplate.Format = config.Deployer.ImageTemplateFormat.Format
		imageTemplate.Latest = config.Deployer.ImageTemplateFormat.Latest
		glog.Infof("DeploymentConfig controller using images from %q", imageTemplate.ExpandOrDie("<component>"))
	}
	{
		imageTemplate := variable.NewDefaultImageTemplate()
		imageTemplate.Format = config.Build.ImageTemplateFormat.Format
		imageTemplate.Latest = config.Build.ImageTemplateFormat.Latest
		glog.Infof("Build controller using images from %q", imageTemplate.ExpandOrDie("<component>"))
	}

	originControllerManager := func(stopCh <-chan struct{}) {
		if err := waitForHealthyAPIServer(kubeClient.Discovery().RESTClient()); err != nil {
			glog.Fatal(err)
		}

		controllerContext, err := origincontrollers.NewControllerContext(*config, clientConfig, stopCh)
		if err != nil {
			glog.Fatal(err)
		}
		if err := startControllers(controllerContext); err != nil {
			glog.Fatal(err)
		}
		controllerContext.StartInformers(stopCh)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-controller-manager"})
	id, err := os.Hostname()
	if err != nil {
		return err
	}
	rl, err := resourcelock.New(
		"configmaps",
		"kube-system",
		"openshift-master-controllers", // this matches what ansible used to set
		kubeClient.CoreV1(),
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
func startControllers(controllerContext *origincontrollers.ControllerContext) error {
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

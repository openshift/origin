package openshift_network_controller

import (
	"context"
	"os"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager"
	origincontrollers "github.com/openshift/origin/pkg/cmd/openshift-controller-manager/controller"
	"github.com/openshift/origin/pkg/cmd/util"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOpenShiftNetworkController(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig, clientConfig *rest.Config) error {
	util.InitLogrus()
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	originControllerManager := func(ctx context.Context) {
		if err := openshift_controller_manager.WaitForHealthyAPIServer(kubeClient.Discovery().RESTClient()); err != nil {
			glog.Fatal(err)
		}

		controllerContext, err := origincontrollers.NewControllerContext(*config, clientConfig, nil)
		if err != nil {
			glog.Fatal(err)
		}
		enabled, err := origincontrollers.RunSDNController(controllerContext)
		if err != nil {
			glog.Fatalf("Error starting OpenShift Network Controller: %v", err)
		} else if !enabled {
			glog.Fatalf("OpenShift Network Controller requested, but not running an OpenShift Network plugin")
		}
		glog.Infof("Started OpenShift Network Controller")
		controllerContext.StartInformers(nil)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-network-controller"})
	id, err := os.Hostname()
	if err != nil {
		return err
	}
	rl, err := resourcelock.New(
		"configmaps",
		"openshift-sdn",
		"openshift-network-controller",
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return err
	}
	go leaderelection.RunOrDie(context.Background(),
		leaderelection.LeaderElectionConfig{
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

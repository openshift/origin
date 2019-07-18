package openshift_network_controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	configv1 "github.com/openshift/api/config/v1"
	leaderelectionconverter "github.com/openshift/library-go/pkg/config/leaderelection"
	"github.com/openshift/library-go/pkg/serviceability"
	sdnmaster "github.com/openshift/sdn/pkg/network/master"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOpenShiftNetworkController() error {
	serviceability.InitLogrusFromKlog()

	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	originControllerManager := func(ctx context.Context) {
		if err := WaitForHealthyAPIServer(kubeClient.Discovery().RESTClient()); err != nil {
			klog.Fatal(err)
		}

		controllerContext, err := newControllerContext(clientConfig)
		if err != nil {
			klog.Fatal(err)
		}
		if err := sdnmaster.Start(
			controllerContext.NetworkClient,
			controllerContext.KubernetesClient,
			controllerContext.KubernetesInformers,
			controllerContext.NetworkInformers,
		); err != nil {
			klog.Fatalf("Error starting OpenShift Network Controller: %v", err)
		}
		klog.Infof("Started OpenShift Network Controller")
		controllerContext.StartInformers()
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-network-controller"})
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	leaderConfig := leaderelectionconverter.LeaderElectionDefaulting(configv1.LeaderElection{}, "openshift-sdn", "openshift-network-controller")
	rl, err := resourcelock.New(
		"configmaps",
		leaderConfig.Namespace,
		leaderConfig.Name,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
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
			LeaseDuration: leaderConfig.LeaseDuration.Duration,
			RenewDeadline: leaderConfig.RenewDeadline.Duration,
			RetryPeriod:   leaderConfig.RetryPeriod.Duration,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: originControllerManager,
				OnStoppedLeading: func() {
					klog.Fatalf("leaderelection lost")
				},
			},
		})

	return nil
}

func WaitForHealthyAPIServer(client rest.Interface) error {
	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		healthStatus := 0
		resp := client.Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			klog.Errorf("Server isn't healthy yet. Waiting a little while.")
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

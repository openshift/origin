package openshift_network_controller

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/network/networkutils"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager"
	sdnmaster "github.com/openshift/origin/pkg/network/master"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOpenShiftNetworkController(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig, clientConfig *rest.Config) error {
	serviceability.InitLogrusFromKlog()
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	originControllerManager := func(ctx context.Context) {
		if err := openshift_controller_manager.WaitForHealthyAPIServer(kubeClient.Discovery().RESTClient()); err != nil {
			klog.Fatal(err)
		}

		controllerContext, err := NewControllerContext(*config, clientConfig, nil)
		if err != nil {
			klog.Fatal(err)
		}
		enabled, err := runSDNController(controllerContext)
		if err != nil {
			klog.Fatalf("Error starting OpenShift Network Controller: %v", err)
		} else if !enabled {
			klog.Fatalf("OpenShift Network Controller requested, but not running an OpenShift Network plugin")
		}
		klog.Infof("Started OpenShift Network Controller")
		controllerContext.StartInformers(nil)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
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
			LeaseDuration: config.LeaderElection.LeaseDuration.Duration,
			RenewDeadline: config.LeaderElection.RenewDeadline.Duration,
			RetryPeriod:   config.LeaderElection.RetryPeriod.Duration,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: originControllerManager,
				OnStoppedLeading: func() {
					klog.Fatalf("leaderelection lost")
				},
			},
		})

	return nil
}

func runSDNController(ctx *ControllerContext) (bool, error) {
	if !isOpenShiftNetworkPlugin(ctx.OpenshiftControllerConfig.Network.NetworkPluginName) {
		return false, nil
	}

	if err := sdnmaster.Start(
		ctx.OpenshiftControllerConfig.Network,
		ctx.NetworkClient,
		ctx.KubernetesClient,
		ctx.KubernetesInformers,
		ctx.NetworkInformers,
	); err != nil {
		return false, fmt.Errorf("failed to start SDN plugin controller: %v", err)
	}

	return true, nil
}

func isOpenShiftNetworkPlugin(pluginName string) bool {
	switch strings.ToLower(pluginName) {
	case networkutils.SingleTenantPluginName, networkutils.MultiTenantPluginName, networkutils.NetworkPolicyPluginName:
		return true
	}
	return false
}

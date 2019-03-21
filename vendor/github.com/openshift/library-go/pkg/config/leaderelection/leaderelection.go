package leaderelection

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	configv1 "github.com/openshift/api/config/v1"
)

// ToConfigMapLeaderElection returns a leader election config that you just need to fill in the Callback for.  Don't forget the callbacks!
func ToConfigMapLeaderElection(clientConfig *rest.Config, config configv1.LeaderElection, component, identity string) (leaderelection.LeaderElectionConfig, error) {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return leaderelection.LeaderElectionConfig{}, err
	}

	if len(identity) == 0 {
		identity = string(uuid.NewUUID())
	}
	if len(config.Namespace) == 0 {
		return leaderelection.LeaderElectionConfig{}, fmt.Errorf("namespace may not be empty")
	}
	if len(config.Name) == 0 {
		return leaderelection.LeaderElectionConfig{}, fmt.Errorf("name may not be empty")
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(clientgoscheme.Scheme, corev1.EventSource{Component: component})
	rl, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		config.Namespace,
		config.Name,
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      identity,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return leaderelection.LeaderElectionConfig{}, err
	}

	return leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: config.LeaseDuration.Duration,
		RenewDeadline: config.RenewDeadline.Duration,
		RetryPeriod:   config.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	}, nil
}

// LeaderElectionDefaulting applies what we think are reasonable defaults.  It does not mutate the original.
// We do defaulting outside the API so that we can change over time and know whether the user intended to override our values
// as opposed to simply getting the defaulted serialization at some point.
func LeaderElectionDefaulting(config configv1.LeaderElection, defaultNamespace, defaultName string) configv1.LeaderElection {
	ret := *(&config).DeepCopy()

	if ret.LeaseDuration.Duration == 0 {
		ret.LeaseDuration.Duration = 120 * time.Second
	}
	if ret.RenewDeadline.Duration == 0 {
		ret.RenewDeadline.Duration = 90 * time.Second
	}
	if ret.RetryPeriod.Duration == 0 {
		ret.RetryPeriod.Duration = 20 * time.Second
	}
	if len(ret.Namespace) == 0 {
		if len(defaultNamespace) > 0 {
			ret.Namespace = defaultNamespace
		} else {
			// Fall back to the namespace associated with the service account token, if available
			if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
				if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
					ret.Namespace = ns
				}
			}
		}
	}
	if len(ret.Name) == 0 {
		ret.Name = defaultName
	}
	return ret
}

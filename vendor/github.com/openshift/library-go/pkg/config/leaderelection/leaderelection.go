package leaderelection

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"time"

	"k8s.io/klog/v2"

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

// ToLeaderElectionWithLease returns a "leases" based leader
// election config that you just need to fill in the Callback for.
// NOTE: we had switched from "configmaps" to "configmapsleases"
// to give an opportunity for the operators to migrate in a
// backward compatible way. The final step in the migration is
// switch to using Leases.
// See https://github.com/kubernetes/kubernetes/issues/107454 for
// details on how to migrate to "leases" leader election.
//
// Don't forget the callbacks!
func ToLeaderElectionWithLease(clientConfig *rest.Config, config configv1.LeaderElection, component, identity string) (leaderelection.LeaderElectionConfig, error) {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return leaderelection.LeaderElectionConfig{}, err
	}

	if len(identity) == 0 {
		if hostname, err := os.Hostname(); err != nil {
			// on errors, make sure we're unique
			identity = string(uuid.NewUUID())
		} else {
			// add a uniquifier so that two processes on the same host don't accidentally both become active
			identity = hostname + "_" + string(uuid.NewUUID())
		}
	}
	if len(config.Namespace) == 0 {
		return leaderelection.LeaderElectionConfig{}, fmt.Errorf("namespace may not be empty")
	}
	if len(config.Name) == 0 {
		return leaderelection.LeaderElectionConfig{}, fmt.Errorf("name may not be empty")
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(clientgoscheme.Scheme, corev1.EventSource{Component: component})
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		config.Namespace,
		config.Name,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      identity,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return leaderelection.LeaderElectionConfig{}, err
	}

	return leaderelection.LeaderElectionConfig{
		Lock:            rl,
		ReleaseOnCancel: true,
		LeaseDuration:   config.LeaseDuration.Duration,
		RenewDeadline:   config.RenewDeadline.Duration,
		RetryPeriod:     config.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStoppedLeading: func() {
				defer os.Exit(0)
				klog.Warningf("leader election lost")
			},
		},
	}, nil
}

// LeaderElectionDefaulting applies what we think are reasonable defaults.  It does not mutate the original.
// We do defaulting outside the API so that we can change over time and know whether the user intended to override our values
// as opposed to simply getting the defaulted serialization at some point.
func LeaderElectionDefaulting(config configv1.LeaderElection, defaultNamespace, defaultName string) configv1.LeaderElection {
	ret := *(&config).DeepCopy()

	// We want to be able to tolerate 60s of kube-apiserver disruption without causing pod restarts.
	// We want the graceful lease re-acquisition fairly quick to avoid waits on new deployments and other rollouts.
	// We want a single set of guidance for nearly every lease in openshift.  If you're special, we'll let you know.
	// 1. clock skew tolerance is leaseDuration-renewDeadline == 30s
	// 2. kube-apiserver downtime tolerance is == 78s
	//      lastRetry=floor(renewDeadline/retryPeriod)*retryPeriod == 104
	//      downtimeTolerance = lastRetry-retryPeriod == 78s
	// 3. worst non-graceful lease acquisition is leaseDuration+retryPeriod == 163s
	// 4. worst graceful lease acquisition is retryPeriod == 26s
	if ret.LeaseDuration.Duration == 0 {
		ret.LeaseDuration.Duration = 137 * time.Second
	}

	if ret.RenewDeadline.Duration == 0 {
		// this gives 107/26=4 retries and allows for 137-107=30 seconds of clock skew
		// if the kube-apiserver is unavailable for 60s starting just before t=26 (the first renew),
		// then we will retry on 26s intervals until t=104 (kube-apiserver came back up at 86), and there will
		// be 33 seconds of extra time before the lease is lost.
		ret.RenewDeadline.Duration = 107 * time.Second
	}
	if ret.RetryPeriod.Duration == 0 {
		ret.RetryPeriod.Duration = 26 * time.Second
	}

	retryTimes := int(math.Floor(float64(ret.RenewDeadline.Duration / ret.RetryPeriod.Duration)))
	klog.Infof("The leader election gives %v retries and allows for %v of clock skew. The kube-apiserver downtime tolerance is %vs. Worst non-graceful lease acquisition is %v. Worst graceful lease acquisition is %v.",
		retryTimes,
		ret.LeaseDuration.Duration-ret.RenewDeadline.Duration,
		(retryTimes-1)*(int(ret.RetryPeriod.Duration.Seconds())),
		ret.LeaseDuration.Duration+ret.RetryPeriod.Duration,
		ret.RetryPeriod,
	)

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

// LeaderElectionSNOConfig uses the formula derived in LeaderElectionDefaulting with increased
// retry period and lease duration for SNO clusters that have limited resources.
// This method does not respect the passed in LeaderElection config and the returned object will have values
// that are overridden with SNO environments in mind.
// This method should only be called when running in an SNO Cluster.
func LeaderElectionSNOConfig(config configv1.LeaderElection) configv1.LeaderElection {

	// We want to make sure we respect a 30s clock skew as well as a 4 retry attempt with out making
	// leader election ineffectual while still having some small performance gain by limiting calls against
	// the api server.

	// 1. clock skew tolerance is leaseDuration-renewDeadline == 30s
	// 2. kube-apiserver downtime tolerance is == 180s
	//      lastRetry=floor(renewDeadline/retryPeriod)*retryPeriod == 240
	//      downtimeTolerance = lastRetry-retryPeriod == 180s
	// 3. worst non-graceful lease acquisition is leaseDuration+retryPeriod == 330s
	// 4. worst graceful lease acquisition is retryPeriod == 60s

	ret := *(&config).DeepCopy()
	// 270-240 = 30s of clock skew tolerance
	ret.LeaseDuration.Duration = 270 * time.Second
	// 240/60 = 4 retries attempts before leader is lost.
	ret.RenewDeadline.Duration = 240 * time.Second
	// With 60s retry config we aim to maintain 30s of clock skew as well as 4 retry attempts.
	ret.RetryPeriod.Duration = 60 * time.Second
	return ret
}

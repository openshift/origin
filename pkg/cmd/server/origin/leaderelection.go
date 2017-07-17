package origin

import (
	"fmt"
	"path"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilrand "k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	v1corev1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	rl "k8s.io/kubernetes/pkg/client/leaderelection/resourcelock"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/util/leaderlease"
)

// NewLeaderElection returns a plug that blocks controller startup until the lease is acquired
// and a function that will start the process to attain the lease. There are two modes for
// lease operation - a legacy mode that directly connects to etcd, and the preferred mode which
// coordinates on a service endpoints object in the kube-system namespace. The legacy mode will
// periodically poll to see if the endpoints object exists, and if so will stand down, allowing
// newer controllers to take over.
func NewLeaderElection(options configapi.MasterConfig, leader componentconfig.LeaderElectionConfiguration, kc kclientsetexternal.Interface) (plug.Plug, func(), error) {
	id := fmt.Sprintf("master-%s", kutilrand.String(8))
	name := "openshift-controller-manager"
	namespace := "kube-system"
	useEndpoints := false
	if election := options.ControllerConfig.Election; election != nil {
		if election.LockResource.Resource != "endpoints" || election.LockResource.Group != "" {
			return nil, nil, fmt.Errorf("only the \"endpoints\" resource is supported for election")
		}
		name = election.LockName
		namespace = election.LockNamespace
		useEndpoints = true
	}

	lock := &rl.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Client:        kc.Core(),
		LockConfig: rl.ResourceLockConfig{
			Identity: id,
		},
	}

	// legacy path, for native etcd leases. Will periodically check for the controller service to exist and
	// release any held lease if one is detected
	if !useEndpoints {
		ttl := time.Duration(options.ControllerLeaseTTL) * time.Second
		if ttl == 0 {
			return plug.New(!options.PauseControllers), func() {}, nil
		}

		client, err := etcd.MakeEtcdClient(options.EtcdClientInfo)
		if err != nil {
			return nil, nil, err
		}

		leaser := leaderlease.NewEtcd(
			client,
			path.Join(options.EtcdStorageConfig.OpenShiftStoragePrefix, "leases/controllers"),
			id,
			uint64(options.ControllerLeaseTTL),
		)

		leased := plug.NewLeased(leaser)
		return leased, legacyLeaderElectionStart(id, name, leased, lock, ttl), nil
	}

	// use the endpoints leader election path.
	plug := plug.New(false)
	events := record.NewBroadcaster()
	events.StartLogging(glog.Infof)
	events.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kc.Core().RESTClient()).Events("")})
	lock.LockConfig.EventRecorder = events.NewRecorder(kapi.Scheme, v1corev1.EventSource{Component: name})
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: leader.LeaseDuration.Duration,
		RenewDeadline: leader.RenewDeadline.Duration,
		RetryPeriod:   leader.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				plug.Start()
			},
			OnStoppedLeading: func() {
				plug.Stop(fmt.Errorf("%s %s lost election, stepping down", name, id))
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return plug, func() {
		glog.V(2).Infof("Attempting to acquire %s lease as %s, renewing every %s, holding for %s, and giving up after %s", name, id, leader.RetryPeriod.Duration, leader.LeaseDuration.Duration, leader.RenewDeadline.Duration)
		go elector.Run()
	}, nil
}

// legacyLeaderElectionStart waits to verify lock has not been taken, then attempts to acquire and hold
// the legacy lease. If it detects the lock is acquired it will stop immediately.
func legacyLeaderElectionStart(id, name string, leased *plug.Leased, lock rl.Interface, ttl time.Duration) func() {
	return func() {
		glog.V(2).Infof("Verifying no controller manager is running for %s", id)
		wait.PollInfinite(ttl/2, func() (bool, error) {
			_, err := lock.Get()
			if err == nil {
				return false, nil
			}
			if kapierrors.IsNotFound(err) {
				return true, nil
			}
			utilruntime.HandleError(fmt.Errorf("unable to confirm %s lease exists: %v", name, err))
			return false, nil
		})
		glog.V(2).Infof("Attempting to acquire controller lease as %s, renewing every %s", id, ttl)
		go leased.Run()
		go wait.PollInfinite(ttl/2, func() (bool, error) {
			_, err := lock.Get()
			if err == nil {
				glog.V(2).Infof("%s lease has been taken, %s is exiting", name, id)
				leased.Stop(nil)
				return true, nil
			}
			// NotFound indicates the endpoint is missing and the etcd lease should continue to be held
			if !kapierrors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("unable to confirm %s lease exists: %v", name, err))
			}
			return false, nil
		})
	}
}

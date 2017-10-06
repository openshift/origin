package origin

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"

	kutilrand "k8s.io/apimachinery/pkg/util/rand"
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
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/util/leaderlease"
)

// NewLeaderElection returns a plug that blocks controller startup until the lease is acquired
// and a function that will start the process to attain the lease. There are two modes for
// lease operation - a legacy mode that directly connects to etcd, and the preferred mode which
// coordinates on a service endpoints object in the kube-system namespace. Because legacy mode
// and the new mode do not coordinate on the same key, an upgrade must stop all controllers before
// changing the configuration and starting controllers with the new config.
func NewLeaderElection(options configapi.MasterConfig, leader componentconfig.LeaderElectionConfiguration, kc kclientsetexternal.Interface, eventClient v1core.EventInterface) (plug.Plug, func(), error) {
	id := getUniqueLeaderID()

	election := options.ControllerConfig.Election
	if election == nil {
		// legacy path, for native etcd leases
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
		return leased, func() {
			glog.V(2).Infof("Attempting to acquire controller lease as %s, renewing every %s", id, ttl)
			go leased.Run()
		}, nil
	}

	switch election.LockResource {
	case configapi.GroupResource{Resource: "endpoints"}, configapi.GroupResource{Resource: "configmaps"}:
	default:
		return nil, nil, fmt.Errorf("only the \"endpoints\" or \"configmaps\" resource is supported for leader election")
	}

	name := election.LockName
	namespace := election.LockNamespace

	events := record.NewBroadcaster()
	events.StartLogging(glog.Infof)
	events.StartRecordingToSink(&v1core.EventSinkImpl{Interface: eventClient})
	lock, err := rl.New(election.LockResource.Resource, namespace, name, kc.Core(), rl.ResourceLockConfig{
		Identity:      id,
		EventRecorder: events.NewRecorder(kapi.Scheme, v1corev1.EventSource{Component: name}),
	})
	if err != nil {
		return nil, nil, err
	}

	// use the endpoints leader election path.
	plug := plug.New(false)
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

// getUniqueLeaderID returns a unique name based on the machine's hostname and ip.
// This makes it easy to determine which machine is the current controller leader.
func getUniqueLeaderID() string {
	hostname := "unknown_hostname"
	ip := "unknown_ip"
	if realHostname, _ := os.Hostname(); len(realHostname) > 0 {
		hostname = strings.ToLower(strings.TrimSpace(realHostname))
	}
	if realIP, _ := util.DefaultLocalIP4(); realIP != nil {
		ip = realIP.String()
	}
	return fmt.Sprintf("master-%s-%s-%s", hostname, ip, kutilrand.String(8))
}

package openshift_sdn

import (
	"sync"

	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
)

// NotifyingHealthzServer is a shim around the existing proxy healthcheck
// code, so we can wait until the proxy has successfully reached health.
// It would be nice to get rid of this in favor of proper status reporting
// upstream.
type NotifyingHealthzServer struct {
	*healthcheck.HealthzServer

	// Channel that is closed when Update is called once
	UpdatedOnce chan struct{}
	updateCount int

	mu sync.Mutex
}

func NewNotifyingHealthzServer(s *healthcheck.HealthzServer) *NotifyingHealthzServer {
	return &NotifyingHealthzServer{
		s,
		make(chan struct{}),
		0,
		sync.Mutex{},
	}
}

// UpdateTimestamp hooks in to the proxy's health reporting mechanism by
// closing the Updated channel after the *second* update.

// This is because the proxy always calls UpdateTimestamp once on startup,
// then after every subsequent sync.
func (nhz *NotifyingHealthzServer) UpdateTimestamp() {
	klog.V(3).Infof("UpdateTimestamp called")
	nhz.mu.Lock()
	defer nhz.mu.Unlock()

	if nhz.updateCount < 4 {
		nhz.updateCount++
	}

	// On the second update, close the channel.
	if nhz.updateCount == 2 {
		close(nhz.UpdatedOnce)
	}

	if nhz.HealthzServer != nil {
		nhz.HealthzServer.UpdateTimestamp()
	}
}

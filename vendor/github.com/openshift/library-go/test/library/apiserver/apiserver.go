package apiserver

import (
	"time"

	"github.com/openshift/library-go/test/library"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	// the following parameters specify for how long apis must
	// stay on the same revision to be considered stable
	waitForAPIRevisionSuccessThreshold = 6
	waitForAPIRevisionSuccessInterval  = 1 * time.Minute

	// the following parameters specify max timeout after which
	// apis are considered to not converged
	waitForAPIRevisionPollInterval = 30 * time.Second
	waitForAPIRevisionTimeout      = 22 * time.Minute
)

// WaitForAPIServerToStabilizeOnTheSameRevision waits until all API Servers are running at the same revision.
// The API Servers must stay on the same revision for at least waitForAPIRevisionSuccessThreshold * waitForAPIRevisionSuccessInterval.
// Mainly because of the difference between the propagation time of triggering a new release and the actual roll-out.
//
// Observations:
//
//	rolling out a new version is not instant you need to account for a propagation time (~1/2 minutes)
//	for some API servers (KAS) rolling out a new version can take ~10 minutes
//
// Note:
//
//	the number of instances is calculated based on the number of running pods in a namespace.
//	only pods with apiserver=true label are considered
//	only pods in the given namespace are considered (podClient)
func WaitForAPIServerToStabilizeOnTheSameRevision(t library.LoggingT, podClient corev1client.PodInterface) error {
	return library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "apiserver=true", waitForAPIRevisionSuccessThreshold, waitForAPIRevisionSuccessInterval, waitForAPIRevisionPollInterval, waitForAPIRevisionTimeout)
}

package hostconnectionintegrity

import (
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
)

const (
	// SourceGatewayReconcile marks an ovnkube-node gateway reconcile ("Reconciling gateway with updates").
	SourceGatewayReconcile monitorapi.IntervalSource = "OVNKubeGatewayReconcile"
	// SourceUDNTeardown marks the teardown of a user defined network on a node.
	SourceUDNTeardown monitorapi.IntervalSource = "OVNKubeUDNTeardown"

	gatewayReconcileLine = "Reconciling gateway with updates"
)

// udnTeardownLines are ovnkube-controller log lines emitted when a UDN is torn down on a node.
var udnTeardownLines = []string{
	"Delete OVN logical entities for layer3 network controller",
	"Stopping UDN node network controller",
}

// ovnkubeControllerLogHandler turns ovnkube-controller log lines into point intervals keyed by node.
type ovnkubeControllerLogHandler struct {
	notBefore time.Time

	lock      sync.Mutex
	intervals monitorapi.Intervals
}

func newOVNKubeControllerLogHandler(notBefore time.Time) *ovnkubeControllerLogHandler {
	return &ovnkubeControllerLogHandler{notBefore: notBefore}
}

func (h *ovnkubeControllerLogHandler) HandleLogLine(line podaccess.LogLineContent) {
	if line.Pod == nil || line.Instant.IsZero() || line.Instant.Before(h.notBefore) {
		return
	}
	var source monitorapi.IntervalSource
	switch {
	case strings.Contains(line.Line, gatewayReconcileLine):
		source = SourceGatewayReconcile
	case containsAny(line.Line, udnTeardownLines):
		source = SourceUDNTeardown
	default:
		return
	}
	interval := monitorapi.NewInterval(source, monitorapi.Info).
		Locator(monitorapi.NewLocator().NodeFromName(line.Pod.Spec.NodeName)).
		Message(monitorapi.NewMessage().HumanMessage(truncate(line.Line, 300))).
		Build(line.Instant, line.Instant)

	h.lock.Lock()
	defer h.lock.Unlock()
	h.intervals = append(h.intervals, interval)
}

func (h *ovnkubeControllerLogHandler) Intervals() monitorapi.Intervals {
	h.lock.Lock()
	defer h.lock.Unlock()
	return append(monitorapi.Intervals{}, h.intervals...)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

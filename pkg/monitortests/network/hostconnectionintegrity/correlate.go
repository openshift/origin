package hostconnectionintegrity

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortests/network/hostconnectionintegrity/poller"
)

const (
	// SourceBlackholedAfterGatewayReconcile is the computed interval for an established connection that broke
	// right around a gateway reconcile on the same node.
	SourceBlackholedAfterGatewayReconcile monitorapi.IntervalSource = "EstablishedConnectionBlackholedAfterGatewayReconcile"

	// correlationSlack is added on both sides of the window in which the failure of an established connection
	// must have begun. In the CI runs behind OCPBUGS-128289 the connection broke 1-30ms after the reconcile.
	correlationSlack = 250 * time.Millisecond

	// minOVSStall is the ovs-vswitchd poll interval above which a failure is attributed to an ovs-vswitchd
	// stall (tracked separately) instead of the gateway reconcile.
	minOVSStall = 1000 * time.Millisecond
)

var ovsPollIntervalRegex = regexp.MustCompile(`Unreasonably long (\d+)ms poll interval`)

// Finding is one correlated black-hole event.
type Finding struct {
	Node           string
	Backend        string
	Target         string
	Reason         string
	Onset          time.Time
	OnsetUpTo      time.Time
	Recovered      time.Time
	Reconcile      time.Time
	ReconcileDelta time.Duration // time from the reconcile to the start of the first failed probe
	LocalAddr      string
}

// FailureLine is the stable one-line description used in the junit failure output. Sippy symptoms match on
// the "established connection blackholed after gateway reconcile" prefix.
func (f Finding) FailureLine() string {
	return fmt.Sprintf("established connection blackholed after gateway reconcile: node=%s backend=%s target=%s reason=%s onset=%s..%s reconcile=%s reconcileBeforeFailedProbe=%dms duration=%s localAddr=%s",
		f.Node, f.Backend, f.Target, f.Reason,
		f.Onset.UTC().Format("15:04:05.000"), f.OnsetUpTo.UTC().Format("15:04:05.000"),
		f.Reconcile.UTC().Format("15:04:05.000"), f.ReconcileDelta.Milliseconds(),
		f.Recovered.Sub(f.Onset).Round(10*time.Millisecond), f.LocalAddr)
}

// onsetWindow returns the window in which the failure of the established connection began: after the last
// successful probe (From) and no later than the start of the first failed probe.
func onsetWindow(i monitorapi.Interval) (time.Time, time.Time) {
	upTo := i.To
	if s := i.Message.Annotations[poller.AnnotationFailedProbeStart]; len(s) > 0 {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil && !t.Before(i.From) {
			upTo = t
		}
	}
	return i.From, upTo
}

func nodeOf(i monitorapi.Interval) string {
	if n := i.Locator.Keys[monitorapi.LocatorNodeKey]; len(n) > 0 {
		return n
	}
	// APIServerGracefulShutdown intervals carry the pod name kube-apiserver-<node>.
	return strings.TrimPrefix(i.Locator.Keys[monitorapi.LocatorPodKey], "kube-apiserver-")
}

func overlaps(i monitorapi.Interval, from, to time.Time) bool {
	iTo := i.To
	if iTo.IsZero() {
		// open ended interval.
		iTo = to
	}
	return !i.From.After(to) && !iTo.Before(from)
}

func ovsStallMs(i monitorapi.Interval) int {
	m := ovsPollIntervalRegex.FindStringSubmatch(i.Message.HumanMessage)
	if len(m) != 2 {
		return 0
	}
	ms, _ := strconv.Atoi(m[1])
	return ms
}

// UDNTeardownObserved reports whether any UDN was torn down during the run.
func UDNTeardownObserved(intervals monitorapi.Intervals) bool {
	for _, i := range intervals {
		if i.Source == SourceUDNTeardown {
			return true
		}
	}
	return false
}

// Correlate finds Reset/Stalled failures of established host-network connections whose onset window contains
// a gateway reconcile on the same node, excluding failures explained by a kube-apiserver graceful shutdown
// (for the apiserver backends, see OCPBUGS-100298) or by an ovs-vswitchd stall on the same node.
func Correlate(intervals monitorapi.Intervals) []Finding {
	reconciles := map[string][]time.Time{}
	var shutdowns, ovsStalls monitorapi.Intervals
	for _, i := range intervals {
		switch i.Source {
		case SourceGatewayReconcile:
			n := nodeOf(i)
			reconciles[n] = append(reconciles[n], i.From)
		case monitorapi.APIServerGracefulShutdown:
			shutdowns = append(shutdowns, i)
		case monitorapi.SourceOVSVswitchdLog:
			if time.Duration(ovsStallMs(i))*time.Millisecond >= minOVSStall {
				ovsStalls = append(ovsStalls, i)
			}
		}
	}
	for n := range reconciles {
		sort.Slice(reconciles[n], func(a, b int) bool { return reconciles[n][a].Before(reconciles[n][b]) })
	}

	findings := []Finding{}
	for _, i := range intervals {
		if i.Source != poller.IntervalSource {
			continue
		}
		reason := string(i.Message.Reason)
		if reason != string(poller.ReasonReset) && reason != string(poller.ReasonStalled) {
			continue
		}
		node := nodeOf(i)
		backend := i.Locator.Keys[poller.LocatorBackendKey]
		onset, upTo := onsetWindow(i)
		from, to := onset.Add(-correlationSlack), upTo.Add(correlationSlack)

		excluded := false
		if backend != poller.BackendPeerKubelet {
			for _, s := range shutdowns {
				// localhost-apiserver only talks to this node's kube-apiserver; api-int is load balanced.
				if backend == poller.BackendLocalhostAPIServer && nodeOf(s) != node {
					continue
				}
				if overlaps(s, from, to) {
					excluded = true
					break
				}
			}
		}
		for _, s := range ovsStalls {
			if nodeOf(s) != node {
				continue
			}
			// the warning is logged when the stalled iteration ends, so the stall itself precedes From.
			stallStart := s.From.Add(-time.Duration(ovsStallMs(s)) * time.Millisecond)
			if !stallStart.After(to) && !s.To.Before(from) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		for _, r := range reconciles[node] {
			if r.Before(from) {
				continue
			}
			if r.After(to) {
				break
			}
			findings = append(findings, Finding{
				Node:           node,
				Backend:        backend,
				Target:         i.Locator.Keys[poller.LocatorTargetKey],
				Reason:         reason,
				Onset:          onset,
				OnsetUpTo:      upTo,
				Recovered:      i.To,
				Reconcile:      r,
				ReconcileDelta: upTo.Sub(r),
				LocalAddr:      i.Message.Annotations[poller.AnnotationLocalAddr],
			})
			break
		}
	}
	sort.Slice(findings, func(a, b int) bool { return findings[a].Onset.Before(findings[b].Onset) })
	return findings
}

// FindingToInterval converts a finding to a computed interval.
func FindingToInterval(f Finding) monitorapi.Interval {
	return monitorapi.NewInterval(SourceBlackholedAfterGatewayReconcile, monitorapi.Error).
		Locator(monitorapi.Locator{
			Type: monitorapi.LocatorTypeNode,
			Keys: map[monitorapi.LocatorKey]string{
				monitorapi.LocatorNodeKey: f.Node,
				poller.LocatorBackendKey:  f.Backend,
				poller.LocatorTargetKey:   f.Target,
			},
		}).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.IntervalReason(f.Reason)).
			HumanMessage(f.FailureLine())).
		Display().
		Build(f.Onset, f.Recovered)
}

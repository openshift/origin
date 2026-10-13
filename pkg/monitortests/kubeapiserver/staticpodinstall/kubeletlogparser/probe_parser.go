package kubeletlogparser

import (
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
)

type SyncLoopProbeParser struct {
	source monitorapi.IntervalSource
	want   filterFunc
}

var probeRegExp = regexp.MustCompile(`probe="(?P<PROBE>[a-z]+)" status="(?P<STATUS>[a-z\s]*)" pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)"`)

// Parse parses a SyncLoop probe event line from kubelet log, the line has the followng format:
// [kubelet.go:2542] "SyncLoop (probe)" probe="readiness" status="" pod="openshift-etcd/etcd-ci-op-bzbjn2bk-206af-gfdsw-master-2"
func (p SyncLoopProbeParser) Parse(node, line string) (monitorapi.Intervals, bool) {
	_, after, found := strings.Cut(line, `"SyncLoop (probe)"`)
	if !found {
		// not a probe event, let the other parsers inspect
		return nil, false
	}

	// probe="readiness" status="not ready" pod="openshift-etcd/etcd-ci-op-bzbjn2bk-206af-gfdsw-master-2"
	subMatches := probeRegExp.FindStringSubmatch(after)
	names := probeRegExp.SubexpNames()
	if len(subMatches) == 0 || len(names) > len(subMatches) {
		return nil, true
	}
	var probeType, status, podNamespace, podName string
	for i, name := range names {
		switch name {
		case "NS":
			podNamespace = subMatches[i]
		case "POD":
			podName = subMatches[i]
		case "PROBE":
			probeType = subMatches[i]
		case "STATUS":
			status = subMatches[i]
		}
	}

	if p.want != nil && !p.want(node, podNamespace, podName) {
		return nil, true
	}

	// older version of kubelet uses empty string to denote not ready
	if status == "" {
		status = "not ready"
	}
	at := utility.SystemdJournalLogTime(line, time.Now().Year())
	locator := monitorapi.NewLocator().KubeletSyncLoopProbe(node, podNamespace, podName, probeType)
	interval := monitorapi.NewInterval(p.source, monitorapi.Info).
		Locator(locator).
		Message(
			monitorapi.NewMessage().
				Reason(monitorapi.IntervalReason(status)).
				Node(node).
				WithAnnotation("probe", probeType).
				WithAnnotation("status", status).
				HumanMessage("kubelet SyncLoop probe"),
		).Build(at, at)
	return monitorapi.Intervals{interval}, true
}

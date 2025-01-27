package kubeletlogparser

import (
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
)

type SyncLoopPLEGParser struct {
	source monitorapi.IntervalSource
	filter filterFunc
}

var (
	plegRegExp = regexp.MustCompile(`pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)" event={"ID":"[a-z0-9.-]+","Type":"(?P<TYPE>[a-zA-Z]+)"`)

	//TODO:  using the well defined container reasons in use today will
	// cause the following job to fail:
	//  platform pods in ns/openshift-etcd should not exit an excessive amount of times
	// so for now we will use the names reported by kubelet
	plegToContainerReasons = map[string]monitorapi.IntervalReason{
		"ContainerStarted": monitorapi.ContainerReasonContainerStart,
		"ContainerDied":    monitorapi.ContainerReasonContainerExit,
	}
)

// Parse parses a PLEG pod event from the kubelet log and generates an
// appropriate interval for it, the format of a PLEG line is as follows:
// "SyncLoop (PLEG): event for pod" pod="openshift-etcd/installer-4-ci-op-bzbjn2bk-206af-gfdsw-master-2" event={"ID":"0d817ff9-f980-46f0-b046-57ee340e2d38","Type":"ContainerStarted","Data":"f8d11fe0b65575141b38a7310faebaff0b287779bc27d3c635a144891a2304fa"}
func (p SyncLoopPLEGParser) Parse(node, line string) (monitorapi.Intervals, bool) {
	_, after, found := strings.Cut(line, `"SyncLoop (PLEG): event for pod"`)
	if !found {
		// not a probe event, let the other parsers inspect
		return nil, false
	}

	subMatches := plegRegExp.FindStringSubmatch(after)
	names := plegRegExp.SubexpNames()
	if len(subMatches) == 0 || len(names) > len(subMatches) {
		return nil, true
	}

	var podNamespace, podName, eventType string
	for i, name := range names {
		switch name {
		case "NS":
			podNamespace = subMatches[i]
		case "POD":
			podName = subMatches[i]
		case "TYPE":
			eventType = subMatches[i]
		}
	}

	if p.filter != nil && !p.filter(node, podNamespace, podName) {
		return nil, true
	}

	at := utility.SystemdJournalLogTime(line, time.Now().Year())
	locator := monitorapi.NewLocator().KubeletSyncLoopPLEG(node, podNamespace, podName, eventType)
	interval := monitorapi.NewInterval(p.source, monitorapi.Info).
		Locator(locator).
		Message(
			monitorapi.NewMessage().
				Reason(monitorapi.IntervalReason(eventType)).
				Node(node).
				WithAnnotation("type", eventType).
				HumanMessage("kubelet PLEG event"),
		).Build(at, at)
	return monitorapi.Intervals{interval}, true
}

package kubeletlogparser

import (
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// NewEtcdStaticPodEventsFromKubelet returns a parser that will parse kubelet
// log for SyncLoop PLEG and probe events.
// For now, we are interested in the etcd installer and the etcd static pod
func NewEtcdStaticPodEventsFromKubelet() func(node, line string) monitorapi.Intervals {
	p := parsers{
		// we want to observe the unready window for ectd static pods
		&SyncLoopProbeParser{
			source: monitorapi.SourceKubeletLog,
			want: func(node, ns, podName string) bool {
				return ns == "openshift-etcd" && podName == "etcd-"+node
			},
		},
		// we want to observe the container start and exit pleg events for the etcd installer pods
		&SyncLoopPLEGParser{
			source: monitorapi.SourceKubeletLog,
			filter: func(node, ns, podName string) bool {
				return ns == "openshift-etcd" && strings.HasPrefix(podName, "installer-")
			},
		},
	}
	return p.parse
}

// interval is created only if the given filter func returns true
type filterFunc func(node, ns, podName string) bool

type parser interface {
	Parse(node, line string) (monitorapi.Intervals, bool)
}

type parsers []parser

// this complies with the signature the default node log parser in origin requires
func (p parsers) parse(node, line string) monitorapi.Intervals {
	accummulated := monitorapi.Intervals{}
	for _, parser := range p {
		intervals, handled := parser.Parse(node, line)
		accummulated = append(accummulated, intervals...)
		if handled {
			return accummulated
		}
	}
	return accummulated
}

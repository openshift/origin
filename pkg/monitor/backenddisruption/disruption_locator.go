package backenddisruption

import (
	"fmt"
	"regexp"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// this entire file should be a separate package with disruption_***, but we are entanged because the sampler lives in monitor
// and the things being started by the monitor are coupled into .Start.
// we also got stuck on writing the disruption backends.  We need a way to track which disruption checks we have started,
// so we can properly write out "zero"

func LocateRouteForDisruptionCheck(ns, name, disruptionBackendName string, connectionType BackendConnectionType) string {
	return fmt.Sprintf("ns/%s route/%s disruption/%s connection/%s", ns, name, disruptionBackendName, connectionType)
}

func LocateDisruptionCheck(disruptionBackendName string, connectionType BackendConnectionType) string {
	return fmt.Sprintf("disruption/%s connection/%s", disruptionBackendName, connectionType)
}

func DisruptionEndedMessage(locator string, connectionType BackendConnectionType) string {
	switch connectionType {
	case NewConnectionType:
		return fmt.Sprintf("%s started responding to GET requests over new connections", locator)
	case ReusedConnectionType:
		return fmt.Sprintf("%s started responding to GET requests over reused connections", locator)
	default:
		return fmt.Sprintf("%s started responding to GET requests over %v connections", locator, "Unknown")
	}
}

// dnsLookupRegex is a specific error we often see when sampling for disruption, which indicates a DNS
// problem in the cluster running openshift-tests, not real disruption in the cluster under test.
// Used to downgrade to a warning instead of an error, and omitted from final disruption numbers and testing.
var dnsLookupRegex = regexp.MustCompile(`dial tcp: lookup.*: i/o timeout`)

const (
	DisruptionBeganEventReason              = "DisruptionBegan"
	DisruptionEndedEventReason              = "DisruptionEnded"
	DisruptionSamplerOutageBeganEventReason = "DisruptionSamplerOutageBegan"
)

// DisruptionBegan examines the error received, attempts to determine if it looks like real disruption to the cluster under test,
// or other problems possibly on the system running the tests/monitor, and returns an appropriate user message, event reason, and monitoring level.
func DisruptionBegan(locator string, connectionType BackendConnectionType, err error) (string, string, monitorapi.EventLevel) {
	if dnsLookupRegex.MatchString(err.Error()) {
		switch connectionType {
		case NewConnectionType:
			return fmt.Sprintf("reason/%s DNS lookup timeouts began for %s GET requests over new connections: %v (likely a problem in cluster running tests, not the cluster under test)",
				DisruptionSamplerOutageBeganEventReason, locator, err), DisruptionSamplerOutageBeganEventReason, monitorapi.Warning
		case ReusedConnectionType:
			return fmt.Sprintf("reason/%s DNS lookup timeouts began for %s GET requests over reused connections: %v (likely a problem in cluster running tests, not the cluster under test)",
				DisruptionSamplerOutageBeganEventReason, locator, err), DisruptionSamplerOutageBeganEventReason, monitorapi.Warning
		default:
			return fmt.Sprintf("reason/%s DNS lookup timeouts began for %s GET requests over %v connections: %v (likely a problem in cluster running tests, not the cluster under test)",
				DisruptionSamplerOutageBeganEventReason, locator, "Unknown", err), DisruptionSamplerOutageBeganEventReason, monitorapi.Warning
		}
	}
	switch connectionType {
	case NewConnectionType:
		return fmt.Sprintf("reason/%s %s stopped responding to GET requests over new connections: %v",
			DisruptionBeganEventReason, locator, err), DisruptionBeganEventReason, monitorapi.Error
	case ReusedConnectionType:
		return fmt.Sprintf("reason/%s %s stopped responding to GET requests over reused connections: %v",
			DisruptionBeganEventReason, locator, err), DisruptionBeganEventReason, monitorapi.Error
	default:
		return fmt.Sprintf("reason/%s %s stopped responding to GET requests over %v connections: %v",
			DisruptionBeganEventReason, locator, "Unknown", err), DisruptionBeganEventReason, monitorapi.Error
	}
}

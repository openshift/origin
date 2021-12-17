package backenddisruption

import (
	"fmt"
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

func DisruptionBeganMessage(locator string, connectionType BackendConnectionType, err error) string {
	switch connectionType {
	case NewConnectionType:
		return fmt.Sprintf("%s stopped responding to GET requests over new connections: %v", locator, err)
	case ReusedConnectionType:
		return fmt.Sprintf("%s stopped responding to GET requests over reused connections: %v", locator, err)
	default:
		return fmt.Sprintf("%s stopped responding to GET requests over %v connections: %v", locator, "Unknown", err)
	}
}

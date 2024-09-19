package legacynodemonitortests

import (
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

type Exclusion struct {
	upgradeJob  bool
	clusterData platformidentification.ClusterData
}

func isThisContainerRestartExcluded(locator string, exclusion Exclusion) bool {
	// This is a list of known container restart failures
	// Our goal is to conquer these restarts.
	// So we are sadly putting these as exceptions.
	// If you discover a container restarting more than 3 times, it is a bug and you should investigate it.
	exceptions := []string{
		"container/metal3-static-ip-set",      // https://issues.redhat.com/browse/OCPBUGS-39314
		"container/ingress-operator",          // https://issues.redhat.com/browse/OCPBUGS-39315
		"container/networking-console-plugin", // https://issues.redhat.com/browse/OCPBUGS-39316
	}

	// Upgrades seem to have a lot of failures.
	// Let's exclude these for now generally.
	if exclusion.upgradeJob {
		return true
	}
	for _, val := range exclusion.clusterData.ClusterVersionHistory {
		if strings.Contains(val, "4.17") {
			return true
		}
	}

	for _, val := range exceptions {
		matched, err := regexp.MatchString(val, locator)
		if err != nil {
			return false
		}
		if matched {
			return true
		}
	}
	return false
}

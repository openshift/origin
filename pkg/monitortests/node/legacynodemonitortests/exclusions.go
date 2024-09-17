package legacynodemonitortests

import (
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

type Exclusion struct {
	clusterData platformidentification.ClusterData
}

func isThisContainerRestartExcluded(locator string, exclusion Exclusion) bool {
	// This is a list of known container restart failures
	// Our goal is to conquer these restarts.
	// So we are sadly putting these as exceptions.
	// If you discover a container restarting more than 3 times, it is a bug and you should investigate it.
	type exceptionVariants struct {
		containerName      string
		platformsToExclude string
		topologyToExclude  string
	}
	exceptions := []exceptionVariants{
		{
			// In this case, we found that we only saw failures for this container on bare metal.
			// We did not find failures for vsphere where this is also run
			// So if we start seeing failures on vsphere this would be a regression.
			containerName:      "container/metal3-static-ip-set", // https://issues.redhat.com/browse/OCPBUGS-39314
			platformsToExclude: "metal",
		},
		{
			// ingress operator seems to only fail on the single topology.
			// platform did not matter.
			containerName:     "container/ingress-operator", // https://issues.redhat.com/browse/OCPBUGS-39315
			topologyToExclude: "single",
		},
		{
			containerName: "container/networking-console-plugin", // https://issues.redhat.com/browse/OCPBUGS-39316
		},
	}

	for _, val := range exclusion.clusterData.ClusterVersionHistory {
		if strings.Contains(val, "4.17") {
			return true
		}
	}

	for _, val := range exceptions {
		matched, err := regexp.MatchString(val.containerName, locator)
		if err != nil {
			return false
		}
		if matched {
			switch {
			// if container matches but platform is different, this is a regression.
			case val.platformsToExclude != "" && val.platformsToExclude == exclusion.clusterData.Platform:
				return false
				// if container matches but topology is different, this is a regression.
			case val.topologyToExclude != "" && val.topologyToExclude == exclusion.clusterData.Topology:
				return false
			default:
				return true
			}
		}
	}
	return false
}

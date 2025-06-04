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
			// ingress operator seems to only fail on the single topology.
			// platform did not matter.
			containerName:     "container/ingress-operator", // https://issues.redhat.com/browse/OCPBUGS-39315
			topologyToExclude: "single",
		},
		{
			// snapshot controller operator seems to fail on SNO during kube api upgrades
			// the error from the pod is the inability to connect to the kas to get volumesnapshots on startup.
			containerName:     "container/snapshot-controller", // https://issues.redhat.com/browse/OCPBUGS-43113
			topologyToExclude: "single",
		},
		{
			containerName: "container/kube-multus", // https://issues.redhat.com/browse/OCPBUGS-42267
		},
		{
			containerName: "container/ovn-acl-logging", // https://issues.redhat.com/browse/OCPBUGS-42344
		},
		{
			containerName: "container/managed-upgrade-operator", // https://issues.redhat.com/browse/OSD-26270
		},
		{
			// Managed services like ROSA. This is expected.
			containerName: "container/osd-cluster-ready",
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
			case val.platformsToExclude != "":
				return val.platformsToExclude == exclusion.clusterData.Platform
				// if container matches but topology is different, this is a regression.
			case val.topologyToExclude != "":
				return val.topologyToExclude == exclusion.clusterData.Topology
			default:
				return true
			}
		}
	}
	return false
}

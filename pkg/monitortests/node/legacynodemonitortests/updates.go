package legacynodemonitortests

import (
	"github.com/openshift/library-go/test/library/junitapi"
	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func testMasterNodesUpdated(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-coreos] master nodes updated"

	// Only return a Junit if we detect that the master nodes were updated
	// Used in sippy to differentiate between jobs where the master nodes update and do not (no junit in that case)
	if "Y" == clusterinfo.WasMasterNodeUpdated(events) {
		return []*junitapi.JUnitTestCase{{
			Name: testName,
		}}
	}
	return nil
}

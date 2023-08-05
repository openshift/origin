package legacynodeinvariants

import (
	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/invariantlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func testMarketplaceStartupProbeFailure(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] openshift-marketplace pods should not get excessive startupProbe failures"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events, duplicateevents.MarketplaceStartupProbeFailureRegExpStr, duplicateevents.DuplicateEventThreshold)
}

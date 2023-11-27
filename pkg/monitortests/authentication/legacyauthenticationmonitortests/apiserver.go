package legacyauthenticationmonitortests

import (
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func testOauthApiserverProbeErrorLiveness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on liveness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, "openshift-oauth-apiserver", pathologicaleventlibrary.ProbeErrorLiveness, pathologicaleventlibrary.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorReadiness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, "openshift-oauth-apiserver", pathologicaleventlibrary.ProbeErrorTimeoutAwaitingHeaders, pathologicaleventlibrary.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorConnectionRefused(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiness probe due to connection refused"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, "openshift-oauth-apiserver",
		pathologicaleventlibrary.ProbeErrorConnectionRefused, pathologicaleventlibrary.DuplicateEventThreshold)
}

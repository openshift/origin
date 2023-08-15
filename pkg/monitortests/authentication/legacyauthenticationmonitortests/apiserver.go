package legacyauthenticationmonitortests

import (
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func testOauthApiserverProbeErrorLiveness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on liveiness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, pathologicaleventlibrary.ProbeErrorLivenessMessageRegExpStr, "openshift-oauth-apiserver", pathologicaleventlibrary.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorReadiness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiiness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, pathologicaleventlibrary.ProbeErrorReadinessMessageRegExpStr, "openshift-oauth-apiserver", pathologicaleventlibrary.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorConnectionRefused(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiiness probe due to connection refused"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, pathologicaleventlibrary.ProbeErrorConnectionRefusedRegExpStr, "openshift-oauth-apiserver", pathologicaleventlibrary.DuplicateEventThreshold)
}

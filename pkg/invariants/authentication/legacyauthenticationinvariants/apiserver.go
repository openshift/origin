package legacyauthenticationinvariants

import (
	"github.com/openshift/origin/pkg/invariantlibrary/pathologicaleventlibrary"

	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func testOauthApiserverProbeErrorLiveness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on liveiness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, duplicateevents.ProbeErrorLivenessMessageRegExpStr, "openshift-oauth-apiserver", duplicateevents.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorReadiness(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiiness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, duplicateevents.ProbeErrorReadinessMessageRegExpStr, "openshift-oauth-apiserver", duplicateevents.DuplicateEventThreshold)
}

func testOauthApiserverProbeErrorConnectionRefused(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-apiserver-auth] openshift-oauth-apiserver should not get probe error on readiiness probe due to connection refused"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, duplicateevents.ProbeErrorConnectionRefusedRegExpStr, "openshift-oauth-apiserver", duplicateevents.DuplicateEventThreshold)
}

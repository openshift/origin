package riskanalysis

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Run this test case with the env var set appropriately and sippy's cache disabled to benchmark RA.
func TestRequestRiskAnalysis(t *testing.T) {
	const raApiUrl = "api/jobs/runs/risk_analysis"
	const timesToRunRiskAnalysis = 50
	sippyUrl := os.Getenv("TEST_RA_SIPPY_URL")
	if sippyUrl == "" {
		// not intended to run in CI, human should set env var to run manually
		t.Skip("set TEST_RA_SIPPY_URL to run this test; e.g. http://localhost:8080/ or https://sippy.dptools.openshift.org/")
	}
	// The JSON here can come from any job run that builds a RA summary in the junit directory.
	// Typically you'll want one with some test failures to exercise the RA code.
	// From the job artifacts, descend to the junit directory which should be under:
	//    artifacts/*e2e*/*e2e*/artifacts/junit/
	// ... and look for the test-failures-summary_*.json files. You can combine them by picking the one with
	// more test failures, and changing TestCount to the one with a higher TestCount (otherwise RA bails early).
	//
	// You can also download the junit directory and simply run the actual command that runs in tests, e.g.:
	//    openshift-tests risk-analysis --sippy-url http://localhost:8080/api/jobs/runs/risk_analysis --junit-dir ./junit/
	// Of course, this creates a lot of chaff about disruption that obscures RA output. And it only runs once.
	const summaryJson = `
		{
			    "ID": 2041023444769837056,
				"ProwJob": {
					"Name": "periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-ovn-single-node-serial"
				},
				"ClusterData": {
					"Release": "4.22",
					"FromRelease": "",
					"Platform": "aws",
					"Architecture": "amd64",
					"Network": "ovn",
					"Topology": "single",
					"os": {
						"Default": "",
						"ControlPlaneMachineConfigPool": "",
						"WorkerMachineConfigPool": ""
					},
					"NetworkStack": "IPv4",
					"CloudRegion": "us-east-1",
					"CloudZone": "us-east-1d",
					"ClusterVersionHistory": [
						"4.22.0-0.nightly-2026-04-06-051707"
					],
					"MasterNodesUpdated": "Y"
				},
				"Tests": [
					{
						"Test": {
							"Name": "[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io] Ensure gateway loadbalancer service and dnsrecords could be deleted and then get recreated [Serial] [Suite:openshift/conformance/serial]"
						},
						"Suite": {
							"Name": "openshift-tests"
						},
						"Status": 12
					}
				],
				"TestCount": 2149
		}
    `

	client := &http.Client{}
	tmp := t.TempDir()
	opt := &Options{SippyURL: sippyUrl + raApiUrl, JUnitDir: tmp}
	testStart := time.Now()
	for times := 0; times < timesToRunRiskAnalysis; times++ {
		_, err := opt.requestRiskAnalysis([]byte(summaryJson), client, &mockSleeper{})
		assert.NoError(t, err)
	}
	fmt.Printf("Test took %v\n", time.Since(testStart))
}

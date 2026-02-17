package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/cpuset"
)

var rtTestThresholds = map[string]int{
	"deadline_test": 100,
	"oslat":         100,
	"cyclictest":    100,
	"hwlatdetect":   100,
}

func runPiStressFifo(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1"}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running pi_stress with the standard algorithm")
	}

	writeTestArtifacts("pi_stress_standard_results.log", res)

	return nil
}

func runPiStressRR(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1", "--rr"}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running pi_stress with the round-robin algorithm")
	}

	writeTestArtifacts("pi_stress_rr_results.log", res)

	return nil
}

func runDeadlineTest(oc *exutil.CLI) error {
	testName := "deadline_test"

	args := []string{rtPodName, "--",
		testName,
		"-i", fmt.Sprintf("%d", rtTestThresholds[testName]),
	}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running deadline_test")
	}

	writeTestArtifacts("deadlinetest_results.log", res)

	return nil
}

func runHwlatdetect(oc *exutil.CLI) error {
	testName := "hwlatdetect"
	args := []string{rtPodName, "--", testName, "--duration=600s", "--window=1s", "--width=500ms", "--debug", fmt.Sprintf("--threshold=%dus", rtTestThresholds[testName])}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running hwlatdetect")
	}

	writeTestArtifacts("hwlatdetect_results.log", res)

	return nil
}

func runOslat(cpuCount int, oc *exutil.CLI) error {
	testName := "oslat"
	oslatReportFile := "/tmp/oslatresults.json"

	// Make sure there is enough hardware for this test
	if cpuCount <= 4 {
		return fmt.Errorf("more than 4 cores are required to run this oslat test. Found %d cores", cpuCount)
	}

	reservedCores, err := getReservedCores(oc)
	if err != nil {
		return errors.Wrap(err, "unable to get the reserved core configuration")
	}

	// Run the test
	args := []string{rtPodName, "--",
		testName,
		"--cpu-list", fmt.Sprintf("%d-%d", reservedCores+1, cpuCount-1),
		"--cpu-main-thread", fmt.Sprint(reservedCores + 1),
		"--rtprio", "1",
		"--duration", "600",
		"--json", oslatReportFile}
	_, err = oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return errors.Wrap(err, "error running oslat")
	}

	// Get the results
	args = []string{rtPodName, "--", "cat", oslatReportFile}
	report, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return errors.Wrap(err, "error retrieving oslat results")
	}

	writeTestArtifacts("oslat_results.json", report)

	// Parse the results and return any errors detected
	if err = parseOslatResults(report, rtTestThresholds[testName]); err != nil {
		return errors.Wrap(err, "error parsing oslat report")
	}

	return nil
}

func parseOslatResults(jsonReport string, maxThresholdusec int) error {
	var oslatReport struct {
		Threads map[string]struct {
			Cpu int `json:"cpu"`
			Max int `json:"max"`
		} `json:"thread"`
	}

	// Parse the data
	err := json.Unmarshal([]byte(jsonReport), &oslatReport)
	if err != nil {
		return errors.Wrap(err, "unable to decode oslat report json")
	}

	if len(oslatReport.Threads) == 0 {
		return fmt.Errorf("no thread reports found")
	}

	failedCPUs := make([]int, 0, len(oslatReport.Threads)) // Report all failed cores
	for _, thread := range oslatReport.Threads {
		if thread.Max > maxThresholdusec {
			failedCPUs = append(failedCPUs, thread.Cpu)
		}
	}

	if len(failedCPUs) > 0 {
		return fmt.Errorf("the following CPUs were over the max latency threshold: %v", failedCPUs)
	}

	return nil
}

func runCyclictest(cpuCount int, oc *exutil.CLI) error {
	testName := "cyclictest"
	cyclictestReportFile := "/tmp/cyclictestresults.json"
	// Make sure there is enough hardware for this test
	if cpuCount <= 4 {
		return fmt.Errorf("more than 4 cores are required to run this oslat test. Found %d cores", cpuCount)
	}

	// Run the test
	args := []string{rtPodName, "--", testName, "--duration=10m", "--priority=95", fmt.Sprintf("--threads=%d", cpuCount-5), fmt.Sprintf("--affinity=4-%d", cpuCount-1), "--interval=1000", fmt.Sprintf("--breaktrace=%d", rtTestThresholds[testName]), "--mainaffinity=4", "-m", fmt.Sprintf("--json=%s", cyclictestReportFile)}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return errors.Wrap(err, "error running cyclictest")
	}

	// Gather the results
	args = []string{rtPodName, "--", "cat", cyclictestReportFile}
	report, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return errors.Wrap(err, "error retrieving cyclictest results")
	}

	writeTestArtifacts("cyclictest_results.json", report)

	// Parse the results and return any errors detected
	if err = parseCyclictestResults(report, rtTestThresholds[testName]); err != nil {
		return errors.Wrap(err, "error parsing cyclictest report")
	}

	return nil
}

func parseCyclictestResults(jsonReport string, maxThresholdusec int) error {
	var cyclictestReport struct {
		Threads map[string]struct {
			Cpu int `json:"cpu"`
			Max int `json:"max"`
		} `json:"thread"`
	}

	// Parse the data
	err := json.Unmarshal([]byte(jsonReport), &cyclictestReport)
	if err != nil {
		return errors.Wrap(err, "unable to decode cyclictest report json")
	}

	if len(cyclictestReport.Threads) == 0 {
		return fmt.Errorf("no thread reports found")
	}

	failedCPUs := make([]int, 0, len(cyclictestReport.Threads)) // Report all failed cores
	for _, thread := range cyclictestReport.Threads {
		if thread.Max > maxThresholdusec {
			failedCPUs = append(failedCPUs, thread.Cpu)
		}
	}

	if len(failedCPUs) > 0 {
		return fmt.Errorf("the following CPUs were over the max latency threshold: %v", failedCPUs)
	}

	return nil
}

func getProcessorCount(oc *exutil.CLI) (int, error) {
	args := []string{rtPodName, "--", "getconf", "_NPROCESSORS_ONLN"}
	num, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return 0, err
	}

	// Parse out the CPU count
	count, err := strconv.Atoi(num)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// getReservedCores will parse the performance profile (if it exists) and look for the reserved cpu configuration
// The expected configurations should start with core 0 and be in the form 0-X
func getReservedCores(oc *exutil.CLI) (int, error) {
	// Performance profiles dictate constrained CPUSets, we gather them here to compare.
	// Note: We're using a dynamic client here to avoid importing the PerformanceProfile
	// for a simple query and keep this code change small. If we end up needing more interaction
	// with PerformanceProfiles, then we should import the package and update this call.

	performanceProfiles, err := oc.AdminDynamicClient().
		Resource(schema.GroupVersionResource{
			Resource: "performanceprofiles",
			Group:    "performance.openshift.io",
			Version:  "v2"}).Namespace("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("error listing performance profiles: %w", err)
	}

	if len(performanceProfiles.Items) == 0 {
		e2e.Logf("no performance profile detected, using 0-3 as reserved cores")

		return 3, nil
	}

	if len(performanceProfiles.Items) > 1 {
		return 0, fmt.Errorf("unable to determine reserved cores, more than 1 performance profile was found")
	}

	pprof := performanceProfiles.Items[0]
	reservedCPU, found, err := unstructured.NestedString(pprof.Object, "spec", "cpu", "reserved")
	if err != nil {
		return 0, fmt.Errorf("error getting reservedCPUSet from PerformanceProfile: %w", err)
	}
	if !found {
		return 0, fmt.Errorf("expected spec.reserved to be found in PerformanceProfile(%s)", pprof.GetName())
	}

	reservedCPUset, err := cpuset.Parse(reservedCPU)
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse the reserved cpuset")
	}

	reservedCPUs := strings.Split(reservedCPUset.String(), "-")
	if len(reservedCPUs) != 2 {
		return 0, fmt.Errorf("abnormal reserved cpu configuration detected. Please use the form '0-X'")
	}

	reservedEnd, err := strconv.Atoi(reservedCPUs[1])
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse the end of the reserved cpu block")
	}

	return reservedEnd, nil
}

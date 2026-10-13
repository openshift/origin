package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/cpuset"
)

type rtTestResult string

const (
	rtResultPass rtTestResult = "PASS"
	rtResultWarn rtTestResult = "WARN"
	rtResultFail rtTestResult = "FAIL"
)

type rtThresholdConfig struct {
	SoftThreshold int // Expected max — warn if exceeded
	HardThreshold int // Absolute max — fail if exceeded
}

type cpuLatencyResult struct {
	CPU        int `json:"cpu"`
	MaxLatency int `json:"max_latency_usec"`
}

type rtLatencyAnalysis struct {
	TestName        string             `json:"test_name"`
	SoftThreshold   int                `json:"soft_threshold_usec"`
	HardThreshold   int                `json:"hard_threshold_usec"`
	TotalCPUs       int                `json:"total_cpus"`
	MaxLatency      int                `json:"max_latency_usec"`
	AvgMaxLatency   float64            `json:"avg_max_latency_usec"`
	P99MaxLatency   int                `json:"p99_max_latency_usec"`
	CPUsOverSoft    int                `json:"cpus_over_soft_threshold"`
	CPUsOverHard    int                `json:"cpus_over_hard_threshold"`
	PercentOverSoft float64            `json:"percent_over_soft_threshold"`
	FailedCPUs      []cpuLatencyResult `json:"failed_cpus,omitempty"`
	Result          rtTestResult       `json:"result"`
	FailureReason   string             `json:"failure_reason,omitempty"`
}

const maxSoftThresholdViolationPercent = 10.0

var rtTestThresholds = map[string]rtThresholdConfig{
	"deadline_test": {SoftThreshold: 100, HardThreshold: 500},
	"oslat":         {SoftThreshold: 150, HardThreshold: 500},
	"cyclictest":    {SoftThreshold: 150, HardThreshold: 500},
	"hwlatdetect":   {SoftThreshold: 100, HardThreshold: 500},
}

func runPiStressFifo(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1"}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running pi_stress with the standard algorithm")
	}

	writeTestArtifacts(fmt.Sprintf("%s_%s.log", "pi_stress_standard", e2e.TimeNow().Format(time.RFC3339)), res)

	return nil
}

func runPiStressRR(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1", "--rr"}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running pi_stress with the round-robin algorithm")
	}

	writeTestArtifacts(fmt.Sprintf("%s_%s.log", "pi_stress_rr", e2e.TimeNow().Format(time.RFC3339)), res)

	return nil
}

func runDeadlineTest(oc *exutil.CLI) error {
	testName := "deadline_test"

	args := []string{rtPodName, "--",
		testName,
		"-i", fmt.Sprintf("%d", rtTestThresholds[testName].HardThreshold),
	}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running deadline_test")
	}

	writeTestArtifacts(fmt.Sprintf("%s_%s.log", testName, e2e.TimeNow().Format(time.RFC3339)), res)

	return nil
}

func runHwlatdetect(oc *exutil.CLI) error {
	testName := "hwlatdetect"
	args := []string{rtPodName, "--", testName, "--duration=600s", "--window=1s", "--width=500ms", "--debug", fmt.Sprintf("--threshold=%dus", rtTestThresholds[testName].HardThreshold)}
	res, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running hwlatdetect")
	}

	writeTestArtifacts(fmt.Sprintf("%s_%s.log", testName, e2e.TimeNow().Format(time.RFC3339)), res)

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

	writeTestArtifacts(fmt.Sprintf("%s_%s.json", testName, e2e.TimeNow().Format(time.RFC3339)), report)

	// Parse the results and evaluate against thresholds
	analysis, err := parseLatencyResults(testName, report, rtTestThresholds[testName])
	if err != nil {
		return errors.Wrap(err, "error parsing oslat report")
	}
	writeAnalysisArtifact(testName, analysis)
	if analysis.Result == rtResultWarn {
		e2e.Logf("WARNING: %s - %s", testName, analysis.FailureReason)
	}
	if analysis.Result == rtResultFail {
		return fmt.Errorf("%s failed: %s", testName, analysis.FailureReason)
	}

	return nil
}

func runCyclictest(cpuCount int, oc *exutil.CLI) error {
	testName := "cyclictest"
	cyclictestReportFile := "/tmp/cyclictestresults.json"
	// Make sure there is enough hardware for this test
	if cpuCount <= 4 {
		return fmt.Errorf("more than 4 cores are required to run this cyclictest test. Found %d cores", cpuCount)
	}

	// Run the test
	args := []string{rtPodName, "--", testName, "--duration=10m", "--priority=95", fmt.Sprintf("--threads=%d", cpuCount-5), fmt.Sprintf("--affinity=4-%d", cpuCount-1), "--interval=1000", fmt.Sprintf("--breaktrace=%d", rtTestThresholds[testName].HardThreshold), "--mainaffinity=4", "-m", fmt.Sprintf("--json=%s", cyclictestReportFile)}
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

	writeTestArtifacts(fmt.Sprintf("%s_%s.json", testName, e2e.TimeNow().Format(time.RFC3339)), report)

	// Parse the results and evaluate against thresholds
	analysis, err := parseLatencyResults(testName, report, rtTestThresholds[testName])
	if err != nil {
		return errors.Wrap(err, "error parsing cyclictest report")
	}
	writeAnalysisArtifact(testName, analysis)
	if analysis.Result == rtResultWarn {
		e2e.Logf("WARNING: %s - %s", testName, analysis.FailureReason)
	}
	if analysis.Result == rtResultFail {
		return fmt.Errorf("%s failed: %s", testName, analysis.FailureReason)
	}

	return nil
}

func parseLatencyResults(testName string, jsonReport string, thresholds rtThresholdConfig) (*rtLatencyAnalysis, error) {
	var report struct {
		Threads map[string]struct {
			Cpu int `json:"cpu"`
			Max int `json:"max"`
		} `json:"thread"`
	}

	if err := json.Unmarshal([]byte(jsonReport), &report); err != nil {
		return nil, fmt.Errorf("unable to decode %s report json: %w", testName, err)
	}

	if len(report.Threads) == 0 {
		return nil, fmt.Errorf("no thread reports found in %s results", testName)
	}

	analysis := &rtLatencyAnalysis{
		TestName:      testName,
		SoftThreshold: thresholds.SoftThreshold,
		HardThreshold: thresholds.HardThreshold,
		TotalCPUs:     len(report.Threads),
	}

	maxValues := make([]int, 0, len(report.Threads))
	var sum int
	for _, thread := range report.Threads {
		maxValues = append(maxValues, thread.Max)
		sum += thread.Max

		if thread.Max > analysis.MaxLatency {
			analysis.MaxLatency = thread.Max
		}
		if thread.Max > thresholds.SoftThreshold {
			analysis.CPUsOverSoft++
			analysis.FailedCPUs = append(analysis.FailedCPUs, cpuLatencyResult{
				CPU:        thread.Cpu,
				MaxLatency: thread.Max,
			})
		}
		if thread.Max > thresholds.HardThreshold {
			analysis.CPUsOverHard++
		}
	}

	analysis.AvgMaxLatency = float64(sum) / float64(analysis.TotalCPUs)
	analysis.PercentOverSoft = (float64(analysis.CPUsOverSoft) / float64(analysis.TotalCPUs)) * 100

	// Compute P99
	sort.Ints(maxValues)
	p99Index := int(math.Ceil(0.99*float64(len(maxValues)))) - 1
	if p99Index < 0 {
		p99Index = 0
	}
	analysis.P99MaxLatency = maxValues[p99Index]

	// Determine result
	switch {
	case analysis.CPUsOverHard > 0:
		analysis.Result = rtResultFail
		analysis.FailureReason = fmt.Sprintf("%d/%d CPUs exceeded hard threshold of %dus (max observed: %dus)",
			analysis.CPUsOverHard, analysis.TotalCPUs, thresholds.HardThreshold, analysis.MaxLatency)
	case analysis.PercentOverSoft > maxSoftThresholdViolationPercent:
		analysis.Result = rtResultFail
		analysis.FailureReason = fmt.Sprintf("%d/%d CPUs (%.1f%%) exceeded soft threshold of %dus — systemic latency issue",
			analysis.CPUsOverSoft, analysis.TotalCPUs, analysis.PercentOverSoft, thresholds.SoftThreshold)
	case analysis.CPUsOverSoft > 0:
		analysis.Result = rtResultWarn
		analysis.FailureReason = fmt.Sprintf("%d/%d CPUs (%.1f%%) exceeded soft threshold of %dus (max observed: %dus) — isolated spikes within tolerance",
			analysis.CPUsOverSoft, analysis.TotalCPUs, analysis.PercentOverSoft, thresholds.SoftThreshold, analysis.MaxLatency)
	default:
		analysis.Result = rtResultPass
	}

	return analysis, nil
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

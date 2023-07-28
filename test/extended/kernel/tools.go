package kernel

import (
	"encoding/json"
	"fmt"
	"strconv"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
)

const (
	hwlatdetectThresholdusec = 5000
	oslatThresholdusec       = 5000
	cyclictestThresholdusec  = 5000
)

func runPiStressFifo(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1"}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()

	return err
}

func runPiStressRR(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1", "--rr"}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()

	return err
}

func runDeadlineTest(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "deadline_test"}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()

	return err
}

func runHwlatdetect(oc *exutil.CLI) error {
	args := []string{rtPodName, "--", "hwlatdetect", "--duration=600s", "--window=1s", "--width=500ms", fmt.Sprintf("--threshold=%dus", hwlatdetectThresholdusec)}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		// An error here indicates thresholds were exceeded or an issue with the test
		return errors.Wrap(err, "error running hwlatdetect")
	}

	return nil
}

func runOslat(cpuCount int, oc *exutil.CLI) error {
	oslatReportFile := "/tmp/oslatresults.json"

	// Make sure there is enough hardware for this test
	if cpuCount <= 4 {
		return fmt.Errorf("more than 4 cores are required to run this oslat test. Found %d cores", cpuCount)
	}

	// Run the test
	args := []string{rtPodName, "--", "oslat", "--cpu-list", fmt.Sprintf("4-%d", cpuCount-1), "--rtprio", "1", "--duration", "600", "--json", oslatReportFile}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return errors.Wrap(err, "error running oslat")
	}

	// Get the results
	args = []string{rtPodName, "--", "cat", oslatReportFile}
	report, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	if err != nil {
		return errors.Wrap(err, "error retrieving oslat results")
	}

	// Parse the results and return any errors detected
	if err = parseOslatResults(report, oslatThresholdusec); err != nil {
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
	cyclictestReportFile := "/tmp/cyclictestresults.json"
	// Make sure there is enough hardware for this test
	if cpuCount <= 4 {
		return fmt.Errorf("more than 4 cores are required to run this oslat test. Found %d cores", cpuCount)
	}

	// Run the test
	args := []string{rtPodName, "--", "cyclictest", "--duration=10m", "--priority=95", fmt.Sprintf("--threads=%d", cpuCount-5), fmt.Sprintf("--affinity=4-%d", cpuCount-1), "--interval=1000", fmt.Sprintf("--breaktrace=%d", cyclictestThresholdusec), "--mainaffinity=4", "-m", fmt.Sprintf("--json=%s", cyclictestReportFile)}
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

	// Parse the results and return any errors detected
	if err = parseCyclictestResults(report, cyclictestThresholdusec); err != nil {
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

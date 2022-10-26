package riskanalysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Options is used to run a risk analysis to determine how severe or unusual
// the test failures in an openshift-tests run were.
type Options struct {
	Out, ErrOut io.Writer
	JUnitDir    string
}

const testFailureSummaryFilePrefix = "test-failures-summary"

// Run performs the test risk analysis by reading the output files from the test run, submitting them to sippy,
// and writing out the analysis result as a new artifact.
func (opt *Options) Run() error {
	fmt.Fprintf(opt.Out, "Scanning for %s files in: %s\n", testFailureSummaryFilePrefix, opt.JUnitDir)

	resultFiles, err := filepath.Glob(fmt.Sprintf("%s/%s*.json", opt.JUnitDir, testFailureSummaryFilePrefix))
	if err != nil {
		return err
	}
	fmt.Fprintf(opt.Out, "Found files: %v\n", resultFiles)

	prowJobRuns := []*ProwJobRun{}
	// Read each result file into a ProwJobRun struct:
	for _, rf := range resultFiles {
		data, err := os.ReadFile(rf)
		if err != nil {
			return err
		}
		jobRun := &ProwJobRun{}
		err = json.Unmarshal(data, jobRun)
		if err != nil {
			return errors.Wrapf(err, "error unmarshalling ProwJob json")
		}
		prowJobRuns = append(prowJobRuns, jobRun)
	}

	// We will often have more than one output file for this job run because openshift-tests is often
	// invoked multiple times (pre/post upgrade). We need to merge the data together in this case.
	var finalProwJobRun *ProwJobRun
	for _, pjr := range prowJobRuns {
		if finalProwJobRun == nil {
			finalProwJobRun = pjr
			continue
		}
		if pjr.ProwJob.Name != finalProwJobRun.ProwJob.Name {
			return fmt.Errorf("mismatched job names found in %s files, %s != %s",
				testFailureSummaryFilePrefix, finalProwJobRun.ProwJob.Name, pjr.ProwJob.Name)
		}
		finalProwJobRun.Tests = append(finalProwJobRun.Tests, pjr.Tests...)
	}

	// TODO: query sippy
	url := "http://localhost:8080/api/jobs/runs/risk_analysis"
	inputBytes, err := json.Marshal(finalProwJobRun)
	if err != nil {
		return errors.Wrap(err, "error marshalling results")
	}

	req, err := http.NewRequest("GET", url, bytes.NewBuffer(inputBytes))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "error requesting risk analysis from sippy")
	}
	defer resp.Body.Close()

	riskAnalysisBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "error reading risk analysis request body from sippy")
	}
	fmt.Println("response Body:", string(riskAnalysisBytes))

	outputFile := filepath.Join(opt.JUnitDir, "risk-analysis.json")
	err = ioutil.WriteFile(outputFile, riskAnalysisBytes, 0644)
	if err != nil {
		return errors.Wrap(err, "error writing risk analysis json artifact")
	}
	fmt.Fprintf(opt.Out, "Successfully wrote: %s\n", outputFile)

	return nil
}

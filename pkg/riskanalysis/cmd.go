package riskanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/openshift/origin/test/extended/testdata"
	"github.com/pkg/errors"
)

const (
	maxRetries = 3
)

// Options is used to run a risk analysis to determine how severe or unusual
// the test failures in an openshift-tests run were.
type Options struct {
	Out, ErrOut io.Writer
	JUnitDir    string
	SippyURL    string
}

const testFailureSummaryFilePrefix = "test-failures-summary"
const sippyURL = "https://sippy.dptools.openshift.org/sippy-ng/"

// Run performs the test risk analysis by reading the output files from the test run, submitting them to sippy,
// and writing out the analysis result as a new artifact.
func (opt *Options) Run() error {
	fmt.Fprintf(opt.Out, "Scanning for %s files in: %s\n", testFailureSummaryFilePrefix, opt.JUnitDir)

	resultFiles, err := filepath.Glob(fmt.Sprintf("%s/%s*.json", opt.JUnitDir, testFailureSummaryFilePrefix))
	if err != nil {
		fmt.Fprintf(opt.Out, "Error scanning for test failure summary files: %v", err)
		return nil
	}
	fmt.Fprintf(opt.Out, "Found files: %v\n", resultFiles)

	// we didn't find any files to process. log but don't return an error as  step may not have produced those files
	if len(resultFiles) == 0 {
		fmt.Fprintf(opt.Out, "Missing : %s file(s), exiting\n", testFailureSummaryFilePrefix)
		return nil
	}

	prowJobRuns := []*ProwJobRun{}
	// Read each result file into a ProwJobRun struct:
	for _, rf := range resultFiles {
		data, err := os.ReadFile(rf)
		if err != nil {
			fmt.Fprintf(opt.Out, "Error reading test failure summary file: %s - %v", rf, err)
			return nil
		}
		jobRun := &ProwJobRun{}
		err = json.Unmarshal(data, jobRun)
		if err != nil {
			fmt.Fprintf(opt.Out, "Error unmarshalling ProwJob json for: %s - %v", rf, err)
			return nil
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
			fmt.Fprintf(opt.Out, "Mismatched job names found in %s files, %s != %s",
				testFailureSummaryFilePrefix, finalProwJobRun.ProwJob.Name, pjr.ProwJob.Name)
			return nil
		}
		finalProwJobRun.Tests = append(finalProwJobRun.Tests, pjr.Tests...)
		finalProwJobRun.TestCount += pjr.TestCount
	}

	inputBytes, err := json.Marshal(finalProwJobRun)
	if err != nil {
		fmt.Fprintf(opt.Out, "Error marshalling results: %v", err)
		return nil
	}

	req, err := http.NewRequest("GET", opt.SippyURL, bytes.NewBuffer(inputBytes))
	if err != nil {
		fmt.Fprintf(opt.Out, "Error creating GET request during risk analysis: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	var resp *http.Response
	clientDoSuccess := false
	for i := 1; i <= maxRetries; i++ {
		ctx, cancelFn := context.WithTimeout(req.Context(), 20*time.Second)
		defer cancelFn()
		startTime := time.Now()
		fmt.Printf("%s: Requesting risk analysis (attempt %d/%d) from: %s\n", startTime.Format(time.RFC3339), i, maxRetries, sippyURL)
		resp, err = client.Do(req.WithContext(ctx))
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		fmt.Printf("%s: Call to sippy finished after: %s\n", endTime.Format(time.RFC3339), duration)
		if err == nil {
			clientDoSuccess = true
			break
		}
		fmt.Println(errors.Wrap(err, "error requesting risk analysis from sippy, sleeping 30s"))

		// cancel the context we just used.
		cancelFn()
		time.Sleep(time.Duration(i*30) * time.Second)
	}
	if !clientDoSuccess {
		fmt.Fprintf(opt.Out, "Unable to obtain risk analysis from sippy after retries: %v", err)
		return nil
	}
	defer resp.Body.Close()

	riskAnalysisBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(opt.Out, "Error reading risk analysis request body from sippy: %v", err)
		return nil
	}
	fmt.Println("response Body:", string(riskAnalysisBytes))

	outputFile := filepath.Join(opt.JUnitDir, "risk-analysis.json")
	err = ioutil.WriteFile(outputFile, riskAnalysisBytes, 0644)
	if err != nil {
		fmt.Fprintf(opt.Out, "Error writing risk analysis json artifact: %v", err)
		return nil
	}
	fmt.Fprintf(opt.Out, "Successfully wrote: %s\n", outputFile)

	// Write html file for spyglass
	riskAnalysisHTMLTemplate := testdata.MustAsset("e2echart/test-risk-analysis.html")
	html := bytes.ReplaceAll(riskAnalysisHTMLTemplate, []byte("TEST_RISK_ANALYSIS_SIPPY_URL_GOES_HERE"), []byte(sippyURL))
	html = bytes.ReplaceAll(html, []byte("TEST_RISK_ANALYSIS_JSON_GOES_HERE"), riskAnalysisBytes)
	path := filepath.Join(opt.JUnitDir, fmt.Sprintf("%s.html", "test-risk-analysis"))
	if err := ioutil.WriteFile(path, html, 0644); err != nil {
		fmt.Fprintf(opt.Out, "Error writing output file: %v", err)
		return nil
	}

	return nil
}

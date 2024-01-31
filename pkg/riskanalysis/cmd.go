package riskanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/allowedbackenddisruption"
	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionserializer"
	"github.com/openshift/origin/test/extended/testdata"
	"github.com/sirupsen/logrus"
)

const (
	maxRetries = 3
)

// Options is used to run a risk analysis to determine how severe or unusual
// the test failures in an openshift-tests run were.
type Options struct {
	JUnitDir string
	SippyURL string
}

const testFailureSummaryFilePrefix = "test-failures-summary"
const sippyURL = "https://sippy.dptools.openshift.org/sippy-ng/"

// Run performs the test risk analysis by reading the output files from the test run, submitting them to sippy,
// and writing out the analysis result as a new artifact.
func (opt *Options) Run() error {
	logrus.Infof("Scanning for %s files in: %s", testFailureSummaryFilePrefix, opt.JUnitDir)

	resultFiles, err := filepath.Glob(fmt.Sprintf("%s/%s*.json", opt.JUnitDir, testFailureSummaryFilePrefix))
	if err != nil {
		logrus.Infof("Error scanning for test failure summary files: %v", err)
		return nil
	}
	logrus.Infof("Found files: %v", resultFiles)

	// we didn't find any files to process. log but don't return an error as  step may not have produced those files
	if len(resultFiles) == 0 {
		logrus.Infof("Missing : %s file(s), exiting", testFailureSummaryFilePrefix)
		return nil
	}

	prowJobRuns := []*ProwJobRun{}
	// Read each result file into a ProwJobRun struct:
	for _, rf := range resultFiles {
		data, err := os.ReadFile(rf)
		if err != nil {
			logrus.Infof("Error reading test failure summary file: %s - %v", rf, err)
			return nil
		}
		jobRun := &ProwJobRun{}
		err = json.Unmarshal(data, jobRun)
		if err != nil {
			logrus.Infof("Error unmarshalling ProwJob json for: %s - %v", rf, err)
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
			logrus.Errorf("Mismatched job names found in %s files, %s != %s",
				testFailureSummaryFilePrefix, finalProwJobRun.ProwJob.Name, pjr.ProwJob.Name)
			return nil
		}
		finalProwJobRun.Tests = append(finalProwJobRun.Tests, pjr.Tests...)
		finalProwJobRun.TestCount += pjr.TestCount
	}

	inputBytes, err := json.Marshal(finalProwJobRun)
	if err != nil {
		logrus.WithError(err).Error("Error marshalling results")
		return nil
	}

	req, err := http.NewRequest("GET", opt.SippyURL, bytes.NewBuffer(inputBytes))
	if err != nil {
		logrus.WithError(err).Error("Error creating GET request during risk analysis")
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
		logrus.Infof("Requesting risk analysis (attempt %d/%d) from: %s", i, maxRetries, sippyURL)
		resp, err = client.Do(req.WithContext(ctx))
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		logrus.Infof("Call to sippy finished after: %s", duration)
		if err == nil {
			clientDoSuccess = true
			break
		}
		logrus.WithError(err).Warn("error requesting risk analysis from sippy, sleeping 30s")

		// cancel the context we just used.
		cancelFn()
		time.Sleep(time.Duration(i*30) * time.Second)
	}
	if !clientDoSuccess {
		logrus.WithError(err).Error("Unable to obtain risk analysis from sippy after retries")
		return nil
	}
	defer resp.Body.Close()

	riskAnalysisBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Error("Error reading risk analysis request body from sippy")
		return nil
	}
	logrus.Info("response Body:", string(riskAnalysisBytes))

	outputFile := filepath.Join(opt.JUnitDir, "risk-analysis.json")
	err = ioutil.WriteFile(outputFile, riskAnalysisBytes, 0644)
	if err != nil {
		logrus.WithError(err).Error("Error writing risk analysis json artifact")
		return nil
	}
	logrus.Infof("Successfully wrote: %s", outputFile)

	disruptionBytes := []byte(`{Backends: []}`)
	da, err := runDisruptionAnalysis(opt, finalProwJobRun.ClusterData.JobType)
	if err != nil {
		logrus.WithError(err).Error("error running disruption analysis locally")
		return nil
	}
	disruptionBytes, err = json.Marshal(da)
	if err != nil {
		logrus.WithError(err).Error("Error marshalling disruption results")
		return nil
	}

	// Write html file for spyglass
	riskAnalysisHTMLTemplate := testdata.MustAsset("e2echart/test-risk-analysis.html")
	html := bytes.ReplaceAll(riskAnalysisHTMLTemplate, []byte("TEST_RISK_ANALYSIS_SIPPY_URL_GOES_HERE"), []byte(sippyURL))
	html = bytes.ReplaceAll(html, []byte("TEST_RISK_ANALYSIS_JSON_GOES_HERE"), riskAnalysisBytes)
	html = bytes.ReplaceAll(html, []byte("TEST_DISRUPTION_ANALYSIS_JSON_GOES_HERE"), disruptionBytes)
	path := filepath.Join(opt.JUnitDir, fmt.Sprintf("%s.html", "test-risk-analysis"))
	if err := ioutil.WriteFile(path, html, 0644); err != nil {
		logrus.WithError(err).Error("Error writing output file")
		return nil
	}

	return nil
}

type disruptionBackendAnalysis struct {
	BackendName        string
	ObservedDisruption int
	P50                float64
	P75                float64
	P95                float64
	P99                float64
	JobRuns            int64
	RiskColor          string // red, yellow or green
}

type disruptionAnalysis struct {
	Backends []disruptionBackendAnalysis
}

func runDisruptionAnalysis(opt *Options, jobType platformidentification.JobType) (*disruptionAnalysis, error) {
	logrus.WithField("jobType", jobType).Infof("Checking disruption results for job type")
	resultFiles, err := filepath.Glob(fmt.Sprintf("%s/backend-disruption*.json", opt.JUnitDir))
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found files: %v", resultFiles)

	// If we have multiple files we need to combine the disruption results into a single value for the
	// overall job run, as we do when we submit to the database.
	analysis := &disruptionAnalysis{}
	for _, filename := range resultFiles {

		var disruptList *disruptionserializer.BackendDisruptionList
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		byteValue, _ := ioutil.ReadAll(f)
		err = json.Unmarshal(byteValue, &disruptList)
		if err != nil {
			return nil, err
		}

		for _, backend := range disruptList.BackendDisruptions {
			tallyBackendInAnalysis(analysis, backend)
		}
	}

	matcher := allowedbackenddisruption.GetCurrentResults()
	for i, ba := range analysis.Backends {
		// Inject the percentiles:
		percentiles, details, err := matcher.BestMatchDuration(ba.BackendName, jobType, 1)
		if err != nil {
			logrus.WithError(err).Error("error looking up historical duration")
		}
		if percentiles == (historicaldata.StatisticalDuration{}) {
			logrus.WithField("details", details).Warn("no historical data found for job run: ")
			continue
		}
		analysis.Backends[i].P50 = percentiles.P50.Seconds()
		analysis.Backends[i].P75 = percentiles.P75.Seconds()
		analysis.Backends[i].P95 = percentiles.P95.Seconds()
		analysis.Backends[i].P99 = percentiles.P99.Seconds()
		analysis.Backends[i].JobRuns = percentiles.JobRuns

		analysis.Backends[i].RiskColor = "lightgreen"
		if float64(analysis.Backends[i].ObservedDisruption) > analysis.Backends[i].P95 {
			analysis.Backends[i].RiskColor = "lightyellow"
		}
		if float64(analysis.Backends[i].ObservedDisruption) > analysis.Backends[i].P99 {
			analysis.Backends[i].RiskColor = "pink"
		}

		logrus.WithField("backend", analysis.Backends[i]).Info("calculated total disruption")
	}

	// Sort with highest disruption at top
	sort.Slice(analysis.Backends, func(i, j int) bool {
		return analysis.Backends[i].ObservedDisruption > analysis.Backends[j].ObservedDisruption
	})

	return analysis, nil
}

func tallyBackendInAnalysis(analysis *disruptionAnalysis, backendDisruption *disruptionserializer.BackendDisruption) {
	for i, existing := range analysis.Backends {
		if existing.BackendName == backendDisruption.BackendName {
			analysis.Backends[i].ObservedDisruption = analysis.Backends[i].ObservedDisruption +
				int(backendDisruption.DisruptedDuration.Seconds())
			return
		}
	}
	// Wasn't in the list, so we add it:
	analysis.Backends = append(analysis.Backends, disruptionBackendAnalysis{
		BackendName:        backendDisruption.BackendName,
		ObservedDisruption: int(backendDisruption.DisruptedDuration.Seconds()),
	})
}

package riskanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/openshift/origin/pkg/dataloader"
	"io"
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
	maxTries                     = 3
	sippyURL                     = "https://sippy.dptools.openshift.org/sippy-ng/"
	raDataFile                   = "risk-analysis.json"
	raReqLogFileName             = "risk-analysis-requests-" + dataloader.AutoDataLoaderSuffix
	testFailureSummaryFilePrefix = "test-failures-summary"
)

// Options is used to run a risk analysis to determine how severe or unusual
// the test failures in an openshift-tests run were.
type Options struct {
	JUnitDir string
	SippyURL string
}

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

	riskAnalysisBytes, errRA := opt.readWriteRiskAnalysis(inputBytes)
	// don't fail out yet, still run disruption if RA fails

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

	if errRA != nil {
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

// struct that records the timing and status of each RA http client request
type raRequestLog struct {
	RequestCount int // which iteration are we on for this job requesting RA
	StartTime    time.Time
	Duration     time.Duration
	Error        string
	BytesRead    int
}

// readWriteRiskAnalysis requests Risk Analysis from sippy, writes the results to disk, and returns the RA html to include in prow job output.
// If the request fails, it will try up to maxTries times before returning an error; an error means no RA data returned.
func (opt *Options) readWriteRiskAnalysis(inputBytes []byte) ([]byte, error) {
	req, err := http.NewRequest("GET", opt.SippyURL, bytes.NewBuffer(inputBytes))
	if err != nil {
		logrus.WithError(err).Error("Error creating GET request during risk analysis")
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	riskAnalysisBytes, err := opt.requestRiskAnalysis(req, &http.Client{}, &realSleeper{})
	if err != nil {
		return nil, err
	}

	outputFile := filepath.Join(opt.JUnitDir, raDataFile)
	err = os.WriteFile(outputFile, riskAnalysisBytes, 0644)
	if err != nil {
		logrus.WithError(err).Error("Error writing risk analysis json artifact")
	} else {
		logrus.Infof("Successfully wrote: %s", outputFile)
	}
	return riskAnalysisBytes, nil // whether or not the file was written
}

// sleeper interface to enable testing without actually sleeping
type sleeper interface {
	Sleep(d time.Duration)
}
type realSleeper struct{}

func (rs *realSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// requestRiskAnalysis makes the http request(s) and records the timing and status for each
func (opt *Options) requestRiskAnalysis(req *http.Request, client *http.Client, sleepy sleeper) ([]byte, error) {
	var resp *http.Response
	var err error
	reqLogs := []*raRequestLog{}
	var finalReqLog *raRequestLog = nil    // keep final log entry to amend before writing if needed
	defer opt.writeRARequestLogs(&reqLogs) // write all failures or successes after processing
	clientDoSuccess := false
	for i := 1; i <= maxTries; i++ {
		reqLog := &raRequestLog{RequestCount: i, StartTime: time.Now()}
		finalReqLog = reqLog
		reqLogs = append(reqLogs, finalReqLog)
		ctx, cancelFn := context.WithTimeout(req.Context(), 20*time.Second)
		defer cancelFn()

		logrus.Infof("Requesting risk analysis (attempt %d/%d) from: %s", i, maxTries, sippyURL)
		resp, err = client.Do(req.WithContext(ctx))
		cancelFn() // cancel the context timeout we just used.
		reqLog.Duration = time.Now().Sub(reqLog.StartTime)
		logrus.Infof("Call to sippy finished after: %f seconds", reqLog.Duration.Seconds())
		if err == nil && resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("error requesting risk analysis from sippy: status %s", resp.Status)
		}
		if err == nil {
			clientDoSuccess = true
			break
		}
		reqLog.Error = fmt.Sprintf("%v", err)
		logrus.WithError(err).Warn("error requesting risk analysis from sippy, sleeping 30s")
		sleepy.Sleep(time.Duration(i*30) * time.Second)
	}
	if !clientDoSuccess {
		failure := "unable to obtain risk analysis from sippy after retries"
		logrus.WithError(err).Error(failure)
		if err == nil { // no error, but no success either
			err = fmt.Errorf(failure)
		}
		return nil, err
	}

	// we have a response, read the body
	defer resp.Body.Close()
	riskAnalysisBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Error("Error reading risk analysis request body from sippy")
		finalReqLog.Error = fmt.Sprintf("%v", err)
		return nil, err
	}
	logrus.Info("response Body:", string(riskAnalysisBytes))
	finalReqLog.BytesRead = len(riskAnalysisBytes)
	return riskAnalysisBytes, nil
}

func (opt *Options) writeRARequestLogs(logs *[]*raRequestLog) {
	rows := []map[string]string{}
	for _, log := range *logs {
		rows = append(rows, map[string]string{
			"RequestCount":    fmt.Sprintf("%d", log.RequestCount),
			"StartTime":       log.StartTime.Format(time.RFC3339),
			"DurationSeconds": fmt.Sprintf("%f", log.Duration.Seconds()),
			"Error":           log.Error,
			"BytesRead":       fmt.Sprintf("%d", log.BytesRead),
		})
	}
	dataFile := dataloader.DataFile{
		TableName: "risk_analysis_api_requests",
		Schema: map[string]dataloader.DataType{
			"RequestCount":    dataloader.DataTypeInteger,
			"StartTime":       dataloader.DataTypeTimestamp,
			"DurationSeconds": dataloader.DataTypeFloat64,
			"Error":           dataloader.DataTypeString,
			"BytesRead":       dataloader.DataTypeInteger,
		},
		Rows: rows,
	}
	fileName := filepath.Join(opt.JUnitDir, raReqLogFileName)
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}
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

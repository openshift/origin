package riskanalysis

import (
	"encoding/json"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteRARequestLogsWritesDataCorrectly(t *testing.T) {
	tmp := t.TempDir()
	opt := &Options{JUnitDir: tmp}
	logs := []*raRequestLog{
		{
			RequestCount: 1,
			StartTime:    time.Now(),
			Duration:     2 * time.Second,
			Error:        "",
			BytesRead:    1024,
		},
	}

	opt.writeRARequestLogs(&logs)

	// Attempt to load JSON from the expected log file
	fileName := filepath.Join(tmp, raReqLogFileName)
	fileContent, err := os.ReadFile(fileName)
	assert.NoError(t, err, "Failed to read the output file")

	// println(string(fileContent))

	// Check if the file content is valid JSON
	var data struct {
		TableName string `json:"table_name"`
		Rows      []struct {
			BytesRead string `json:"BytesRead"`
		} `json:"rows"`
	}
	err = json.Unmarshal(fileContent, &data)
	assert.NoError(t, err, "File content is not valid JSON")

	// further assert the content of the JSON
	assert.Equal(t, "risk_analysis_api_requests", data.TableName)
	assert.Equal(t, "1024", data.Rows[0].BytesRead)
}

type mockSleeper struct{}

func (ms *mockSleeper) Sleep(d time.Duration) {
	// Do nothing
}

func TestRequestRiskAnalysisSuccessAfterRetry(t *testing.T) {
	defer gock.Off() // Ensure that we clean up after the test

	const reqContent = `{"success": true}`
	const url = "https://example.com"
	// First call fails, second succeeds
	gock.New(url).Reply(500)
	gock.New(url).Reply(200).BodyString(reqContent)
	client := &http.Client{}
	gock.InterceptClient(client)

	tmp := t.TempDir()
	opt := &Options{SippyURL: url, JUnitDir: tmp}
	// req, _ := http.NewRequest("GET", opt.SippyURL, nil)
	raContent, err := opt.requestRiskAnalysis(make([]byte, 0), client, &mockSleeper{})
	assert.NoError(t, err, "Failed to read the request content")
	assert.Equal(t, reqContent, string(raContent))

	// Attempt to load JSON from the expected log file
	fileName := filepath.Join(tmp, raReqLogFileName)
	fileContent, err := os.ReadFile(fileName)
	assert.NoError(t, err, "Failed to read the log file")
	// println(string(fileContent))

	// Check if the file content has expected JSON and recorded logs
	var data struct {
		Rows []interface{} `json:"rows"`
	}
	err = json.Unmarshal(fileContent, &data)
	assert.NoError(t, err, "File content is not valid JSON")
	assert.Equal(t, 2, len(data.Rows))
}

func TestRequestRiskAnalysisAllRetriesFail(t *testing.T) {
	defer gock.Off() // Ensure that we clean up after the test

	const url = "https://example.com"
	// First call fails, second succeeds
	gock.New(url).Persist().Reply(500)
	client := &http.Client{}
	gock.InterceptClient(client)

	tmp := t.TempDir()
	opt := &Options{SippyURL: url, JUnitDir: tmp}
	// req, _ := http.NewRequest("GET", opt.SippyURL, nil)
	_, err := opt.requestRiskAnalysis(make([]byte, 0), client, &mockSleeper{})
	assert.Error(t, err, "Should fail to request RA content")

	// Attempt to load JSON from the expected log file
	fileName := filepath.Join(tmp, raReqLogFileName)
	fileContent, err := os.ReadFile(fileName)
	assert.NoError(t, err, "Failed to read the log file")
	// println(string(fileContent))
	// Check if the file content has expected JSON and recorded logs
	var data struct {
		Rows []interface{} `json:"rows"`
	}
	err = json.Unmarshal(fileContent, &data)
	assert.NoError(t, err, "Log file content is not valid JSON")
	assert.Equal(t, 3, len(data.Rows), "expect three attempts that fail")
}

func TestWriteRAResultsWritesDataCorrectly(t *testing.T) {
	var sample []byte = []byte(`
		{
		  "ProwJobName": "periodic-ci-openshift-release-master-ci-4.17-e2e-aws-ovn-upgrade",
		  "ProwJobRunID": 1808221684344295424,
		  "Release": "4.17",
		  "CompareRelease": "4.17",
		  "Tests": [
			{
			  "Name": "[sig-api-machinery] ValidatingAdmissionPolicy [Privileged:ClusterAdmin] should type check a CRD [Suite:openshift/conformance/parallel] [Suite:k8s]",
			  "TestID": 133049,
			  "Risk": {
				"Level": {
				  "Name": "Medium",
				  "Level": 50
				},
				"Reasons": [
				  "This test has passed 90.67% of 343 runs on jobs ['periodic-ci-openshift-release-master-ci-4.17-e2e-aws-ovn-upgrade'] in the last 14 days."
				],
				"CurrentRuns": 343,
				"CurrentPasses": 311,
				"CurrentPassPercentage": 90.67055393586006
			  },
			  "OpenBugs": []
			}
		  ],
		  "OverallRisk": {
			"Level": {
			  "Name": "Medium",
			  "Level": 50
			},
			"Reasons": [
			  "Maximum failed test risk: Medium"
			],
			"JobRunTestCount": 3446,
			"JobRunTestFailures": 2,
			"NeverStableJob": false,
			"HistoricalRunTestCount": 3208
		  },
		  "OpenBugs": []
		}

`)

	tmp := t.TempDir()
	opt := &Options{JUnitDir: tmp}
	opt.writeRAResults(sample)

	// Attempt to load and validate expected JSON from the tests autodl file
	// ---------------------------------------------------------------------
	fileName := filepath.Join(tmp, raTestResultsFileName)
	fileContent, err := os.ReadFile(fileName)
	assert.NoError(t, err, "Failed to read the output file %s", fileName)

	// println(string(fileContent))
	// Check if the file content is valid JSON
	var results struct {
		TableName string `json:"table_name"`
		Rows      []struct {
			TestName  string
			TestID    string
			RiskLevel string
			RiskName  string
		} `json:"rows"`
	}
	err = json.Unmarshal(fileContent, &results)
	assert.NoError(t, err, "File %s content is not valid JSON", fileName)

	// further assert the content of the JSON
	assert.Equal(t, raTestResultsTableName, results.TableName)
	assert.Equal(t, 1, len(results.Rows))
	assert.Equal(t, "50", results.Rows[0].RiskLevel)
	assert.Equal(t, "Medium", results.Rows[0].RiskName)
	assert.Contains(t, results.Rows[0].TestName, "ValidatingAdmissionPolicy")
	assert.Equal(t, "133049", results.Rows[0].TestID)

	// Attempt to load and validate expected JSON from the overall autodl file
	// ---------------------------------------------------------------------
	fileName = filepath.Join(tmp, raOverallRiskFileName)
	fileContent, err = os.ReadFile(fileName)
	assert.NoError(t, err, "Failed to read the output file %s", fileName)

	// println(string(fileContent))
	// Check if the file content is valid JSON
	var overallResults struct {
		TableName string `json:"table_name"`
		Rows      []struct {
			RiskLevel      string
			RiskName       string
			NeverStableJob string
		} `json:"rows"`
	}
	err = json.Unmarshal(fileContent, &overallResults)
	assert.NoError(t, err, "File %s content is not valid JSON", fileName)

	// further assert the content of the JSON
	assert.Equal(t, raOverallRiskTableName, overallResults.TableName)
	assert.Equal(t, 1, len(overallResults.Rows))
	assert.Equal(t, "50", overallResults.Rows[0].RiskLevel)
	assert.Equal(t, "Medium", overallResults.Rows[0].RiskName)
	assert.Equal(t, "false", overallResults.Rows[0].NeverStableJob)
}

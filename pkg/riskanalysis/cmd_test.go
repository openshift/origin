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
	req, _ := http.NewRequest("GET", opt.SippyURL, nil)
	raContent, err := opt.requestRiskAnalysis(req, client, &mockSleeper{})
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
	req, _ := http.NewRequest("GET", opt.SippyURL, nil)
	_, err := opt.requestRiskAnalysis(req, client, &mockSleeper{})
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

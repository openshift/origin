package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
)

const (
	batchSize = 500
)

func pushBatch(client *http.Client, bearerToken string, lokiData LokiData) error {
	logrus.Infof("pushing batch of %d intervals", len(lokiData.Streams[0].Values))
	url := "https://logging-loki-openshift-operators-redhat.apps.cr.j7t7.p1.openshiftapps.com/api/logs/v1/openshift-trt/loki/api/v1/push"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + bearerToken,
	}
	jsonData, err := json.Marshal(lokiData)
	if err != nil {
		logrus.WithError(err).Error("error marshalling loki data")
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logrus.WithError(err).Error("error creating HTTP request")
		return err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	response, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).Error("error making HTTP request")
		return err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.WithError(err).Error("error reading response body")
		return err
	}

	status := response.StatusCode
	if len(body) > 0 {
		logrus.Infof("request body: %s", body)
	}
	if status >= 200 && status < 300 {
		logrus.WithField("status", status).
			Infof("%d log entries pushed to loki", len(lokiData.Streams[0].Values))
	} else {
		logrus.WithField("status", status).
			Errorf("%d log entries failed to push to loki", len(lokiData.Streams[0].Values))
		return fmt.Errorf("logs push to loki failed with http status %d", status)
	}

	// TODO: error handling and retries based on status code?

	return nil
}

func UploadIntervalsToLoki(clientID, clientSecret string, intervals monitorapi.Intervals) error {
	bearerToken, err := obtainSSOBearerToken(clientID, clientSecret, intervals)
	if err != nil {
		return err
	}

	logrus.Info("bearer token obtained")
	client := &http.Client{}

	// NOTE: there is a promtail go client available at https://github.com/grafana/loki/blob/main/clients/pkg/promtail/client/client.go,
	// however it appears near impossible to vendor. For now, we roll our own with http requests, the worst
	// part of which is we lose out on resilient retries in the event the ingesters are overloaded.

	values := make([][]interface{}, 0)
	// TODO: Replace, this is exported in scripts not globally defined: export OPENSHIFT_INSTALL_INVOKER="openshift-internal-ci/${JOB_NAME}/${BUILD_ID}"
	invoker := os.Getenv("OPENSHIFT_INSTALL_INVOKER")
	if invoker == "" {
		return fmt.Errorf("OPENSHIFT_INSTALL_INVOKER is needed for the invoker loki label")
	}
	labels := map[string]string{
		"invoker": invoker,
		"src":     "origin-interval",
	}

	for _, i := range intervals {
		//logLine := LogEntry{Entry: i.Message}
		logLine := map[string]string{
			"_entry": i.Message,
		}

		if strings.HasPrefix(i.Locator, "e2e-test/\"") {
			startIndex := strings.Index(i.Locator, "\"") + 1
			endIndex := strings.LastIndex(i.Locator, "\"")
			logLine["e2e-test"] = i.Locator[startIndex:endIndex]
			continue
		}

		locatorParts := strings.Split(i.Locator, " ")

		for _, lp := range locatorParts {
			parts := strings.Split(lp, "/")
			tag, val := parts[0], parts[1]

			// TODO: Unclear if we should massage data here to match existing loki entries and open the door
			// to efficient searches by namespace/node (which will require the intervals processing code
			// in sippy to change a little and no longer work with plain intervals files from gcs)...
			/*
				if tag == "ns" {
					tag = "namespace"
				}
				if tag == "node" {
					tag = "host"
				}

			*/

			logLine[tag] = val
		}

		// Convert the log data to a packed json log line, where the message is in '_entry'.
		// This allows us to search additional fields we didn't index as labels using the unpack/json
		// directives in logQL.
		logLineJson, err := json.Marshal(logLine)
		if err != nil {
			logrus.WithError(err).Errorf("unable to unmashall log line: %s", logLine)
		}

		values = append(values, []interface{}{
			strconv.FormatInt(i.From.UnixNano(), 10),
			string(logLineJson),
		})
		//fmt.Printf("Added value: %v\n", values[len(values)-1])

		if len(values) == batchSize {
			ld := LokiData{
				Streams: []Streams{
					{
						Stream: labels,
						Values: values,
					},
				},
			}
			err := pushBatch(client, bearerToken, ld)
			if err != nil {
				return err
			}
			values = make([][]interface{}, 0)
		}
	}

	// Push the final batch:
	if len(values) > 0 {
		ld := LokiData{
			Streams: []Streams{
				{
					Stream: labels,
					Values: values,
				},
			},
		}
		err := pushBatch(client, bearerToken, ld)
		if err != nil {
			return err
		}
		values = make([][]interface{}, 0)
	}
	return nil
}

type Streams struct {
	Stream map[string]string `json:"stream"`
	Values [][]interface{}   `json:"values"` // array of arrays of [ts, jsonStr]
}

type LokiData struct {
	Streams []Streams `json:"streams"`
}

func obtainSSOBearerToken(clientID, clientSecret string, intervals monitorapi.Intervals) (string, error) {
	logrus.Info("requesting bearer token from RH SSO")
	// Obtain a bearer token for pushing intervals to loki using RH SSO:
	client := &http.Client{}
	requestBody := url.Values{}
	requestBody.Set("grant_type", "client_credentials")
	requestBody.Set("client_id", clientID)
	requestBody.Set("client_secret", clientSecret)

	req, err := http.NewRequest("POST", "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token", strings.NewReader(requestBody.Encode()))
	if err != nil {
		logrus.WithError(err).Error("error creating bearer token HTTP request")
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).Error("error making bearer token HTTP request")
		return "", err
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.WithError(err).Error("error reading bearer token HTTP request body for bearer token")
		return "", err
	}

	var responseJSON map[string]interface{}
	if err := json.Unmarshal(responseBody, &responseJSON); err != nil {
		logrus.WithError(err).Error("error parsing bearer token HTTP request body as json")
		fmt.Println("Error decoding response JSON:", err)
		return "", err
	}
	bearerToken := responseJSON["access_token"].(string)
	return bearerToken, nil
}

package uploadtolokiserializer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type lokiSerializer struct {
}

func NewUploadSerializer() monitortestframework.MonitorTest {
	return &lokiSerializer{}
}

func (w *lokiSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *lokiSerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*lokiSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*lokiSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*lokiSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	fmt.Fprintf(os.Stderr, "Uploading to loki.\n")
	if err := UploadIntervalsToLoki(finalIntervals); err != nil {
		// Best effort, we do not want to error out here:
		// TODO do we need to have a junit return option from this function to allow us to find failures?
		logrus.WithError(err).Warn("unable to upload intervals to loki")
	}

	return nil
}

func (*lokiSerializer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

const (
	batchSize = 500
)

var batchCtr int

// pushBatch sends logs to Loki in batches of 500 using raw HTTP requets. Attempts were made to vendor and use the
// promtail library but this gets extremely complicated when you already vendor kube.
// Includes a rudimentary retry mechanism.
func pushBatch(client *http.Client, bearerToken string, lokiData LokiData) error {
	batchCtr++
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

	return retry.OnError(
		wait.Backoff{
			Duration: 2 * time.Second,
			Steps:    5,
			Factor:   5.0,
			Jitter:   0.1,
		},
		func(err error) bool {
			// re-try on any error
			logrus.WithError(err).Warningf("error occurred on batch %d, re-trying", batchCtr)
			return true
		},
		func() error {
			logrus.Infof("pushing batch %d of %d intervals", batchCtr,
				len(lokiData.Streams[0].Values))
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

			return nil
		})
}

// UploadIntervalsToLoki attempts to push the set of intervals from this portion of the job run up to Loki,
// allowing for more efficient querying and searching in TRT tooling.
func UploadIntervalsToLoki(intervals monitorapi.Intervals) error {

	// Ensure we have appropriate env vars defined, if not exit with an error.
	clientID, defined := os.LookupEnv("LOKI_SSO_CLIENT_ID")
	if !defined {
		return fmt.Errorf("LOKI_SSO_CLIENT_ID env var is not defined")
	}
	clientSecret, defined := os.LookupEnv("LOKI_SSO_CLIENT_SECRET")
	if !defined {
		return fmt.Errorf("LOKI_SSO_CLIENT_SECRET env var is not defined")
	}
	jobName, defined := os.LookupEnv("JOB_NAME")
	if !defined {
		return fmt.Errorf("JOB_NAME env var is not defined")
	}
	buildID, defined := os.LookupEnv("BUILD_ID")
	if !defined {
		return fmt.Errorf("BUILD_ID env var is not defined")
	}

	bearerToken, err := obtainSSOBearerToken(clientID, clientSecret)
	if err != nil {
		return err
	}

	logrus.Info("bearer token obtained")
	client := &http.Client{}

	// NOTE: there is a promtail go client available at https://github.com/grafana/loki/blob/main/clients/pkg/promtail/client/client.go,
	// however it appears near impossible to vendor. For now, we roll our own with http requests, the worst
	// part of which is we lose out on resilient retries in the event the ingesters are overloaded.

	values := make([][]interface{}, 0)
	// This format matches the labels used for the cluster logs in loki, and we want this
	// to be labelled the same.
	invoker := fmt.Sprintf("openshift-internal-ci/%s/%s", jobName, buildID)
	if invoker == "" {
		return fmt.Errorf("OPENSHIFT_INSTALL_INVOKER is needed for the invoker loki label")
	}
	labels := map[string]string{
		"invoker": invoker,
		"type":    "origin-interval",
		// TODO: in future, may group intervals in origin rather than in the js itself, if we do,
		// ensure this group gets carried over to this series.
	}

	for _, i := range intervals {
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
			if len(parts) < 2 {
				logrus.Warnf("unable to split locator parts: %+v", lp)
				continue
			}
			tag, val := parts[0], parts[1]
			if strings.TrimSpace(tag) == "" {
				// Have seen this fail on: WARN[0002] unable to process locator tag: May 12 11:10:00.000 I /machine-config reason/OperatorVersionChanged clusteroperator/machine-config-operator version changed from [{operator 4.14.0-0.nightly-2023-05-03-163151}] to [{operator 4.14.0-0.nightly-2023-05-12-121801}]
				// Would be interesting to track down where this is coming from. Should be identification.go OperatorLocator, but this is impossible.
				logrus.Warnf("unable to process locator tag: %+v", i)
				continue
			}

			// some locator fields can slip in with '-', which is not a valid json key and then breaks unpack in loki.
			// replace them with '_'.
			tag = strings.ReplaceAll(tag, "-", "_")

			// WARNING: opting for a potentially risky change here, I want namespace filtering to be available as a
			// label in loki soon,	and thus I am translating some labels we used historically in origin intervals to
			// match those we pull from pod logs in the cluster itself. This will require our intervals charts in sippy
			// be able to parse this new name instead of the other. Ideally, we would go rename these fully in origin wherever used.
			if tag == "ns" {
				tag = "namespace"
			}
			if tag == "node" {
				tag = "host"
			}

			logLine[tag] = val
		}

		// Convert the log data to a packed json log line, where the message is in '_entry'.
		// This allows us to search additional fields we didn't index as labels using the unpack/json
		// directives in logQL.
		logLineJson, err := json.Marshal(logLine)
		if err != nil {
			logrus.WithError(err).Errorf("unable to mashall log line to json: %s", logLine)
			continue
		}

		if strings.HasPrefix(logLine["namespace"], "openshift-") {
			// If we have a namespace in the locator, bump it up to a proper loki label:
			labels["namespace"] = logLine["namespace"]
		}

		values = append(values, []interface{}{
			strconv.FormatInt(i.From.UnixNano(), 10),
			string(logLineJson),
		})

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

func obtainSSOBearerToken(clientID, clientSecret string) (string, error) {
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
	logrus.WithField("status", response.StatusCode).Info("obtained response to bearer token request")

	var responseJSON map[string]interface{}
	if err := json.Unmarshal(responseBody, &responseJSON); err != nil {
		logrus.WithError(err).Error("error parsing bearer token HTTP request body as json")
		return "", err
	}
	if _, ok := responseJSON["access_token"]; !ok {
		logrus.Error("no access token in response")
		return "", fmt.Errorf("no access token in response")
	}
	bearerToken := responseJSON["access_token"].(string)
	return bearerToken, nil
}

package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

const (
	batchSize = 500
)

var batchCtr int

// pushBatch sends logs to Loki in batches of 500 using raw HTTP requets. Attempts were made to vendor and use the
// promtail library but this gets extremely complicated when you already vendor kube.
// Includes a rudimentary retry mechanism.
func pushBatch(client *http.Client, bearerToken, invoker, namespace string, values [][]interface{}, dryRun bool) error {
	batchCtr++
	logger := logrus.WithField("namespace", namespace)
	logger.Info("uploading an intervals batch")

	labels := map[string]string{
		"invoker": invoker,
		"type":    "origin-interval",
		// TODO: in future, may group intervals in origin rather than in the js itself, if we do,
		// ensure this group gets carried over to this series.
	}
	if namespace != "" {
		labels["namespace"] = namespace
	}
	lokiData := LokiData{
		Streams: []Streams{
			{
				Stream: labels,
				Values: values,
			},
		},
	}

	lokiURL := "https://logging-loki-openshift-operators-redhat.apps.cr.j7t7.p1.openshiftapps.com/api/logs/v1/openshift-trt/loki/api/v1/push"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + bearerToken,
	}
	jsonData, err := json.Marshal(lokiData)
	if err != nil {

		logger.WithError(err).Error("error marshalling loki data")
		return err
	}

	req, err := http.NewRequest("POST", lokiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.WithError(err).Error("error creating HTTP request")
		return err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if dryRun {
		logger.Warn("skipping upload (dry-run)")
		return nil
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
			logger.WithError(err).Warningf("error occurred on batch %d, re-trying", batchCtr)
			return true
		},
		func() error {
			logger.Infof("pushing batch %d of %d intervals", batchCtr,
				len(lokiData.Streams[0].Values))
			response, err := client.Do(req)
			if err != nil {
				logger.WithError(err).Error("error making HTTP request")
				return err
			}
			defer response.Body.Close()

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				logger.WithError(err).Error("error reading response body")
				return err
			}

			status := response.StatusCode
			if len(body) > 0 {
				logger.Infof("request body: %s", body)
			}
			if status >= 200 && status < 300 {
				logger.WithField("status", status).
					Infof("%d log entries pushed to loki", len(lokiData.Streams[0].Values))
			} else {
				logger.WithField("status", status).
					Errorf("%d log entries failed to push to loki", len(lokiData.Streams[0].Values))
				return fmt.Errorf("logs push to loki failed with http status %d", status)
			}

			return nil
		})
}

// UploadIntervalsToLoki attempts to push the set of intervals from this portion of the job run up to Loki,
// allowing for more efficient querying and searching in TRT tooling.
//
// timeSuffix is the same as used when writing e2e-events json files to gcs. It allows us
// to diffentiate between multiple sets of intervals from the same job run. (i.e. upgrade and conformance run)
func UploadIntervalsToLoki(intervals monitorapi.Intervals, timeSuffix string, dryRun bool) error {

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

	var bearerToken string
	var err error
	if !dryRun {
		bearerToken, err = obtainSSOBearerToken(clientID, clientSecret)
		if err != nil {
			return err
		}
	}

	logrus.Info("bearer token obtained")
	client := &http.Client{}

	// This format matches the labels used for the cluster logs in loki, and we want this
	// to be labelled the same.
	invoker := fmt.Sprintf("openshift-internal-ci/%s/%s", jobName, buildID)
	if invoker == "" {
		return fmt.Errorf("OPENSHIFT_INSTALL_INVOKER is needed for the invoker loki label")
	}
	// maps namespace name to all the log entries for that namespace. used as we need to batch up all the log entries
	// per stream (set of labels), which includes namespace. Any intervals without a namespace, or in a non-openshift
	// namespace, will be under the key ""
	nsIntervals := map[string][][]interface{}{}

	for _, i := range intervals {
		namespace, logLine, err := intervalToLogLine(i, timeSuffix)

		// Convert the log data to a packed json log line, where the message is in '_entry'.
		// This allows us to search additional fields we didn't index as labels using the unpack/json
		// directives in logQL.
		logLineJson, err := json.Marshal(logLine)
		if err != nil {
			logrus.WithError(err).Errorf("unable to mashall log line to json: %s", logLine)
			continue
		}

		if _, ok := nsIntervals[namespace]; !ok {
			nsIntervals[namespace] = make([][]interface{}, 0)
		}
		nsIntervals[namespace] = append(nsIntervals[namespace], []interface{}{
			strconv.FormatInt(i.From.UnixNano(), 10),
			string(logLineJson),
		})

		if len(nsIntervals[namespace]) == batchSize {
			err := pushBatch(client, bearerToken, invoker, namespace, nsIntervals[namespace], dryRun)
			if err != nil {
				return err
			}
			nsIntervals[namespace] = make([][]interface{}, 0)
		}
	}

	// Push the final batches for each namespace:
	logrus.Info("pushing final batches for each remaining namespace")
	for namespace, v := range nsIntervals {
		if len(v) > 0 {
			err := pushBatch(client, bearerToken, invoker, namespace, nsIntervals[namespace], dryRun)
			if err != nil {
				return err
			}
			nsIntervals[namespace] = make([][]interface{}, 0)
		}
	}
	return nil
}

func intervalToLogLine(i monitorapi.EventInterval, timeSuffix string) (string, map[string]string, error) {

	logLine := map[string]string{
		"_entry":   i.Message,
		"filename": timeSuffix, // used in the UI to differentiate batches of intervals for one job run (upgrade + conformance)
	}

	// Loki logs are timestamped, for this we use the "from" of the interval. We'll include a durationSec
	// so we can calculate the "to" time in our tooling. This will also have the benefit of allowing loki queries
	// on durationSec > 5 or similar.
	if i.To.IsZero() {
		// Not 100% sure To is always set
		logLine["durationSec"] = "1"
	} else {
		d := i.To.Sub(i.From)
		durInSeconds := math.Round(d.Seconds())
		if durInSeconds < 1 {
			durInSeconds = 1
		}
		logLine["durationSec"] = strconv.FormatFloat(durInSeconds, 'f', -1, 64)
	}

	logLine["level"] = i.Level.String()

	if strings.HasPrefix(i.Locator, "e2e-test/\"") {
		startIndex := strings.Index(i.Locator, "\"") + 1
		endIndex := strings.LastIndex(i.Locator, "\"")
		logLine["e2e-test"] = i.Locator[startIndex:endIndex]
		return "", logLine, nil
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

	// If we have a namespace in the locator, return and it will be used as a proper label:
	var namespace string
	if strings.HasPrefix(logLine["namespace"], "openshift-") {
		namespace = logLine["namespace"]
	}

	return namespace, logLine, nil
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

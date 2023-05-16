package monitor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
)

func UploadIntervalsToLoki(clientID, clientSecret string, intervals monitorapi.Intervals) error {
	_, err := obtainSSOBearerToken(clientID, clientSecret, intervals)
	if err != nil {
		return err
	}
	logrus.Info("bearer token obtained")
	return nil
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

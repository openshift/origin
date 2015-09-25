// +build integration,etcd

package integration

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testserver "github.com/openshift/origin/test/util/server"
)

func tryAccessURL(t *testing.T, url string, expectedStatus int, expectedRedirectLocation string) *http.Response {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "text/html")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Errorf("Unexpected error while accessing %q: %v", url, err)
		return nil
	}
	if resp.StatusCode != expectedStatus {
		t.Errorf("Expected status %d for %q, got %d", expectedStatus, url, resp.StatusCode)
	}
	// ignore query parameters
	location := resp.Header.Get("Location")
	location = strings.SplitN(location, "?", 2)[0]
	if location != expectedRedirectLocation {
		t.Errorf("Expected redirecttion to %q for %q, got %q instead", expectedRedirectLocation, url, location)
	}
	return resp
}

func TestAccessOriginWebConsole(t *testing.T) {
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err = testserver.StartConfiguredMaster(masterOptions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for endpoint, exp := range map[string]struct {
		statusCode int
		location   string
	}{
		"":                    {http.StatusFound, masterOptions.AssetConfig.PublicURL},
		"healthz":             {http.StatusOK, ""},
		"login":               {http.StatusOK, ""},
		"oauth/token/request": {http.StatusFound, masterOptions.AssetConfig.MasterPublicURL + "/oauth/authorize"},
		"console":             {http.StatusMovedPermanently, "/console/"},
		"console/":            {http.StatusOK, ""},
		"console/java":        {http.StatusOK, ""},
	} {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		tryAccessURL(t, url, exp.statusCode, exp.location)
	}
}

func TestAccessDisabledWebConsole(t *testing.T) {
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterOptions.DisabledFeatures.Add(configapi.FeatureWebConsole)
	if _, err := testserver.StartConfiguredMaster(masterOptions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := tryAccessURL(t, masterOptions.AssetConfig.MasterPublicURL+"/", http.StatusOK, "")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read reposponse's body: %v", err)
	} else {
		var value interface{}
		if err = json.Unmarshal(body, &value); err != nil {
			t.Errorf("expected json body which couldn't be parsed: %v, got: %s", err, body)
		}
	}

	for endpoint, exp := range map[string]struct {
		statusCode int
		location   string
	}{
		"healthz":             {http.StatusOK, ""},
		"login":               {http.StatusOK, ""},
		"oauth/token/request": {http.StatusFound, masterOptions.AssetConfig.MasterPublicURL + "/oauth/authorize"},
		"console":             {http.StatusForbidden, ""},
		"console/":            {http.StatusForbidden, ""},
		"console/java":        {http.StatusForbidden, ""},
	} {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		tryAccessURL(t, url, exp.statusCode, exp.location)
	}
}

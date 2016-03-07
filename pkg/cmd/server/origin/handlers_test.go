package origin

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kversion "k8s.io/kubernetes/pkg/version"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/version"
)

var (
	currentOCKubeResources                 = "oc/v1.2.0 (linux/amd64) kubernetes/bc4550d"
	currentOCOriginResources               = "oc/v1.1.3 (linux/amd64) openshift/b348c2f"
	currentOpenshiftKubectlKubeResources   = "openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d"
	currentOpenshiftKubectlOriginResources = "openshift/v1.1.3 (linux/amd64) openshift/b348c2f"
	currentOADMKubeResources               = "oadm/v1.2.0 (linux/amd64) kubernetes/bc4550d"
	currentOADMOriginResources             = "oadm/v1.1.3 (linux/amd64) openshift/b348c2f"
	currentVersionUserAgents               = []string{
		currentOCKubeResources, currentOCOriginResources, currentOpenshiftKubectlKubeResources, currentOpenshiftKubectlOriginResources, currentOADMKubeResources, currentOADMOriginResources}

	olderOCKubeResources                 = "oc/v1.1.10 (linux/amd64) kubernetes/bc4550d"
	olderOCOriginResources               = "oc/v1.1.1 (linux/amd64) openshift/b348c2f"
	olderOpenshiftKubectlKubeResources   = "openshift/v1.1.10 (linux/amd64) kubernetes/bc4550d"
	olderOpenshiftKubectlOriginResources = "openshift/v1.1.1 (linux/amd64) openshift/b348c2f"
	olderOADMKubeResources               = "oadm/v1.1.10 (linux/amd64) kubernetes/bc4550d"
	olderOADMOriginResources             = "oadm/v1.1.1 (linux/amd64) openshift/b348c2f"
	olderVersionUserAgents               = []string{
		olderOCKubeResources, olderOCOriginResources, olderOpenshiftKubectlKubeResources, olderOpenshiftKubectlOriginResources, olderOADMKubeResources, olderOADMOriginResources}

	newerOCKubeResources                 = "oc/v1.2.1 (linux/amd64) kubernetes/bc4550d"
	newerOCOriginResources               = "oc/v1.1.4 (linux/amd64) openshift/b348c2f"
	newerOpenshiftKubectlKubeResources   = "openshift/v1.2.1 (linux/amd64) kubernetes/bc4550d"
	newerOpenshiftKubectlOriginResources = "openshift/v1.1.4 (linux/amd64) openshift/b348c2f"
	newerOADMKubeResources               = "oadm/v1.2.1 (linux/amd64) kubernetes/bc4550d"
	newerOADMOriginResources             = "oadm/v1.1.4 (linux/amd64) openshift/b348c2f"
	newerVersionUserAgents               = []string{
		newerOCKubeResources, newerOCOriginResources, newerOpenshiftKubectlKubeResources, newerOpenshiftKubectlOriginResources, newerOADMKubeResources, newerOADMOriginResources}

	notOCVersion = "something else"

	openshiftServerVersion = version.Info{GitVersion: "v1.1.3"}
	kubeServerVersion      = kversion.Info{GitVersion: "v1.2.0"}
)

type versionSkewTestCase struct {
	name           string
	userAgents     []string
	failureMessage string
	methods        []string
}

func (tc versionSkewTestCase) Run(url string, t *testing.T) {
	// gets always succeed
	for _, userAgent := range tc.userAgents {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			return
		}
		req.Header.Add("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: unexpected status: %v", tc.name, resp.StatusCode)
			return
		}
	}

	for _, method := range tc.methods {
		for _, userAgent := range tc.userAgents {
			req, err := http.NewRequest(method, url, nil)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
				return
			}
			req.Header.Add("User-Agent", userAgent)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
				return
			}
			if len(tc.failureMessage) == 0 {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("%s: unexpected status: %v", tc.name, resp.StatusCode)
					return
				}

			} else {
				if resp.StatusCode != http.StatusForbidden {
					t.Errorf("%s: unexpected status: %v", tc.name, resp.StatusCode)
					return
				}

				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.name, err)
					return
				}

				if !strings.Contains(string(body), tc.failureMessage) {
					t.Errorf("%s: expected %v, got %v", tc.name, tc.failureMessage, string(body))
					return
				}
			}
		}
	}

}

func TestVersionSkewFilterAllowAll(t *testing.T) {
	verbs := []string{"PUT", "POST"}
	doNothingHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	})
	config := MasterConfig{}
	config.Options.PolicyConfig.LegacyClientPolicyConfig.LegacyClientPolicy = configapi.AllowAll
	config.Options.PolicyConfig.LegacyClientPolicyConfig.RestrictedHTTPVerbs = verbs
	server := httptest.NewServer(config.versionSkewFilter(openshiftServerVersion, kubeServerVersion, doNothingHandler))
	defer server.Close()

	testCases := []versionSkewTestCase{
		{
			name:       "missing",
			userAgents: []string{""},
			methods:    verbs,
		},
		{
			name:       "not oc",
			userAgents: []string{notOCVersion},
			methods:    verbs,
		},
		{
			name:       "older",
			userAgents: olderVersionUserAgents,
			methods:    verbs,
		},
		{
			name:       "newer",
			userAgents: newerVersionUserAgents,
			methods:    verbs,
		},
		{
			name:       "exact",
			userAgents: currentVersionUserAgents,
			methods:    verbs,
		},
	}

	for _, tc := range testCases {
		tc.Run(server.URL, t)
	}
}

func TestVersionSkewFilterDenyOld(t *testing.T) {
	verbs := []string{"PATCH", "POST"}
	doNothingHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	})
	config := MasterConfig{}
	config.Options.PolicyConfig.LegacyClientPolicyConfig.LegacyClientPolicy = configapi.DenyOldClients
	config.Options.PolicyConfig.LegacyClientPolicyConfig.RestrictedHTTPVerbs = verbs
	server := httptest.NewServer(config.versionSkewFilter(openshiftServerVersion, kubeServerVersion, doNothingHandler))
	defer server.Close()

	testCases := []versionSkewTestCase{
		{
			name:       "missing",
			userAgents: []string{""},
			methods:    verbs,
		},
		{
			name:       "not oc",
			userAgents: []string{notOCVersion},
			methods:    verbs,
		},
		{
			name:           "older",
			userAgents:     olderVersionUserAgents,
			failureMessage: " is older than the server version",
			methods:        verbs,
		},
		{
			name:       "newer",
			userAgents: newerVersionUserAgents,
			methods:    verbs,
		},
		{
			name:       "exact",
			userAgents: currentVersionUserAgents,
			methods:    verbs,
		},
	}

	for _, tc := range testCases {
		tc.Run(server.URL, t)
	}
}

func TestVersionSkewFilterDenySkewed(t *testing.T) {
	verbs := []string{"PUT", "DELETE"}
	doNothingHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	})
	config := MasterConfig{}
	config.Options.PolicyConfig.LegacyClientPolicyConfig.LegacyClientPolicy = configapi.DenySkewedClients
	config.Options.PolicyConfig.LegacyClientPolicyConfig.RestrictedHTTPVerbs = verbs
	server := httptest.NewServer(config.versionSkewFilter(openshiftServerVersion, kubeServerVersion, doNothingHandler))
	defer server.Close()

	testCases := []versionSkewTestCase{
		{
			name:       "missing",
			userAgents: []string{""},
			methods:    verbs,
		},
		{
			name:       "not oc",
			userAgents: []string{notOCVersion},
			methods:    verbs,
		},
		{
			name:           "older",
			userAgents:     olderVersionUserAgents,
			failureMessage: "is different than the server version",
			methods:        verbs,
		},
		{
			name:           "newer",
			userAgents:     newerVersionUserAgents,
			failureMessage: "is different than the server version",
			methods:        verbs,
		},
		{
			name:       "current",
			userAgents: currentVersionUserAgents,
			methods:    verbs,
		},
	}

	for _, tc := range testCases {
		tc.Run(server.URL, t)
	}
}

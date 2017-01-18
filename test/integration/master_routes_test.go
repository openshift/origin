package integration

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	knet "k8s.io/kubernetes/pkg/util/net"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// expectedIndex contains the routes expected at the api root /. Keep them sorted.
var expectedIndex = []string{
	"/api",
	"/api/v1",
	"/apis",
	"/apis/apps",
	"/apis/apps/v1beta1",
	"/apis/authentication.k8s.io",
	"/apis/authentication.k8s.io/v1beta1",
	"/apis/autoscaling",
	"/apis/autoscaling/v1",
	"/apis/batch",
	"/apis/batch/v1",
	"/apis/batch/v2alpha1",
	"/apis/certificates.k8s.io",
	"/apis/certificates.k8s.io/v1alpha1",
	"/apis/extensions",
	"/apis/extensions/v1beta1",
	"/apis/policy",
	"/apis/policy/v1beta1",
	"/apis/storage.k8s.io",
	"/apis/storage.k8s.io/v1beta1",
	"/controllers",
	"/healthz",
	"/healthz/ping",
	"/healthz/poststarthook/bootstrap-controller",
	"/healthz/poststarthook/extensions/third-party-resources",
	"/healthz/ready",
	"/metrics",
	"/oapi",
	"/oapi/v1",
	"/osapi",
	"/swaggerapi/",
	"/version",
	"/version/openshift",
}

func TestRootRedirect(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, err := anonymousHttpTransport(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL, nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Expected %s, got %s", "application/json", resp.Header.Get("Content-Type"))
	}
	type result struct {
		Paths []string
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Unexpected error reading the body: %v", err)
	}
	var got result
	json.Unmarshal(body, &got)
	sort.Strings(got.Paths)
	if !reflect.DeepEqual(got.Paths, expectedIndex) {
		t.Fatalf("Unexpected index: got=%v, expected=%v", got, expectedIndex)
	}

	req, err = http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL, nil)
	req.Header.Set("Accept", "text/html")
	resp, err = transport.RoundTrip(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if resp.Header.Get("Location") != masterConfig.AssetConfig.PublicURL {
		t.Errorf("Expected %s, got %s", masterConfig.AssetConfig.PublicURL, resp.Header.Get("Location"))
	}

	// TODO add a test for when asset config is nil, the redirect should not occur in this case even when
	// accept header contains text/html
}

func TestWellKnownOAuth(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, err := anonymousHttpTransport(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL+"/.well-known/oauth-authorization-server", nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Unexpected error reading the body: %v", err)
	}
	if !strings.Contains(string(body), "authorization_endpoint") {
		t.Fatal("Expected \"authorization_endpoint\" in the body.")
	}
}

func TestWellKnownOAuthOff(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterConfig.OAuthConfig = nil
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, err := anonymousHttpTransport(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL+"/.well-known/oauth-authorization-server", nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func anonymousHttpTransport(clusterAdminKubeConfig string) (*http.Transport, error) {
	restConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(restConfig.TLSClientConfig.CAData); !ok {
		return nil, errors.New("failed to add server CA certificates to client pool")
	}
	return knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{
			// only use RootCAs from client config, especially no client certs
			RootCAs: pool,
		},
	}), nil
}

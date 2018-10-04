package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestRootAPIPaths(t *testing.T) {
	masterConfig, adminConfigFile, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error starting test master: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clientConfig, err := testutil.GetClusterAdminClientConfig(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error getting cluster admin client config: %v", err)
	}

	transport, err := restclient.TransportFor(clientConfig)
	if err != nil {
		t.Fatalf("unexpected error getting transport for client config: %v", err)
	}

	rootRequest, err := http.NewRequest("GET", masterConfig.OAuthConfig.MasterPublicURL+"/", nil)
	rootRequest.Header.Set("Accept", "*/*")
	rootResponse, err := transport.RoundTrip(rootRequest)
	if err != nil {
		t.Fatalf("unexpected error issuing GET to root path: %v", err)
	}

	var broadcastRootPaths metav1.RootPaths
	if err := json.NewDecoder(rootResponse.Body).Decode(&broadcastRootPaths); err != nil {
		t.Fatalf("unexpected error decoding root path response: %v", err)
	}
	defer rootResponse.Body.Close()

	// Send a GET to every advertised address and check that we get the correct response
	for _, route := range broadcastRootPaths.Paths {
		req, err := http.NewRequest("GET", masterConfig.OAuthConfig.MasterPublicURL+route, nil)
		req.Header.Set("Accept", "*/*")
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Errorf("unexpected error issuing GET for path %q: %v", route, err)
			continue
		}
		expectedCode := http.StatusOK
		if resp.StatusCode != expectedCode {
			t.Errorf("incorrect status code for %s endpoint: expected %d, got %d", route, expectedCode, resp.StatusCode)
		}
	}
}

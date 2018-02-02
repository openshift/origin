package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
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

	// We need to make sure that any APILevels specified in the config are present in the RootPaths, and that
	// any not specified are not
	expectedOpenShiftAPILevels := sets.NewString(masterConfig.APILevels...)
	expectedKubeAPILevels := sets.NewString(configapi.GetEnabledAPIVersionsForGroup(*masterConfig.KubernetesMasterConfig, kapi.GroupName)...)
	actualOpenShiftAPILevels := sets.String{}
	actualKubeAPILevels := sets.String{}
	for _, route := range broadcastRootPaths.Paths {
		if strings.HasPrefix(route, "/oapi/") {
			actualOpenShiftAPILevels.Insert(route[6:])
		}

		if strings.HasPrefix(route, "/api/") {
			actualKubeAPILevels.Insert(route[5:])
		}
	}
	if !expectedOpenShiftAPILevels.Equal(actualOpenShiftAPILevels) {
		t.Errorf("actual OpenShift API levels served don't match expected levels:\n\texpected:\n\t%s\n\tgot:\n\t%s", expectedOpenShiftAPILevels.List(), actualOpenShiftAPILevels.List())
	}
	if !expectedKubeAPILevels.Equal(actualKubeAPILevels) {
		t.Errorf("actual Kube API levels served don't match expected levels:\n\texpected:\n\t%s\n\tgot:\n\t%s", expectedKubeAPILevels.List(), actualKubeAPILevels.List())
	}

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

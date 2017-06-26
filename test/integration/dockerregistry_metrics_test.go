package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

const integrationTestSecret = "integration-test-secret"

func runRegistryWithMetrics() error {
	config := `version: 0.1
log:
  level: debug
http:
  addr: 127.0.0.1:5000
storage:
  inmemory: {}
auth:
  openshift: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
openshift:
  version: 1.0
  metrics:
    enabled: true
    secret: ` + integrationTestSecret + `
`
	os.Setenv("DOCKER_REGISTRY_URL", "127.0.0.1:5000")

	go dockerregistry.Execute(strings.NewReader(config))

	if err := cmdutil.WaitForSuccessfulDial(false, "tcp", "127.0.0.1:5000", 100*time.Millisecond, 1*time.Second, 35); err != nil {
		return err
	}
	return nil
}

func TestDockerRegistryMetrics(t *testing.T) {
	if err := runRegistryWithMetrics(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "http://127.0.0.1:5000/v2/foo/bar/manifests/latest", nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Bearer check-me")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error sending token to registry: %s", err)
	}
	resp.Body.Close()

	req, err = http.NewRequest("GET", "http://127.0.0.1:5000/extensions/v2/metrics", nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Bearer "+integrationTestSecret)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error receiving metrics from registry: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status: %s", resp.Status)
	}

	metrics, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading metrics from registry: %s", err)
	}

	if !bytes.Contains(metrics, []byte("openshift_registry_masterapi_request_duration_seconds")) {
		t.Errorf("no metrics for master api found\n%s", metrics)
	}
}

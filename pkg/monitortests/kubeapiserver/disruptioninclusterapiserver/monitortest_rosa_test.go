package disruptioninclusterapiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitortestframework"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestROSAHCPDoesNotRequireManagementCredentials(t *testing.T) {
	infra := &configv1.Infrastructure{}
	infra.APIVersion = "config.openshift.io/v1"
	infra.Kind = "Infrastructure"
	infra.Name = "cluster"
	infra.Status.ControlPlaneTopology = configv1.ExternalTopologyMode
	infra.Status.PlatformStatus = &configv1.PlatformStatus{
		Type: configv1.AWSPlatformType,
		AWS:  &configv1.AWSPlatformStatus{ResourceTags: []configv1.AWSResourceTag{{Key: "red-hat-clustertype", Value: "rosa"}}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/v1/namespaces/kube-public/configmaps/microshift-version":
			http.NotFound(w, req)
			return
		case "/apis/config.openshift.io/v1/infrastructures/cluster":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(infra); err != nil {
				t.Error(err)
			}
			return
		default:
			t.Errorf("unexpected API request: %s %s", req.Method, req.URL.Path)
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	kubeconfig := filepath.Join(t.TempDir(), "guest-kubeconfig")
	cfg := clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"guest": {Server: server.URL}},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{"admin": {}},
		Contexts:       map[string]*clientcmdapi.Context{"guest": {Cluster: "guest", AuthInfo: "admin"}},
		CurrentContext: "guest",
	}
	if err := clientcmd.WriteToFile(cfg, kubeconfig); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)
	t.Setenv("STATIC_CONFIG_MANIFEST_DIR", "")
	t.Setenv("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG", "")
	t.Setenv("HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE", "")
	gomega.RegisterFailHandler(func(message string, _ ...int) { panic(message) })

	monitor := &InvariantInClusterDisruption{payloadImagePullSpec: "quay.io/example/release:4.19"}
	var recovered interface{}
	var err error
	func() {
		defer func() { recovered = recover() }()
		err = monitor.StartCollection(context.Background(), &rest.Config{Host: server.URL}, nil)
	}()
	if recovered != nil {
		t.Fatalf("ROSA HCP should skip before requiring management credentials: %v", recovered)
	}
	if err != nil {
		t.Fatalf("ROSA HCP setup failed instead of skipping: %v", err)
	}
	var unsupported *monitortestframework.NotSupportedError
	if !errors.As(monitor.notSupportedReason, &unsupported) || unsupported.Reason != "platform ROSA not supported" {
		t.Fatalf("expected stored ROSA unsupported reason, got %v", monitor.notSupportedReason)
	}
}

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestIngressManagementModeUsesStableClient(t *testing.T) {
	for _, testCase := range []struct {
		mode     operatorv1.GatewayAPIManagementMode
		conflict bool
	}{
		{mode: operatorv1.GatewayAPIManagementModeManaged},
		{mode: operatorv1.GatewayAPIManagementModeUnmanaged, conflict: true},
	} {
		t.Run(string(testCase.mode), func(t *testing.T) {
			initialMode := operatorv1.GatewayAPIManagementModeManaged
			if testCase.mode == initialMode {
				initialMode = operatorv1.GatewayAPIManagementModeUnmanaged
			}
			ingress := operatorv1.Ingress{
				TypeMeta: metav1.TypeMeta{APIVersion: "operator.openshift.io/v1", Kind: "Ingress"},
				ObjectMeta: metav1.ObjectMeta{
					Name: ingressCRName, ResourceVersion: "1",
				},
				Spec: operatorv1.IngressSpec{
					GatewayAPI: operatorv1.GatewayAPIIngressConfig{ManagementMode: initialMode},
				},
			}
			var lock sync.Mutex
			conflict := testCase.conflict
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				lock.Lock()
				defer lock.Unlock()
				response.Header().Set("Content-Type", "application/json")
				if request.URL.Path != "/apis/operator.openshift.io/v1/ingresses/cluster" {
					t.Errorf("unexpected Ingress request path: %s", request.URL.Path)
					http.NotFound(response, request)
					return
				}
				methods = append(methods, request.Method)
				switch request.Method {
				case http.MethodGet:
				case http.MethodPut:
					var updated operatorv1.Ingress
					if err := json.NewDecoder(request.Body).Decode(&updated); err != nil {
						t.Errorf("decode Ingress update: %v", err)
						response.WriteHeader(http.StatusBadRequest)
						return
					}
					if updated.APIVersion != "operator.openshift.io/v1" || updated.Kind != "Ingress" {
						t.Errorf("unexpected update GVK: %s %s", updated.APIVersion, updated.Kind)
					}
					if updated.Name != ingressCRName || updated.ResourceVersion != ingress.ResourceVersion || updated.Spec.GatewayAPI.ManagementMode != testCase.mode {
						t.Errorf("unexpected Ingress update: %#v", updated)
					}
					if conflict {
						conflict = false
						ingress.ResourceVersion = "2"
						response.WriteHeader(http.StatusConflict)
						if err := json.NewEncoder(response).Encode(&metav1.Status{Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict, Code: http.StatusConflict}); err != nil {
							t.Errorf("encode conflict: %v", err)
						}
						return
					}
					ingress = updated
					condition := metav1.Condition{Type: "GatewayAPICRDsManaged", Status: metav1.ConditionFalse, Reason: "Unmanaged"}
					if testCase.mode == operatorv1.GatewayAPIManagementModeManaged {
						condition.Status = metav1.ConditionTrue
						condition.Reason = "Managed"
					}
					ingress.Status.Conditions = []metav1.Condition{condition}
				default:
					t.Errorf("unexpected request method: %s", request.Method)
					response.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if err := json.NewEncoder(response).Encode(&ingress); err != nil {
					t.Errorf("encode Ingress response: %v", err)
				}
			}))
			defer server.Close()

			configPath := filepath.Join(t.TempDir(), "kubeconfig")
			config := clientcmdapi.Config{
				Clusters:       map[string]*clientcmdapi.Cluster{"test": {Server: server.URL}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"test": {}},
				Contexts:       map[string]*clientcmdapi.Context{"test": {Cluster: "test", AuthInfo: "test"}},
				CurrentContext: "test",
			}
			if err := clientcmd.WriteToFile(config, configPath); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KUBECONFIG", configPath)
			t.Setenv("STATIC_CONFIG_MANIFEST_DIR", "")
			oc := exutil.NewCLIForMonitorTest("ingress-management-mode")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			current, err := getIngressCR(ctx, oc)
			if err != nil {
				t.Fatal(err)
			}
			if current.Spec.GatewayAPI.ManagementMode != initialMode {
				t.Fatalf("unexpected initial management mode: %s", current.Spec.GatewayAPI.ManagementMode)
			}
			if err := setManagementMode(ctx, oc, testCase.mode); err != nil {
				t.Fatal(err)
			}
			if err := waitForManagementModeTransition(ctx, oc, testCase.mode, time.Second); err != nil {
				t.Fatal(err)
			}
			expectedMethods := []string{http.MethodGet, http.MethodGet, http.MethodPut, http.MethodGet}
			if testCase.conflict {
				expectedMethods = []string{http.MethodGet, http.MethodGet, http.MethodPut, http.MethodGet, http.MethodPut, http.MethodGet}
			}
			lock.Lock()
			actualMethods := slices.Clone(methods)
			lock.Unlock()
			if !slices.Equal(actualMethods, expectedMethods) {
				t.Errorf("unexpected request sequence: got %v, want %v", actualMethods, expectedMethods)
			}
		})
	}
}

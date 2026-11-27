package psalabelchecker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestGenerateTestCase_WarningHandlerEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces":
			w.Header().Add("Warning", `299 - "list namespaces deprecation noise"`)
			_, _ = w.Write([]byte(`{
				"kind": "NamespaceList",
				"apiVersion": "v1",
				"items": [{
					"metadata": {"name": "openshift-test-ns"}
				}]
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/openshift-test-ns/pods":
			w.Header().Add("Warning", `299 - "list pods noise"`)
			_, _ = w.Write([]byte(`{
				"kind": "PodList",
				"apiVersion": "v1",
				"items": [{"metadata": {"name": "workload-pod", "namespace": "openshift-test-ns"}}]
			}`))

		// Dry-run PUT on the namespace emits PSA enforcement warnings
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/namespaces/openshift-test-ns":
			require.Equal(t, []string{"All"}, r.URL.Query()["dryRun"])
			w.Header().Add("Warning", `299 - "would violate PodSecurity \"restricted:latest\": privileged (container \"c\" must not set securityContext.privileged=true)"`)
			_, _ = w.Write([]byte(`{
				"kind": "Namespace",
				"apiVersion": "v1",
				"metadata": {"name": "openshift-test-ns"}
			}`))

		// exutil.IsHypershift probes infrastructures/cluster; returning 404 defaults isHyperShift to false
		case r.Method == http.MethodGet && r.URL.Path == "/apis/config.openshift.io/v1/infrastructures/cluster":
			http.NotFound(w, r)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	checker := &noPodSecurityAdmissionLabelNamespaceChecker{}
	err := checker.StartCollection(context.Background(), &rest.Config{Host: server.URL}, nil)
	require.NoError(t, err)

	_, junits, err := checker.CollectData(context.Background(), "", time.Now(), time.Now())
	require.NoError(t, err)
	require.NotEmpty(t, junits)
	require.NotNil(t, junits[0].FailureOutput)

	failureOut := junits[0].FailureOutput.Output
	require.Contains(t, failureOut, `[URGENT] dry-run of "restricted" enforcement on namespace openshift-test-ns reported:`)
	require.Contains(t, failureOut, `would violate PodSecurity "restricted:latest"`)
	require.NotContains(t, failureOut, `list namespaces deprecation noise`)
	require.NotContains(t, failureOut, `list pods noise`)
}

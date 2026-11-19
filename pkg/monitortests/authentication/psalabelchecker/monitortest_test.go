package psalabelchecker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestWarningCollector_IsolationAndPop(t *testing.T) {
	runID := uuid.New().String()
	otherRunID := uuid.New().String()
	collector := &warningCollector{runID: runID}

	// 1. Untagged context (e.g., List calls) -> ignored
	collector.HandleWarningHeaderWithContext(context.Background(), 299, "-", "untagged warning")

	// 2. Wrong runID (e.g., leaked context from another collector instance) -> ignored
	wrongRunCtx := context.WithValue(context.Background(), nsKey{}, requestTag{
		runID:     otherRunID,
		namespace: "openshift-foo",
	})
	collector.HandleWarningHeaderWithContext(wrongRunCtx, 299, "-", "wrong run warning")

	// 3. Empty namespace -> ignored
	emptyNSCtx := context.WithValue(context.Background(), nsKey{}, requestTag{
		runID:     runID,
		namespace: "",
	})
	collector.HandleWarningHeaderWithContext(emptyNSCtx, 299, "-", "empty ns warning")

	// 4. Valid tags for two different namespaces
	nsACtx := context.WithValue(context.Background(), nsKey{}, requestTag{
		runID:     runID,
		namespace: "openshift-a",
	})
	nsBCtx := context.WithValue(context.Background(), nsKey{}, requestTag{
		runID:     runID,
		namespace: "openshift-b",
	})

	collector.HandleWarningHeaderWithContext(nsACtx, 299, "-", "warning A1")
	collector.HandleWarningHeaderWithContext(nsBCtx, 299, "-", "warning B1")
	collector.HandleWarningHeaderWithContext(nsACtx, 299, "-", "warning A2")

	// Popping openshift-a must return A1 & A2, leave B1 intact, and drain A
	gotA := collector.PopWarnings("openshift-a")
	require.Equal(t, []string{"warning A1", "warning A2"}, gotA)
	require.Empty(t, collector.PopWarnings("openshift-a"))

	// openshift-b must still have its warning
	gotB := collector.PopWarnings("openshift-b")
	require.Equal(t, []string{"warning B1"}, gotB)
	require.Empty(t, collector.PopWarnings("openshift-b"))
}

func TestWarningCollector_ConcurrentAccess(t *testing.T) {
	collector := &warningCollector{runID: uuid.New().String()}
	const workers = 20

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			ns := fmt.Sprintf("openshift-ns-%d", id)
			ctx := context.WithValue(context.Background(), nsKey{}, requestTag{
				runID:     collector.runID,
				namespace: ns,
			})
			collector.HandleWarningHeaderWithContext(ctx, 299, "-", "msg-"+ns)
			got := collector.PopWarnings(ns)
			require.Equal(t, []string{"msg-" + ns}, got)
		}(i)
	}
	wg.Wait()
}

func TestGenerateTestCase_WarningHandlerEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Simulate noisy warnings on List calls (untagged contexts) that must be ignored
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

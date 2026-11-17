package router

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/exec"
	"k8s.io/utils/ptr"
)

const replicaSensitive = "SENSITIVE_SENTINEL_password_host_command"

func TestReplicaRequestAccounting(test *testing.T) {
	test.Parallel()
	for _, testCase := range []struct {
		name     string
		code     int
		err      error
		category replicaFailure
		success  bool
	}{
		{name: "success", code: 200, success: true},
		{name: "redirect preserves success", code: 302, success: true},
		{name: "503", code: 503, category: replicaHTTP503},
		{name: "4xx", code: 404, category: replicaHTTP4xx},
		{name: "5xx", code: 500, category: replicaHTTP5xx},
		{name: "unknown HTTP", code: 700, category: replicaHTTPOther},
		{name: "empty response", category: replicaHTTPZero},
		{name: "exec", err: exec.CodeExitError{Err: errors.New(replicaSensitive), Code: 1}, category: replicaExec},
		{name: "timeout", err: fmt.Errorf("%s: %w", replicaSensitive, context.DeadlineExceeded), category: replicaTimeout},
		{name: "API timeout", err: apierrors.NewTimeoutError(replicaSensitive, 1), category: replicaTimeout},
		{name: "wrapped API timeout", err: fmt.Errorf("%s: %w", replicaSensitive, apierrors.NewServerTimeout(schema.GroupResource{Resource: replicaSensitive}, replicaSensitive, 1)), category: replicaTimeout},
		{name: "canceled", err: context.Canceled, category: replicaCanceled},
		{name: "parse", err: &strconv.NumError{Func: replicaSensitive, Num: replicaSensitive, Err: strconv.ErrSyntax}, category: replicaParse},
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Resource: replicaSensitive}, replicaSensitive, errors.New(replicaSensitive)), category: replicaForbidden},
		{name: "unauthorized", err: apierrors.NewUnauthorized(replicaSensitive), category: replicaForbidden},
		{name: "not found", err: apierrors.NewNotFound(schema.GroupResource{Resource: replicaSensitive}, replicaSensitive), category: replicaNotFound},
		{name: "unknown", err: errors.New(replicaSensitive), category: replicaOther},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			stats := replicaRequestStats{}
			output, err := replicaReadURLResult(testCase.code, replicaSensitive, testCase.err, &stats)
			if stats.attempts != 1 || stats.successes != boolReplicaCount(testCase.success) || (err == nil) != testCase.success {
				test.Fatalf("accounting mismatch: attempts=%d successes=%d error_present=%test", stats.attempts, stats.successes, err != nil)
			}
			if testCase.success && output != replicaSensitive {
				test.Fatal("successful response changed")
			}
			if testCase.err != nil && err != testCase.err {
				test.Fatal("original error changed")
			}
			if !testCase.success && stats.failures[testCase.category] != 1 {
				test.Fatal("incorrect failure classification")
			}
			plainOutput, plainErr := replicaReadURLResult(testCase.code, replicaSensitive, testCase.err, nil)
			if plainOutput != output || (plainErr == nil) != (err == nil) {
				test.Fatal("uninstrumented caller behavior changed")
			}
		})
	}
	stats := replicaRequestStats{}
	stats.record(200, nil)
	stats.record(503, nil)
	stats.record(0, errors.New(replicaSensitive))
	if stats.attempts != 3 || stats.successes != 1 || stats.failures[replicaHTTP503] != 1 || stats.failures[replicaOther] != 1 {
		test.Fatal("mixed accounting mismatch")
	}
}

func boolReplicaCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func replicaTestSlice() *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: replicaSensitive, Namespace: "test", Labels: map[string]string{discoveryv1.LabelServiceName: "service"}},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{replicaSensitive}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true), Serving: ptr.To(true)}},
			{Addresses: []string{replicaSensitive}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false), Terminating: ptr.To(true)}},
			{Addresses: []string{replicaSensitive}},
		},
	}
}

func replicaTestPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", Labels: map[string]string{replicaSensitive: replicaSensitive}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: replicaSensitive, Image: replicaSensitive}}},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, Message: replicaSensitive}},
			ContainerStatuses: []corev1.ContainerStatus{{Name: replicaSensitive, Ready: true, RestartCount: 2, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
		},
	}
}

func TestReplicaEndpointReadinessAndScope(test *testing.T) {
	test.Parallel()
	client := fake.NewSimpleClientset(replicaTestSlice())
	counts := collectReplicaEndpoints(context.Background(), client, "test", "service")
	if counts.slices != 1 || counts.ready != 1 || counts.notReady != 1 || counts.unknown != 1 || counts.serving != 1 || counts.terminating != 1 || !counts.incomplete || counts.truncated {
		test.Fatalf("unexpected readiness counts: %+v", counts)
	}
	action := client.Actions()[0].(clienttesting.ListAction)
	if action.GetNamespace() != "test" || action.GetListRestrictions().Labels.String() != discoveryv1.LabelServiceName+"=service" {
		test.Fatal("collection escaped namespace or single-service scope")
	}
	client.PrependReactor("list", "endpointslices", func(clienttesting.Action) (bool, runtime.Object, error) {
		wrong := replicaTestSlice()
		wrong.Namespace = replicaSensitive
		return true, &discoveryv1.EndpointSliceList{Items: []discoveryv1.EndpointSlice{*wrong}}, nil
	})
	counts = collectReplicaEndpoints(context.Background(), client, "test", "service")
	if counts.endpoints != 0 || !counts.incomplete {
		test.Fatal("out-of-scope endpoints were counted")
	}
}

func TestReplicaCollectionBounds(test *testing.T) {
	test.Parallel()
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "endpointslices", func(clienttesting.Action) (bool, runtime.Object, error) {
		slices := &discoveryv1.EndpointSliceList{ListMeta: metav1.ListMeta{Continue: replicaSensitive}}
		for range replicaMaxSlices + 1 {
			slice := replicaTestSlice()
			slice.Endpoints = make([]discoveryv1.Endpoint, replicaMaxEndpoints+1)
			slices.Items = append(slices.Items, *slice)
		}
		return true, slices, nil
	})
	counts := collectReplicaEndpoints(context.Background(), client, "test", "service")
	if counts.slices != replicaMaxSlices || counts.endpoints != replicaMaxEndpoints || !counts.truncated || !counts.incomplete {
		test.Fatal("endpoint collection is not bounded")
	}
	pod := replicaTestPod("router-haproxy-cfgmgr")
	pod.Status.ContainerStatuses = make([]corev1.ContainerStatus, replicaMaxContainers+1)
	client = fake.NewSimpleClientset(pod)
	pods := collectReplicaPod(context.Background(), client, "test", pod.Name)
	if pods.containers != replicaMaxContainers || !pods.truncated || !pods.incomplete {
		test.Fatal("container collection is not bounded")
	}
	output := boundReplicaOutput(strings.Repeat("0", replicaMaxOutput+1))
	if len(output) != replicaMaxOutput || !strings.HasSuffix(output, "incomplete=true output_truncated=true\n") {
		test.Fatal("output bound or truncation marker missing")
	}
}

func TestReplicaFailureOnlyBeforeCleanupAndSuppression(test *testing.T) {
	test.Parallel()
	router := replicaTestPod(configManagerRouterPodName)
	router.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: replicaSensitive, Message: replicaSensitive}}
	execPod := replicaTestPod(replicaSensitive)
	execPod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: replicaSensitive, Message: replicaSensitive, ExitCode: 1}}
	client := fake.NewSimpleClientset(replicaTestSlice(), router, execPod)
	emitted := false
	reportReplicaFailure(context.Background(), nil, replicaRequestStats{}, client, nil, "test", "service", replicaSensitive, func(string) { emitted = true })
	if emitted || len(client.Actions()) != 0 {
		test.Fatal("success triggered diagnostics")
	}
	stats := replicaRequestStats{iteration: 3, expected: 3, observed: 2, attempts: 4, successes: 2}
	stats.failures[replicaHTTP503] = 1
	stats.failures[replicaExec] = 1
	cleaned := false
	func() {
		defer func() { cleaned = true }()
		reportReplicaFailure(context.Background(), errors.New(replicaSensitive), stats, client, nil, "test", "service", replicaSensitive, func(output string) {
			emitted = true
			if cleaned || len(client.Actions()) != 3 {
				test.Fatal("collection did not complete before cleanup")
			}
			for _, expected := range []string{"iteration=3 expected=3 observed=2 attempts=4 successes=2", "request_http_503=1", "request_exec=1", "ready=1 not_ready=1 unknown=1", "restarts=2", "waiting=1", "terminated=1", "pod role=router", "pod role=exec", "output_truncated=false"} {
				if !strings.Contains(output, expected) {
					test.Fatalf("missing fixed diagnostic: %s", expected)
				}
			}
			if len(output) > replicaMaxOutput || strings.Contains(output, replicaSensitive) || strings.Contains(output, "router-haproxy-cfgmgr") || strings.Contains(output, "service") {
				test.Fatal("diagnostic output exceeded allowlist or bound")
			}
		})
	}()
	if !emitted || !cleaned {
		test.Fatal("failure collection or cleanup did not run")
	}
}

func TestReplicaObservedBackends(test *testing.T) {
	test.Parallel()
	stats := replicaRequestStats{expected: 2}
	expected := sets.New("first", "second")
	observed := sets.New[string]()
	for _, testCase := range []struct {
		response   string
		count      int
		done, fail bool
	}{
		{response: "first", count: 1},
		{response: "first", count: 1},
		{response: "second", count: 2, done: true},
		{response: replicaSensitive, count: 2, fail: true},
	} {
		done, err := stats.observeBackend(testCase.response, expected, observed)
		if done != testCase.done || (err != nil) != testCase.fail || stats.observed != testCase.count {
			test.Fatal("unique-response accounting or original condition result changed")
		}
	}
	if stats.unexpected != 1 {
		test.Fatal("unexpected response not counted")
	}
}

func TestReplicaFailureIntegrationOrder(test *testing.T) {
	test.Parallel()
	source, err := parser.ParseFile(token.NewFileSet(), "config_manager.go", nil, 0)
	if err != nil {
		test.Fatal(err)
	}
	var cleanup, report, assertion token.Pos
	ast.Inspect(source, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "It" {
			return true
		}
		name, ok := call.Args[0].(*ast.BasicLit)
		if !ok || name.Value != strconv.Quote("should add replicas beyond the number of empty slots per backend") {
			return true
		}
		body := call.Args[1].(*ast.FuncLit).Body
		for _, statement := range body.List {
			if deferred, ok := statement.(*ast.DeferStmt); ok {
				if identifier, ok := deferred.Call.Fun.(*ast.Ident); ok && identifier.Name == "doneExecPod" {
					cleanup = deferred.Pos()
				}
			}
		}
		ast.Inspect(body, func(node ast.Node) bool {
			block, ok := node.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for statementIndex, statement := range block.List {
				expression, ok := statement.(*ast.ExprStmt)
				if !ok {
					continue
				}
				call, ok := expression.X.(*ast.CallExpr)
				if !ok {
					continue
				}
				identifier, ok := call.Fun.(*ast.Ident)
				if !ok || identifier.Name != "reportReplicaFailure" {
					continue
				}
				report = call.Pos()
				if statementIndex+1 < len(block.List) {
					ast.Inspect(block.List[statementIndex+1], func(node ast.Node) bool {
						if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Expect" {
							assertion = selector.Pos()
						}
						return true
					})
				}
			}
			return true
		})
		return false
	})
	if cleanup == 0 || report <= cleanup || assertion <= report {
		test.Fatal("diagnostics must run synchronously before the failure assertion unwinds deferred exec cleanup")
	}
}

func TestReplicaPodLifecycle(test *testing.T) {
	test.Parallel()
	for _, phase := range []corev1.PodPhase{corev1.PodPending, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed, corev1.PodPhase(replicaSensitive)} {
		test.Run(string(phase), func(test *testing.T) {
			test.Parallel()
			pod := replicaTestPod(configManagerRouterPodName)
			pod.Status.Phase = phase
			pod.Status.Conditions[0].Status = corev1.ConditionFalse
			now := metav1.Now()
			pod.DeletionTimestamp = &now
			pod.Spec.InitContainers = []corev1.Container{{Name: replicaSensitive}}
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{RestartCount: 3, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Message: replicaSensitive}}}}
			pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Message: replicaSensitive}}
			counts := collectReplicaPod(context.Background(), fake.NewSimpleClientset(pod), "test", pod.Name)
			phaseCounts := map[corev1.PodPhase]int{corev1.PodPending: counts.pending, corev1.PodRunning: counts.running, corev1.PodSucceeded: counts.succeeded, corev1.PodFailed: counts.failed, corev1.PodPhase(replicaSensitive): counts.unknown}
			if phaseCounts[phase] != 1 || !counts.present || !counts.deleting || !counts.readyKnown || counts.ready || counts.containers != 2 || counts.restarts != 5 || counts.waiting != 1 || counts.terminated != 1 || counts.incomplete != (phase == corev1.PodPhase(replicaSensitive)) {
				test.Fatalf("incorrect lifecycle counts: %+v", counts)
			}
			pod.Status.Conditions[0].Status = corev1.ConditionUnknown
			counts = collectReplicaPod(context.Background(), fake.NewSimpleClientset(pod), "test", pod.Name)
			if counts.readyKnown || !counts.incomplete {
				test.Fatal("unknown readiness not represented")
			}
		})
	}
}

func TestReplicaEndpointBoundsIndependently(test *testing.T) {
	test.Parallel()
	for _, testCase := range []struct {
		name      string
		list      discoveryv1.EndpointSliceList
		truncated bool
	}{
		{name: "continue", list: discoveryv1.EndpointSliceList{ListMeta: metav1.ListMeta{Continue: replicaSensitive}}, truncated: true},
		{name: "remaining", list: discoveryv1.EndpointSliceList{ListMeta: metav1.ListMeta{RemainingItemCount: ptr.To(int64(1))}}, truncated: true},
		{name: "slice overflow", list: discoveryv1.EndpointSliceList{Items: make([]discoveryv1.EndpointSlice, replicaMaxSlices+1)}, truncated: true},
		{name: "exact endpoint limit", list: discoveryv1.EndpointSliceList{Items: []discoveryv1.EndpointSlice{*replicaTestSlice()}}},
		{name: "endpoint overflow", list: discoveryv1.EndpointSliceList{Items: []discoveryv1.EndpointSlice{*replicaTestSlice()}}, truncated: true},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			if testCase.name == "slice overflow" {
				for sliceIndex := range testCase.list.Items {
					testCase.list.Items[sliceIndex].ObjectMeta = replicaTestSlice().ObjectMeta
				}
			}
			if len(testCase.list.Items) == 1 {
				total := replicaMaxEndpoints
				if testCase.truncated {
					total++
				}
				testCase.list.Items[0].Endpoints = make([]discoveryv1.Endpoint, total)
				for endpointIndex := range testCase.list.Items[0].Endpoints {
					testCase.list.Items[0].Endpoints[endpointIndex].Conditions.Ready = ptr.To(true)
				}
			}
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", "endpointslices", func(clienttesting.Action) (bool, runtime.Object, error) { return true, &testCase.list, nil })
			counts := collectReplicaEndpoints(context.Background(), client, "test", "service")
			if counts.truncated != testCase.truncated || (len(testCase.list.Items) == 1 && counts.incomplete != testCase.truncated) {
				test.Fatal("independent truncation boundary mismatch")
			}
		})
	}
}

type replicaRoundTripper func(*http.Request) (*http.Response, error)

func (transport replicaRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestReplicaInternalRequestDeadlines(test *testing.T) {
	test.Parallel()
	requests := 0
	client, err := kubernetes.NewForConfig(&rest.Config{Host: "http://synthetic.invalid", Transport: replicaRoundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		deadline, present := request.Context().Deadline()
		remaining := time.Until(deadline)
		if !present || remaining > replicaRequestTimeout || remaining < replicaRequestTimeout-time.Second {
			test.Error("internal per-request deadline missing")
		}
		return &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`))}, nil
	})})
	if err != nil {
		test.Fatal(err)
	}
	reportReplicaFailure(context.Background(), errors.New(replicaSensitive), replicaRequestStats{}, client, nil, "test", "service", replicaSensitive, func(string) {})
	if requests != 3 {
		test.Fatal("expected EndpointSlice and two pod requests")
	}
}

func TestReplicaLookupErrorsSuppressDetails(test *testing.T) {
	test.Parallel()
	client := fake.NewSimpleClientset()
	client.PrependReactor("*", "*", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: replicaSensitive}, replicaSensitive, errors.New(replicaSensitive))
	})
	reportReplicaFailure(context.Background(), context.DeadlineExceeded, replicaRequestStats{}, client, nil, "test", "service", replicaSensitive, func(output string) {
		if strings.Contains(output, replicaSensitive) || strings.Count(output, "lookup_failed=true error=forbidden") != 3 || !strings.Contains(output, "incomplete=true") {
			test.Fatal("error details escaped fixed categories")
		}
	})
}

func TestReplicaCollectionDeadlineAndRequestScope(test *testing.T) {
	test.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/apis/discovery.k8s.io/v1/namespaces/test/endpointslices") {
			<-request.Context().Done()
			return
		}
		if request.URL.Query().Get("limit") != strconv.Itoa(replicaMaxSlices) || request.URL.Query().Get("labelSelector") != discoveryv1.LabelServiceName+"=service" {
			test.Error("request lacks service selector or object limit")
		}
		<-request.Context().Done()
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		test.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	reportReplicaFailure(ctx, context.DeadlineExceeded, replicaRequestStats{}, client, nil, "test", "service", replicaSensitive, func(output string) {
		if !strings.Contains(output, "incomplete=true") || !strings.Contains(output, "lookup_failed=true error=timeout") {
			test.Fatal("deadline not represented")
		}
	})
	if time.Since(start) > time.Second {
		test.Fatal("collection exceeded shared caller deadline")
	}
}

// replicaStateHeader is the commented column header HAProxy emits ahead of the server
// rows of "show servers state".
const replicaStateHeader = "# be_id be_name srv_id srv_name srv_addr srv_op_state srv_admin_state srv_uweight srv_iweight srv_time_since_last_change srv_check_status srv_check_result"

// replicaStateRowText builds one "show servers state" row in the documented column order.
func replicaStateRowText(backend, server, op, admin, weight, check string) string {
	return replicaStateRowAt(backend, server, "10.0.0.1", op, admin, weight, check)
}

// replicaStateRowAt builds a row with an explicit server address.
func replicaStateRowAt(backend, server, address, op, admin, weight, check string) string {
	return strings.Join([]string{"3", backend, "1", server, address, op, admin, weight, "1", "7", "15", check}, " ")
}

func replicaStateDump(rows ...string) string {
	return "1\n" + replicaStateHeader + "\n" + strings.Join(rows, "\n") + "\n"
}

var replicaStateNames = []string{"replica-0", "replica-1", "replica-2"}

func TestReplicaServerStateParsing(test *testing.T) {
	test.Parallel()
	healthy := func(ordinal string) string {
		return replicaStateRowText("be_http:test:route", "pod:replica-"+ordinal+":svc::10.0.0.1:9376", "2", "0", "1", "3")
	}
	for _, testCase := range []struct {
		name                     string
		output                   string
		names                    []string
		addresses                map[string]int
		backendFound, incomplete bool
		truncated, extraCapped   bool
		extra, missing           int
		ops                      []replicaServerOp
		admins                   []replicaServerAdmin
		checks                   []replicaServerCheck
		weightZero               []bool
		duplicate                []bool
	}{
		{
			name: "all servers healthy", output: replicaStateDump(healthy("0"), healthy("1"), healthy("2")), names: replicaStateNames,
			backendFound: true,
			ops:          []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpUp},
			admins:       []replicaServerAdmin{replicaAdminNone, replicaAdminNone, replicaAdminNone},
			checks:       []replicaServerCheck{replicaCheckPassed, replicaCheckPassed, replicaCheckPassed},
		},
		{
			name: "distinguishes down maintenance and drain",
			output: replicaStateDump(
				replicaStateRowText("be_http:test:route", "pod:replica-0:svc::10.0.0.1:9376", "0", "0", "1", "2"),
				replicaStateRowText("be_http:test:route", "pod:replica-1:svc::10.0.0.2:9376", "2", "1", "1", "3"),
				replicaStateRowText("be_http:test:route", "pod:replica-2:svc::10.0.0.3:9376", "2", "8", "1", "3"),
			),
			names: replicaStateNames, backendFound: true,
			ops:    []replicaServerOp{replicaOpDown, replicaOpUp, replicaOpUp},
			admins: []replicaServerAdmin{replicaAdminNone, replicaAdminMaint, replicaAdminDrain},
			checks: []replicaServerCheck{replicaCheckFailed, replicaCheckPassed, replicaCheckPassed},
		},
		{
			name: "zero weight is distinguished from healthy",
			output: replicaStateDump(healthy("0"), healthy("1"),
				replicaStateRowText("be_http:test:route", "pod:replica-2:svc::10.0.0.3:9376", "2", "0", "0", "3")),
			names: replicaStateNames, backendFound: true,
			ops:        []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpUp},
			admins:     []replicaServerAdmin{replicaAdminNone, replicaAdminNone, replicaAdminNone},
			checks:     []replicaServerCheck{replicaCheckPassed, replicaCheckPassed, replicaCheckPassed},
			weightZero: []bool{false, false, true},
		},
		{
			name:   "missing server row is reported as absent",
			output: replicaStateDump(healthy("0"), healthy("2")), names: replicaStateNames,
			backendFound: true, incomplete: true, missing: 1,
			ops:    []replicaServerOp{replicaOpUp, replicaOpAbsent, replicaOpUp},
			admins: []replicaServerAdmin{replicaAdminNone, replicaAdminAbsent, replicaAdminNone},
			checks: []replicaServerCheck{replicaCheckPassed, replicaCheckUnknown, replicaCheckPassed},
		},
		{
			name: "unrelated backends are ignored",
			output: replicaStateDump(
				replicaStateRowText("be_http:test:other", "pod:replica-0:svc::10.0.0.9:9376", "0", "1", "0", "2"),
				replicaStateRowText("be_tcp:other:route", "pod:unrelated:svc::10.0.0.8:9376", "0", "1", "0", "2"),
				healthy("0"), healthy("1"), healthy("2"),
			),
			names: replicaStateNames, backendFound: true,
			ops:    []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpUp},
			admins: []replicaServerAdmin{replicaAdminNone, replicaAdminNone, replicaAdminNone},
			checks: []replicaServerCheck{replicaCheckPassed, replicaCheckPassed, replicaCheckPassed},
		},
		{
			name: "unmatched servers in the target backend are counted only",
			output: replicaStateDump(healthy("0"), healthy("1"), healthy("2"),
				replicaStateRowText("be_http:test:route", "pod:"+replicaSensitive+":svc::10.0.0.7:9376", "2", "0", "1", "3")),
			names: replicaStateNames, backendFound: true, extra: 1,
			ops:    []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpUp},
			admins: []replicaServerAdmin{replicaAdminNone, replicaAdminNone, replicaAdminNone},
			checks: []replicaServerCheck{replicaCheckPassed, replicaCheckPassed, replicaCheckPassed},
		},
		{
			name:   "duplicate rows are flagged not double counted",
			output: replicaStateDump(healthy("0"), healthy("0"), healthy("1"), healthy("2")),
			names:  replicaStateNames, backendFound: true, incomplete: true,
			ops:       []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpUp},
			admins:    []replicaServerAdmin{replicaAdminNone, replicaAdminNone, replicaAdminNone},
			checks:    []replicaServerCheck{replicaCheckPassed, replicaCheckPassed, replicaCheckPassed},
			duplicate: []bool{true, false, false},
		},
		{
			name: "unknown enum values are bucketed not guessed",
			output: replicaStateDump(
				replicaStateRowText("be_http:test:route", "pod:replica-0:svc::10.0.0.1:9376", "9", "bogus", "x", "77"),
				healthy("1"), healthy("2")),
			names: replicaStateNames, backendFound: true, incomplete: true,
			ops:    []replicaServerOp{replicaOpUnknown, replicaOpUp, replicaOpUp},
			admins: []replicaServerAdmin{replicaAdminUnknown, replicaAdminNone, replicaAdminNone},
			checks: []replicaServerCheck{replicaCheckInvalid, replicaCheckPassed, replicaCheckPassed},
		},
		{
			name: "reordered header columns are followed by name",
			output: "1\n# srv_check_result srv_uweight srv_admin_state srv_op_state srv_addr srv_name be_name be_id\n" +
				strings.Join([]string{"3", "1", "0", "2", "10.0.0.1", "pod:replica-0:svc", "be_http:test:route", "3"}, " ") + "\n",
			names: []string{"replica-0"}, backendFound: true,
			ops:    []replicaServerOp{replicaOpUp},
			admins: []replicaServerAdmin{replicaAdminNone},
			checks: []replicaServerCheck{replicaCheckPassed},
		},
		{
			name:       "header without a required column yields no attribution",
			output:     "1\n# be_id be_name srv_id srv_name\n3 be_http:test:route 1 pod:replica-0:svc\n",
			names:      replicaStateNames,
			incomplete: true, missing: 3,
			ops:    []replicaServerOp{replicaOpAbsent, replicaOpAbsent, replicaOpAbsent},
			admins: []replicaServerAdmin{replicaAdminAbsent, replicaAdminAbsent, replicaAdminAbsent},
			checks: []replicaServerCheck{replicaCheckUnknown, replicaCheckUnknown, replicaCheckUnknown},
		},
		{
			name:         "row cut short by the byte cap is not attributed",
			output:       replicaStateDump(healthy("0"), healthy("1")) + "3 be_http:test:route 1 pod:replica-2",
			names:        replicaStateNames,
			backendFound: true, incomplete: true, missing: 1,
			ops:    []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpAbsent},
			admins: []replicaServerAdmin{replicaAdminNone, replicaAdminNone, replicaAdminAbsent},
			checks: []replicaServerCheck{replicaCheckPassed, replicaCheckPassed, replicaCheckUnknown},
		},
		{
			name: "output without a header is incomplete", output: "garbage\nmore garbage\n", names: replicaStateNames,
			incomplete: true, missing: 3,
			ops:    []replicaServerOp{replicaOpAbsent, replicaOpAbsent, replicaOpAbsent},
			admins: []replicaServerAdmin{replicaAdminAbsent, replicaAdminAbsent, replicaAdminAbsent},
			checks: []replicaServerCheck{replicaCheckUnknown, replicaCheckUnknown, replicaCheckUnknown},
		},
		{
			name: "empty output is incomplete", output: "", names: replicaStateNames, incomplete: true, missing: 3,
			ops:    []replicaServerOp{replicaOpAbsent, replicaOpAbsent, replicaOpAbsent},
			admins: []replicaServerAdmin{replicaAdminAbsent, replicaAdminAbsent, replicaAdminAbsent},
			checks: []replicaServerCheck{replicaCheckUnknown, replicaCheckUnknown, replicaCheckUnknown},
		},
		{
			name:   "no expected server present means the backend is not identified",
			output: replicaStateDump(replicaStateRowText("be_http:test:route", "pod:unrelated:svc::10.0.0.6:9376", "2", "0", "1", "3")),
			names:  replicaStateNames, incomplete: true, missing: 3,
			ops:    []replicaServerOp{replicaOpAbsent, replicaOpAbsent, replicaOpAbsent},
			admins: []replicaServerAdmin{replicaAdminAbsent, replicaAdminAbsent, replicaAdminAbsent},
			checks: []replicaServerCheck{replicaCheckUnknown, replicaCheckUnknown, replicaCheckUnknown},
		},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			state := parseReplicaServerState(testCase.output, testCase.names, testCase.addresses)
			if !state.collected {
				test.Fatal("parse did not report collection")
			}
			if state.backendFound != testCase.backendFound || state.incomplete != testCase.incomplete ||
				state.truncated != testCase.truncated || state.extraCapped != testCase.extraCapped ||
				state.extra != testCase.extra || state.missing != testCase.missing {
				test.Fatalf("summary mismatch: backend_found=%t incomplete=%t truncated=%t extra_capped=%t extra=%d missing=%d",
					state.backendFound, state.incomplete, state.truncated, state.extraCapped, state.extra, state.missing)
			}
			if len(state.servers) != len(testCase.names) {
				test.Fatalf("expected %d ordinals, got %d", len(testCase.names), len(state.servers))
			}
			for ordinal, server := range state.servers {
				if server.op != testCase.ops[ordinal] || server.admin != testCase.admins[ordinal] || server.check != testCase.checks[ordinal] {
					test.Fatalf("ordinal %d category mismatch: op=%s admin=%s check=%s", ordinal,
						replicaServerOpNames[server.op], replicaServerAdminNames[server.admin], replicaServerCheckNames[server.check])
				}
				if testCase.weightZero != nil && server.weightZero != testCase.weightZero[ordinal] {
					test.Fatalf("ordinal %d weight mismatch", ordinal)
				}
				if testCase.duplicate != nil && server.duplicate != testCase.duplicate[ordinal] {
					test.Fatalf("ordinal %d duplicate mismatch", ordinal)
				}
			}
		})
	}
}

func TestReplicaServerStateBounds(test *testing.T) {
	test.Parallel()
	rows := make([]string, 0, replicaMaxStateRows*2)
	for index := 0; index < replicaMaxStateRows*2; index++ {
		rows = append(rows, replicaStateRowText("be_http:test:route", fmt.Sprintf("pod:filler-%d:svc", index), "2", "0", "1", "3"))
	}
	state := parseReplicaServerState(replicaStateDump(rows...), replicaStateNames, nil)
	if state.lines > replicaMaxStateRows || !state.truncated || !state.incomplete {
		test.Fatalf("row cap not enforced: lines=%d truncated=%t", state.lines, state.truncated)
	}

	extras := make([]string, 0, replicaMaxServers*2)
	extras = append(extras, replicaStateRowText("be_http:test:route", "pod:replica-0:svc", "2", "0", "1", "3"))
	for index := 0; index < replicaMaxServers*2; index++ {
		extras = append(extras, replicaStateRowText("be_http:test:route", fmt.Sprintf("pod:extra-%d:svc", index), "2", "0", "1", "3"))
	}
	bounded := parseReplicaServerState(replicaStateDump(extras...), []string{"replica-0"}, nil)
	if bounded.extra > replicaMaxServers || !bounded.extraCapped || !bounded.incomplete {
		test.Fatalf("unmatched server count not bounded: extra=%d extra_capped=%t", bounded.extra, bounded.extraCapped)
	}
	if bounded.truncated {
		test.Fatal("saturating the unmatched counter must not claim the dump was truncated")
	}

	oversized := parseReplicaServerState(strings.Repeat("x", replicaMaxStateBytes+1), replicaStateNames, nil)
	if !oversized.truncated || !oversized.incomplete {
		test.Fatal("oversized input not marked truncated")
	}
}

func TestReplicaServerStateCollectionFailures(test *testing.T) {
	// Not parallel: these cases swap the package-level exec hook.
	for _, testCase := range []struct {
		name     string
		err      error
		category replicaFailure
	}{
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Resource: replicaSensitive}, replicaSensitive, errors.New(replicaSensitive)), category: replicaForbidden},
		{name: "timeout", err: fmt.Errorf("%s: %w", replicaSensitive, context.DeadlineExceeded), category: replicaTimeout},
		{name: "exec", err: exec.CodeExitError{Err: errors.New(replicaSensitive), Code: 126}, category: replicaExec},
		{name: "unknown", err: errors.New(replicaSensitive), category: replicaOther},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			restore := replicaRouterExec
			defer func() { replicaRouterExec = restore }()
			replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
				return replicaSensitive, testCase.err
			}
			state := collectReplicaServerState(context.Background(), &rest.Config{}, fake.NewSimpleClientset(), "test", replicaRequestStats{names: replicaStateNames}, replicaEndpointCounts{})
			if !state.collected || !state.failed || !state.incomplete || state.failure != testCase.category {
				test.Fatalf("failure not classified: failed=%t error=%s", state.failed, replicaFailureNames[state.failure])
			}
			if state.backendFound || state.missing != len(replicaStateNames) {
				test.Fatal("failed collection reported backend facts")
			}
		})
	}
}

func TestReplicaServerStateExecDeadlineAndScope(test *testing.T) {
	// Not parallel: these cases swap the package-level exec hook.
	restore := replicaRouterExec
	defer func() { replicaRouterExec = restore }()
	var gotNamespace string
	replicaRouterExec = func(_ *rest.Config, _ kubernetes.Interface, namespace string) (string, error) {
		gotNamespace = namespace
		return replicaStateDump(replicaStateRowText("be_http:test:route", "pod:replica-0:svc", "2", "0", "1", "3")), nil
	}
	state := collectReplicaServerState(context.Background(), &rest.Config{}, fake.NewSimpleClientset(), "test", replicaRequestStats{names: []string{"replica-0"}}, replicaEndpointCounts{})
	if !state.backendFound {
		test.Fatal("collection did not parse the backend")
	}
	if gotNamespace != "test" {
		test.Fatal("collection was not scoped to the test namespace")
	}
	command := strings.Join(replicaServerStateCommand(), " ")
	if !strings.Contains(command, replicaHAProxySocketPath) || !strings.Contains(command, strconv.Itoa(replicaMaxStateBytes)) || !strings.Contains(command, "2>/dev/null") {
		test.Fatal("command does not read the router socket under a byte cap with stderr discarded")
	}

	released := make(chan struct{})
	defer close(released)
	replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
		<-released
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	stalled := collectReplicaServerState(ctx, &rest.Config{}, fake.NewSimpleClientset(), "test", replicaRequestStats{names: replicaStateNames}, replicaEndpointCounts{})
	if time.Since(start) > replicaExecTimeout {
		test.Fatal("stalled exec outlived the caller deadline")
	}
	if !stalled.failed || stalled.failure != replicaTimeout {
		test.Fatalf("stalled exec not reported as a timeout: %s", replicaFailureNames[stalled.failure])
	}
}

func TestReplicaResponseOrdinalAccounting(test *testing.T) {
	test.Parallel()
	stats := replicaRequestStats{expected: 3, names: replicaStateNames}
	expected := sets.New(replicaStateNames...)
	observed := sets.New[string]()
	for _, response := range []string{"replica-0", "replica-0", "replica-2", "replica-1", "replica-0"} {
		if _, err := stats.observeBackend(response, expected, observed); err != nil {
			test.Fatal(err)
		}
	}
	for ordinal, want := range []int{3, 1, 1} {
		if stats.responsesFor(ordinal) != want {
			test.Fatalf("ordinal %d served %d responses, want %d", ordinal, stats.responsesFor(ordinal), want)
		}
	}
	if stats.responsesFor(-1) != 0 || stats.responsesFor(len(replicaStateNames)) != 0 {
		test.Fatal("out of range ordinal did not report zero")
	}
	// Callers that do not supply ordinals must keep the original behavior.
	plain := replicaRequestStats{expected: 1}
	plainObserved := sets.New[string]()
	if done, err := plain.observeBackend("replica-0", sets.New("replica-0"), plainObserved); !done || err != nil || plain.responses != nil {
		test.Fatal("unordered caller behavior changed")
	}
}

func TestReplicaServerStateReportedWithoutIdentifiers(test *testing.T) {
	// Not parallel: these cases swap the package-level exec hook.
	restore := replicaRouterExec
	defer func() { replicaRouterExec = restore }()
	// replica-1 is registered, up, unweighted and passing checks, yet served nothing: the
	// combination the request and endpoint counters alone cannot distinguish.
	replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
		return replicaStateDump(
			replicaStateRowText("be_http:test:"+replicaSensitive, "pod:replica-0:svc::10.0.0.1:9376", "2", "0", "1", "3"),
			replicaStateRowText("be_http:test:"+replicaSensitive, "pod:replica-1:svc::10.0.0.2:9376", "2", "0", "1", "3"),
			replicaStateRowText("be_http:test:"+replicaSensitive, "pod:replica-2:svc::10.0.0.3:9376", "2", "0", "1", "3"),
		), nil
	}
	stats := replicaRequestStats{iteration: 3, expected: 3, observed: 2, names: replicaStateNames, responses: []int{82, 0, 65}}
	client := fake.NewSimpleClientset(replicaTestSlice(), replicaTestPod(configManagerRouterPodName), replicaTestPod(replicaSensitive))
	emitted := false
	reportReplicaFailure(context.Background(), context.DeadlineExceeded, stats, client, &rest.Config{}, "test", "service", replicaSensitive, func(output string) {
		emitted = true
		for _, expected := range []string{
			"haproxy backend_found=true servers=3 extra=0 missing=0",
			"haproxy_server ordinal=0 present=true op=up admin=none check=passed weight_known=true weight_zero=false duplicate=false responses=82",
			"haproxy_server ordinal=1 present=true op=up admin=none check=passed weight_known=true weight_zero=false duplicate=false responses=0",
			"haproxy_server ordinal=2 present=true op=up admin=none check=passed weight_known=true weight_zero=false duplicate=false responses=65",
		} {
			if !strings.Contains(output, expected) {
				test.Fatalf("missing fixed diagnostic: %s", expected)
			}
		}
		for _, forbidden := range []string{replicaSensitive, "be_http", "10.0.0.", "replica-0", "pod:", replicaHAProxySocketPath, "socat", "show servers state", "9376"} {
			if strings.Contains(output, forbidden) {
				test.Fatalf("diagnostic output leaked %q", forbidden)
			}
		}
		if len(output) > replicaMaxOutput {
			test.Fatal("diagnostic output exceeded bound")
		}
	})
	if !emitted {
		test.Fatal("diagnostics were not emitted")
	}
}

func TestReplicaServerStateNeverAltersVerdict(test *testing.T) {
	// Not parallel: these cases swap the package-level exec hook.
	restore := replicaRouterExec
	defer func() { replicaRouterExec = restore }()
	replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
		panic("collection must not be reached without ordinals")
	}
	client := fake.NewSimpleClientset(replicaTestSlice(), replicaTestPod(configManagerRouterPodName), replicaTestPod(replicaSensitive))
	// Without ordinals there is nothing to attribute, so no exec is attempted at all.
	reportReplicaFailure(context.Background(), errors.New(replicaSensitive), replicaRequestStats{}, client, nil, "test", "service", replicaSensitive, func(output string) {
		if strings.Contains(output, "haproxy") {
			test.Fatal("state reported without ordinals")
		}
	})

	// A failing collection still emits the other diagnostics and leaves cleanup ordering
	// and the caller's failure untouched.
	replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
		return "", errors.New(replicaSensitive)
	}
	cleaned, emitted := false, false
	func() {
		defer func() { cleaned = true }()
		reportReplicaFailure(context.Background(), errors.New(replicaSensitive), replicaRequestStats{names: replicaStateNames}, client, &rest.Config{}, "test", "service", replicaSensitive, func(output string) {
			emitted = true
			if cleaned {
				test.Fatal("collection did not complete before cleanup")
			}
			if !strings.Contains(output, "collect_failed=true error=other") || !strings.Contains(output, "incomplete=true") {
				test.Fatal("collection failure not represented")
			}
			if strings.Contains(output, replicaSensitive) {
				test.Fatal("collection failure leaked error text")
			}
		})
	}()
	if !emitted || !cleaned {
		test.Fatal("failure collection or cleanup did not run")
	}
}

func TestReplicaServerStateMatchesDynamicSlotsByAddress(test *testing.T) {
	test.Parallel()
	// A server occupying a pre-provisioned dynamic slot keeps the slot's name, so only its
	// address identifies the endpoint behind it.
	output := replicaStateDump(
		replicaStateRowAt("be_http:test:route", "pod:replica-0:svc::10.0.0.1:9376", "10.0.0.1", "2", "0", "1", "3"),
		replicaStateRowAt("be_http:test:route", "_dynamic_pods-1", "10.0.0.2", "2", "0", "1", "3"),
		replicaStateRowAt("be_http:test:route", "_dynamic_pods-2", "10.0.0.3", "0", "0", "0", "2"),
	)
	addresses := map[string]int{"10.0.0.2": 1, "10.0.0.3": 2}
	state := parseReplicaServerState(output, replicaStateNames, addresses)
	if !state.backendFound || state.missing != 0 || state.extra != 0 {
		test.Fatalf("dynamic slots not attributed: backend_found=%t missing=%d extra=%d", state.backendFound, state.missing, state.extra)
	}
	for ordinal, want := range []replicaServerOp{replicaOpUp, replicaOpUp, replicaOpDown} {
		if state.servers[ordinal].op != want {
			test.Fatalf("ordinal %d op=%s", ordinal, replicaServerOpNames[state.servers[ordinal].op])
		}
	}
	if !state.servers[2].weightZero || state.servers[2].check != replicaCheckFailed {
		test.Fatal("dynamic slot health not reported")
	}
	// Without the address map the same dump cannot identify the dynamic slots.
	if blind := parseReplicaServerState(output, replicaStateNames, nil); blind.missing != 2 || !blind.incomplete {
		test.Fatalf("name-only matching should report the slots missing, got missing=%d", blind.missing)
	}
}

func TestReplicaAddressOrdinalsFromEndpoints(test *testing.T) {
	test.Parallel()
	endpoints := replicaEndpointCounts{addresses: map[string]string{
		"10.0.0.1": "replica-0", "10.0.0.3": "replica-2", "10.0.0.9": replicaSensitive,
	}}
	ordinals := replicaAddressOrdinals(replicaRequestStats{names: replicaStateNames}, endpoints)
	if len(ordinals) != 2 || ordinals["10.0.0.1"] != 0 || ordinals["10.0.0.3"] != 2 {
		test.Fatalf("address map mismatch: %v", ordinals)
	}
	if _, present := ordinals["10.0.0.9"]; present {
		test.Fatal("address of an unexpected pod retained")
	}
	if collected := collectReplicaEndpoints(context.Background(), fake.NewSimpleClientset(replicaTestSlice()), "test", "service"); collected.addresses == nil {
		test.Fatal("endpoint collection did not gather addresses")
	}
}

func TestReplicaServerStateAdditionalBounds(test *testing.T) {
	test.Parallel()
	// A second header would leave later rows reading the first header's offsets.
	doubled := replicaStateDump(replicaStateRowText("be_http:test:route", "pod:replica-0:svc", "2", "0", "1", "3")) +
		replicaStateHeader + "\n" + replicaStateRowText("be_http:test:route", "pod:replica-1:svc", "2", "0", "1", "3") + "\n"
	if state := parseReplicaServerState(doubled, replicaStateNames, nil); !state.incomplete {
		test.Fatal("a second header was not treated as a format change")
	}
	// Weight is compared as a number, not as raw text.
	for _, raw := range []string{"0", "+0", "00"} {
		state := parseReplicaServerState(replicaStateDump(replicaStateRowText("be_http:test:route", "pod:replica-0:svc", "2", "0", raw, "3")), []string{"replica-0"}, nil)
		if !state.servers[0].weightKnown || !state.servers[0].weightZero {
			test.Fatalf("weight %q not recognized as zero", raw)
		}
	}
	// Mismatched response accounting must not index out of range.
	stats := replicaRequestStats{names: replicaStateNames, responses: []int{0}}
	if _, err := stats.observeBackend("replica-2", sets.New(replicaStateNames...), sets.New[string]()); err != nil {
		test.Fatal(err)
	}
	if stats.responsesFor(2) != 1 {
		test.Fatal("response not recorded after the slice was resized")
	}
}

func TestReplicaServerStateEmittedServerLinesAreCapped(test *testing.T) {
	// Not parallel: this case swaps the package-level exec hook.
	restore := replicaRouterExec
	defer func() { replicaRouterExec = restore }()
	names := make([]string, replicaMaxServers+4)
	rows := make([]string, 0, len(names))
	for index := range names {
		names[index] = fmt.Sprintf("replica-%d", index)
		rows = append(rows, replicaStateRowText("be_http:test:route", fmt.Sprintf("pod:replica-%d:svc", index), "2", "0", "1", "3"))
	}
	replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
		return replicaStateDump(rows...), nil
	}
	client := fake.NewSimpleClientset(replicaTestSlice(), replicaTestPod(configManagerRouterPodName), replicaTestPod(replicaSensitive))
	emitted := false
	reportReplicaFailure(context.Background(), context.DeadlineExceeded, replicaRequestStats{names: names}, client, &rest.Config{}, "test", "service", replicaSensitive, func(output string) {
		emitted = true
		if !strings.Contains(output, "servers_capped=true") || !strings.Contains(output, fmt.Sprintf("servers=%d", len(names))) {
			test.Fatal("dropped server lines were not flagged")
		}
		if strings.Count(output, "haproxy_server ordinal=") != replicaMaxServers {
			test.Fatalf("emitted %d server lines, want %d", strings.Count(output, "haproxy_server ordinal="), replicaMaxServers)
		}
		if !strings.Contains(output, "incomplete=true") {
			test.Fatal("capped report not marked incomplete")
		}
	})
	if !emitted {
		test.Fatal("diagnostics were not emitted")
	}
}

func TestReplicaServerStateSkippedWithoutRuntimeAccess(test *testing.T) {
	// Not parallel: this case swaps the package-level exec hook.
	restore := replicaRouterExec
	defer func() { replicaRouterExec = restore }()
	replicaRouterExec = func(*rest.Config, kubernetes.Interface, string) (string, error) {
		panic("collection must not be attempted without runtime access")
	}
	client := fake.NewSimpleClientset(replicaTestSlice(), replicaTestPod(configManagerRouterPodName), replicaTestPod(replicaSensitive))
	reportReplicaFailure(context.Background(), errors.New(replicaSensitive), replicaRequestStats{names: replicaStateNames}, client, nil, "test", "service", replicaSensitive, func(output string) {
		if strings.Contains(output, "haproxy") {
			test.Fatal("state reported without runtime access")
		}
	})
}

func TestReplicaErrorNameOmitsZeroValueCategory(test *testing.T) {
	test.Parallel()
	if replicaErrorName(false, replicaOther) != "none" {
		test.Fatal("a collection that did not fail must not name a failure category")
	}
	if replicaErrorName(true, replicaForbidden) != "forbidden" {
		test.Fatal("a failed collection must name its category")
	}
}

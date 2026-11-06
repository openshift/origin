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
	reportReplicaFailure(context.Background(), nil, replicaRequestStats{}, client, "test", "service", replicaSensitive, func(string) { emitted = true })
	if emitted || len(client.Actions()) != 0 {
		test.Fatal("success triggered diagnostics")
	}
	stats := replicaRequestStats{iteration: 3, expected: 3, observed: 2, attempts: 4, successes: 2}
	stats.failures[replicaHTTP503] = 1
	stats.failures[replicaExec] = 1
	cleaned := false
	func() {
		defer func() { cleaned = true }()
		reportReplicaFailure(context.Background(), errors.New(replicaSensitive), stats, client, "test", "service", replicaSensitive, func(output string) {
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
	reportReplicaFailure(context.Background(), errors.New(replicaSensitive), replicaRequestStats{}, client, "test", "service", replicaSensitive, func(string) {})
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
	reportReplicaFailure(context.Background(), context.DeadlineExceeded, replicaRequestStats{}, client, "test", "service", replicaSensitive, func(output string) {
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
	reportReplicaFailure(ctx, context.DeadlineExceeded, replicaRequestStats{}, client, "test", "service", replicaSensitive, func(output string) {
		if !strings.Contains(output, "incomplete=true") || !strings.Contains(output, "lookup_failed=true error=timeout") {
			test.Fatal("deadline not represented")
		}
	})
	if time.Since(start) > time.Second {
		test.Fatal("collection exceeded shared caller deadline")
	}
}

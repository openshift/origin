package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
)

// dcmDiagnosticTestPod supplies synthetic lifecycle failures and sensitive fields for isolation tests.
func dcmDiagnosticTestPod(name, controllerName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "openshift-ingress", Labels: map[string]string{dcmRouterControllerLabel: controllerName}},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init-router"}, {Name: "haproxy"}},
			Containers:     []corev1.Container{{Name: "router", Env: []corev1.EnvVar{{Name: "PASSWORD", Value: "test-sensitive-config"}}}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "init-router", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing", Message: "SYNTHETIC_STATUS_MESSAGE"}}},
				{Name: "haproxy", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 17, Signal: 9, Reason: "SYNTHETIC_TERMINATION_REASON", Message: "SYNTHETIC_TERMINATION_MESSAGE"}}},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "router", RestartCount: 2, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			},
		},
	}
}

// TestDCMFailureDiagnosticsBeforeCleanup protects collection ordering and unmodified deletion behavior.
func TestDCMFailureDiagnosticsBeforeCleanup(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		failed         bool
		controllerName string
		listError      bool
		logError       bool
		deleteError    bool
		wantLogs       int
	}{
		{name: "failure before execPod assignment", failed: true, controllerName: "test-controller", wantLogs: 6},
		{name: "list failure still deletes controller", failed: true, controllerName: "test-controller", listError: true},
		{name: "container errors still delete controller", failed: true, controllerName: "test-controller", logError: true, wantLogs: 6},
		{name: "cleanup errors remain failures", failed: true, controllerName: "test-controller", deleteError: true, wantLogs: 6},
		{name: "successful spec only deletes controller", controllerName: "test-controller"},
		{name: "controller creation failed", failed: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var mutex sync.Mutex
			var events []string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				mutex.Lock()
				defer mutex.Unlock()
				writer.Header().Set("Content-Type", "application/json")
				if request.Method == http.MethodDelete {
					events = append(events, "delete")
					var options metav1.DeleteOptions
					if err := json.NewDecoder(request.Body).Decode(&options); err != nil {
						t.Errorf("decode deletion options: %v", err)
					}
					if request.URL.Path != "/apis/operator.openshift.io/v1/namespaces/openshift-ingress-operator/ingresscontrollers/"+testCase.controllerName || options.GracePeriodSeconds == nil || *options.GracePeriodSeconds != 1 {
						t.Error("changed controller cleanup target or grace period")
					}
					if testCase.deleteError {
						http.Error(writer, "test delete failure", http.StatusInternalServerError)
						return
					}
					fmt.Fprint(writer, `{}`)
					return
				}
				if request.URL.Path == "/api/v1/namespaces/openshift-ingress/pods" {
					events = append(events, "list")
					if request.URL.Query().Get("labelSelector") != dcmRouterControllerLabel+"=test-controller" || request.URL.Query().Get("limit") != "3" {
						t.Errorf("unexpected selection query: %s", request.URL.RawQuery)
					}
					if testCase.listError {
						http.Error(writer, "test-sensitive-error", http.StatusInternalServerError)
						return
					}
					pods := corev1.PodList{Items: []corev1.Pod{
						dcmDiagnosticTestPod("unrelated", "other-controller"),
						dcmDiagnosticTestPod("router-old", "test-controller"),
						dcmDiagnosticTestPod("router-new", "test-controller"),
					}}
					pods.Items[1].DeletionTimestamp = &metav1.Time{Time: time.Now()}
					if err := json.NewEncoder(writer).Encode(pods); err != nil {
						t.Errorf("encode pods: %v", err)
					}
					return
				}
				if strings.Contains(request.URL.Path, "unrelated") {
					t.Error("requested unrelated pod logs")
				}
				events = append(events, "log")
				if request.URL.Query().Get("tailLines") != "100" || request.URL.Query().Get("limitBytes") != "16384" {
					t.Errorf("unbounded log query: %s", request.URL.RawQuery)
				}
				if testCase.logError {
					http.Error(writer, "test-sensitive-error", http.StatusInternalServerError)
					return
				}
				fmt.Fprint(writer, "test router startup log")
			}))
			defer server.Close()
			kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			operatorClient, err := operatorclient.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			err = cleanupDCMIngressController(context.Background(), kubeClient, operatorClient, types.NamespacedName{Namespace: "openshift-ingress-operator", Name: testCase.controllerName}, testCase.failed)
			if (err != nil) != testCase.deleteError {
				t.Fatalf("unexpected cleanup result: %v", err)
			}
			mutex.Lock()
			defer mutex.Unlock()
			var expected []string
			if testCase.controllerName != "" {
				if testCase.failed {
					expected = append(expected, "list")
				}
				for logIndex := 0; logIndex < testCase.wantLogs; logIndex++ {
					expected = append(expected, "log")
				}
				expected = append(expected, "delete")
			}
			if !reflect.DeepEqual(events, expected) {
				t.Fatalf("diagnostics must precede deletion: got %v, want %v", events, expected)
			}
		})
	}
}

// TestDCMRouterDiagnosticsBoundsAndRedaction checks useful allowlisted evidence and bounded isolation.
func TestDCMRouterDiagnosticsBoundsAndRedaction(t *testing.T) {
	var mutex sync.Mutex
	var containers []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/v1/namespaces/openshift-ingress/pods" {
			pods := corev1.PodList{}
			for podIndex := 0; podIndex < dcmDiagnosticMaxPods+2; podIndex++ {
				pod := dcmDiagnosticTestPod(fmt.Sprintf("synthetic.example.invalid-%d", podIndex), "test-controller")
				pod.Spec.NodeName = "SYNTHETIC_HOSTNAME"
				if podIndex == 1 {
					pod.Status.Phase = "SYNTHETIC_PHASE"
				}
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "extra"}, corev1.Container{Name: "excluded"})
				pods.Items = append(pods.Items, pod)
			}
			if err := json.NewEncoder(writer).Encode(pods); err != nil {
				t.Errorf("encode pods: %v", err)
			}
			return
		}
		containers = append(containers, request.URL.Query().Get("container"))
		fmt.Fprint(writer, "password=SYNTHETIC_PASSWORD api_key=SYNTHETIC_API_KEY session=SYNTHETIC_SESSION user=synthetic@example.invalid\n"+strings.Repeat("unknown log line\n", dcmDiagnosticLogBytes))
	}))
	defer server.Close()
	kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	output := collectDCMRouterDiagnostics(context.Background(), kubeClient, "test-controller")
	mutex.Lock()
	defer mutex.Unlock()
	if len(containers) != dcmDiagnosticMaxPods*dcmDiagnosticMaxContainers {
		t.Fatalf("unexpected number of log requests: %v", containers)
	}
	if !reflect.DeepEqual(containers[:4], []string{"init-router", "haproxy", "router", "extra"}) {
		t.Fatalf("init and regular containers not covered: %v", containers)
	}
	for _, excluded := range []string{"SYNTHETIC", "synthetic@example.invalid", "synthetic.example.invalid", "test-sensitive-config", "test-controller", "unknown log line", "extra", "excluded"} {
		if strings.Contains(output, excluded) {
			t.Errorf("diagnostics contain excluded content %q", excluded)
		}
	}
	for _, expected := range []string{"Pod 1 phase=Pending", "Pod 2 phase=Other", "init=true", "waiting:PodInitializing", "terminated:exit=17,signal=9", "restarts=2 state=waiting:CrashLoopBackOff", "Log sample bytes=16384", "clipped=true (body suppressed)"} {
		if !strings.Contains(output, expected) {
			t.Errorf("missing useful allowlisted evidence %q", expected)
		}
	}
	if len(output) > 4096 {
		t.Fatalf("diagnostic output exceeded size bound: %d", len(output))
	}
}

// TestDCMRouterLogMetadata proves that no body text survives unknown, partial or failed reads.
func TestDCMRouterLogMetadata(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		input      string
		want       string
		status     int
		incomplete bool
		wantError  bool
	}{
		{name: "empty body", want: "Log sample bytes=0 completeLines=0 clipped=false (body suppressed)"},
		{name: "unrecognized messages", input: "password=SYNTHETIC_PASSWORD\napi_key=SYNTHETIC_API_KEY\n", want: "Log sample bytes=54 completeLines=2 clipped=false (body suppressed)"},
		{name: "unterminated line", input: "SYNTHETIC_SESSION", want: "Log sample bytes=17 completeLines=0 clipped=false (body suppressed)"},
		{name: "over byte bound", input: strings.Repeat("S", dcmDiagnosticLogBytes*2), want: "Log sample bytes=16384 completeLines=0 clipped=true (body suppressed)"},
		{name: "at byte bound", input: strings.Repeat("S", dcmDiagnosticLogBytes), want: "Log sample bytes=16384 completeLines=0 clipped=true (body suppressed)"},
		{name: "API error body", input: "SYNTHETIC_ERROR_BODY", status: http.StatusBadRequest, wantError: true},
		{name: "incomplete response", input: "SYNTHETIC_PARTIAL_BODY", incomplete: true, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if testCase.incomplete {
					writer.Header().Set("Content-Length", "1024")
				}
				if testCase.status != 0 {
					writer.WriteHeader(testCase.status)
				}
				fmt.Fprint(writer, testCase.input)
			}))
			defer server.Close()
			kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			got, err := readDCMRouterLog(context.Background(), kubeClient, "router", "haproxy")
			if (err != nil) != testCase.wantError || got != testCase.want {
				t.Fatalf("unexpected log metadata: %q, errorPresent=%t", got, err != nil)
			}
		})
	}
}

// TestDCMContainerState rejects unknown reasons and messages while retaining lifecycle facts.
func TestDCMContainerState(t *testing.T) {
	for _, testCase := range []struct {
		state corev1.ContainerState
		want  string
	}{
		{want: "unobserved"},
		{state: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, want: "running"},
		{state: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "password=SYNTHETIC_PASSWORD", Message: "SYNTHETIC_MESSAGE"}}, want: "waiting:Other"},
		{state: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 23, Signal: 9, Message: "SYNTHETIC_MESSAGE", Reason: "SYNTHETIC_REASON"}}, want: "terminated:exit=23,signal=9"},
	} {
		t.Run(testCase.want, func(t *testing.T) {
			if got := dcmContainerState(testCase.state); got != testCase.want {
				t.Fatalf("unexpected state classification: %q", got)
			}
		})
	}
}

// TestDCMDiagnosticErrorsSuppressDetails checks the final output boundary, not just the reader.
func TestDCMDiagnosticErrorsSuppressDetails(t *testing.T) {
	for _, failList := range []bool{true, false} {
		t.Run(fmt.Sprintf("listFailure=%t", failList), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if !failList && request.URL.Path == "/api/v1/namespaces/openshift-ingress/pods" {
					writer.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(writer).Encode(corev1.PodList{Items: []corev1.Pod{dcmDiagnosticTestPod("router", "test-controller")}}); err != nil {
						t.Errorf("encode pods: %v", err)
					}
					return
				}
				http.Error(writer, "password=SYNTHETIC_PASSWORD host=synthetic.example.invalid", http.StatusBadRequest)
			}))
			defer server.Close()
			kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			output := collectDCMRouterDiagnostics(context.Background(), kubeClient, "test-controller")
			if !strings.Contains(output, "Unable to") || strings.Contains(output, "SYNTHETIC") || strings.Contains(output, "synthetic.example.invalid") {
				t.Fatalf("unexpected error classification output: %q", output)
			}
		})
	}
}

// TestDCMRouterDiagnosticsContextDeadline protects both API and streaming-body deadlines.
func TestDCMRouterDiagnosticsContextDeadline(t *testing.T) {
	for _, blockedOperation := range []string{"list", "log body"} {
		t.Run(blockedOperation, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if blockedOperation == "log body" && request.URL.Path == "/api/v1/namespaces/openshift-ingress/pods" {
					writer.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(writer).Encode(corev1.PodList{Items: []corev1.Pod{dcmDiagnosticTestPod("router", "test-controller")}}); err != nil {
						t.Errorf("encode pods: %v", err)
					}
					return
				}
				if blockedOperation == "log body" {
					writer.WriteHeader(http.StatusOK)
					writer.(http.Flusher).Flush()
				}
				<-request.Context().Done()
			}))
			defer server.Close()
			kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			started := time.Now()
			output := collectDCMRouterDiagnostics(ctx, kubeClient, "test-controller")
			if time.Since(started) > time.Second || !strings.Contains(output, "Unable to") {
				t.Fatalf("%s did not respect the shared deadline: elapsed=%s output=%s", blockedOperation, time.Since(started), output)
			}
		})
	}
}

// TestDCMDiagnosticsDeadlineStillAllowsCleanup ensures collection timeouts cannot skip deletion.
func TestDCMDiagnosticsDeadlineStillAllowsCleanup(t *testing.T) {
	var mutex sync.Mutex
	var requests int
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		if request.Method == http.MethodDelete {
			deleted = true
			mutex.Unlock()
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprint(writer, `{}`)
			return
		}
		requests++
		mutex.Unlock()
		<-request.Context().Done()
	}))
	defer server.Close()
	kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operatorClient, err := operatorclient.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = cleanupDCMIngressController(context.Background(), kubeClient, operatorClient, types.NamespacedName{Namespace: "openshift-ingress-operator", Name: "test-controller"}, true)
	mutex.Lock()
	defer mutex.Unlock()
	if err != nil || !deleted || time.Since(started) > dcmDiagnosticRequestTimeout+2*time.Second {
		t.Fatalf("diagnostics prevented timely cleanup: err=%v deleted=%t elapsed=%s", err, deleted, time.Since(started))
	}
	if requests != 1 {
		t.Fatalf("unexpected requests after deadline: %d", requests)
	}
}

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

func dcmDiagnosticTestPod(name, controllerName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "openshift-ingress", Labels: map[string]string{dcmRouterControllerLabel: controllerName}},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init-router"}, {Name: "haproxy"}},
			Containers:     []corev1.Container{{Name: "router", Env: []corev1.EnvVar{{Name: "PASSWORD", Value: "test-sensitive-config"}}}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}
}

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
				pod := dcmDiagnosticTestPod(fmt.Sprintf("router-%d", podIndex), "test-controller")
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "extra"}, corev1.Container{Name: "excluded"})
				pods.Items = append(pods.Items, pod)
			}
			if err := json.NewEncoder(writer).Encode(pods); err != nil {
				t.Errorf("encode pods: %v", err)
			}
			return
		}
		containers = append(containers, request.URL.Query().Get("container"))
		fmt.Fprint(writer, "Authorization: Bearer test-bearer-value\n-----BEGIN PRIVATE KEY-----\nTEST_KEY_MATERIAL\n-----END PRIVATE KEY-----\n"+strings.Repeat("log line\n", dcmDiagnosticLogBytes))
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
	for _, excluded := range []string{"test-bearer-value", "TEST_KEY_MATERIAL", "test-sensitive-config", "router-3", "excluded"} {
		if strings.Contains(output, excluded) {
			t.Errorf("diagnostics contain excluded content %q", excluded)
		}
	}
	if !strings.Contains(output, "<redacted>") || !strings.Contains(output, "<redacted PEM>") {
		t.Error("expected redaction markers")
	}
	if len(output) > dcmDiagnosticMaxPods*dcmDiagnosticMaxContainers*(dcmDiagnosticLogBytes+256) {
		t.Fatalf("diagnostic output exceeded size bound: %d", len(output))
	}
}

func TestDCMRouterLogRedactsTruncatedPEM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, "startup\n-----BEGIN RSA PRIVATE KEY-----\n"+strings.Repeat("TEST_KEY_MATERIAL", dcmDiagnosticLogBytes))
	}))
	defer server.Close()
	kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	output, err := readDCMRouterLog(context.Background(), kubeClient, "router", "haproxy")
	if err != nil || output != "startup\n<redacted PEM>" {
		t.Fatalf("truncated PEM was not suppressed: error=%v bytes=%d", err, len(output))
	}
}

func TestDCMRouterLogRedaction(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "ordinary router log", input: "router startup\n", want: "router startup\n"},
		{name: "bearer authorization", input: "Authorization: Bearer synthetic-token", want: "Authorization: Bearer <redacted>"},
		{name: "bearer config", input: `BearerToken:"synthetic-token"`, want: `BearerToken:"<redacted>"`},
		{name: "PEM tail without header", input: "SYNTHETIC_KEY_FRAGMENT\n-----END EC PRIVATE KEY-----\nrouter startup", want: "<redacted PEM>\nrouter startup"},
		{name: "PEM body without delimiters", input: "router startup\n" + strings.Repeat("QUJD", 16) + "\nrouter ready", want: "router startup\n<redacted key data>\nrouter ready"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := redactDCMRouterLog(testCase.input); got != testCase.want {
				t.Fatalf("unexpected sanitized output: %q", got)
			}
		})
	}
}

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

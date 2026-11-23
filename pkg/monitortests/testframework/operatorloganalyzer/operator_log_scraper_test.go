package operatorloganalyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
)

func TestScanAllOperatorPodsReturnsContextCancellationAfterTransientListFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		cancel() // stop the retry loop after the first failed request
		return true, nil, apierrors.NewServiceUnavailable("apiserver is restarting")
	})

	err := scanAllOperatorPods(ctx, client, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("scanAllOperatorPods() error = %v, want context cancellation", err)
	}
}

func TestScanAllOperatorPodsRetriesTransientListErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	attempts := 0
	client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts < 3 {
			return true, nil, apierrors.NewServiceUnavailable("apiserver is restarting")
		}
		return false, nil, nil
	})

	if err := scanAllOperatorPods(context.Background(), client, false); err != nil {
		t.Fatalf("scanAllOperatorPods() returned unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("scanAllOperatorPods() made %d list attempts, want 3", attempts)
	}
}

func TestCollectDataDoesNotExposeAPIServerURL(t *testing.T) {
	const internalAPIHost = "api-int.mycluster.example.com"

	tests := []struct {
		name            string
		reducedTopology bool
		wantFlake       bool
	}{
		{name: "reduced topology returns a sanitized flake", reducedTopology: true, wantFlake: true},
		{name: "highly available topology returns a sanitized hard error", reducedTopology: false, wantFlake: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewServiceUnavailable("request to https://" + internalAPIHost + ":6443 failed")
			})

			analyzer := &operatorLogAnalyzer{
				kubeClient:      client,
				reducedTopology: tt.reducedTopology,
			}
			_, _, err := analyzer.CollectData(context.Background(), "", time.Time{}, time.Time{})
			if err == nil {
				t.Fatal("CollectData() succeeded, want an error")
			}

			var flakeErr *monitortestframework.FlakeError
			if got := errors.As(err, &flakeErr); got != tt.wantFlake {
				t.Errorf("CollectData() flake classification = %v, want %v", got, tt.wantFlake)
			}
			if strings.Contains(err.Error(), internalAPIHost) {
				t.Errorf("CollectData() exposed the apiserver address: %q", err)
			}
		})
	}
}

// Both scraper passes list the operator pods and then look each pod up again
// before reading its log. A pod already gone at its own lookup has no log left
// to read, so it is skipped on every topology. Every other failure keeps its
// topology classification: a flake on a reduced topology when transient, and a
// failure otherwise. Each case runs StartCollection and then CollectData.
func TestCollectDataClassifiesPodNotFoundByTopology(t *testing.T) {
	const survivor, startVictim, finalVictim = "survivor-operator-aaaaa", "start-victim-operator-bbbbb", "final-victim-operator-ccccc"
	victims := []string{startVictim, finalVictim}
	notFound := func(resource schema.GroupResource, name string) apiResponse {
		return apiResponse{status: apierrors.NewNotFound(resource, name)}
	}

	// Every pod has two containers, and ReadLog looks the pod up again for each.
	read := func(pod string) []string {
		return []string{podPath(pod), podPath(pod) + "/log", podPath(pod), podPath(pod) + "/log"}
	}
	lookupOnly := func(pod string) []string { return []string{podPath(pod), podPath(pod)} }
	// startVictim is deleted after the StartCollection list and finalVictim after
	// the CollectData list; neither has its log requested once it is gone.
	scrape := func(survivorRequests []string) []string {
		return slices.Concat(
			[]string{infrastructurePath, "/api/v1/pods"}, survivorRequests, lookupOnly(startVictim), read(finalVictim),
			[]string{"/api/v1/pods"}, survivorRequests, lookupOnly(finalVictim),
		)
	}

	tests := []struct {
		name         string
		api          operatorAPI
		reduced, ha  outcome
		wantSkipped  []string
		wantRequests []string
	}{
		{
			name:         "pods deleted between listing and lookup are skipped",
			reduced:      pass,
			ha:           pass,
			wantSkipped:  victims,
			wantRequests: scrape(read(survivor)),
		},
		{
			name:         "NotFound from the log of a pod that was found",
			api:          operatorAPI{logs: map[string]apiResponse{survivor: notFound(corev1.Resource("pods"), survivor)}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(read(survivor)),
		},
		{
			name:         "lookup NotFound naming another pod",
			api:          operatorAPI{lookups: map[string]apiResponse{survivor: notFound(corev1.Resource("pods"), "other-operator-zzzzz")}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name:         "lookup NotFound naming no pod",
			api:          operatorAPI{lookups: map[string]apiResponse{survivor: notFound(corev1.Resource("pods"), "")}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name: "lookup NotFound without details",
			api: operatorAPI{lookups: map[string]apiResponse{survivor: {status: &apierrors.StatusError{ErrStatus: metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound, Code: http.StatusNotFound,
			}}}}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name:         "lookup NotFound for another resource",
			api:          operatorAPI{lookups: map[string]apiResponse{survivor: notFound(corev1.Resource("configmaps"), survivor)}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name: "lookup NotFound for pods of another API group",
			api: operatorAPI{lookups: map[string]apiResponse{
				survivor: notFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "pods"}, survivor),
			}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name:         "lookup 404 that is not an API status",
			api:          operatorAPI{lookups: map[string]apiResponse{survivor: {code: http.StatusNotFound, body: "404 page not found"}}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name: "lookup forbidden",
			api: operatorAPI{lookups: map[string]apiResponse{
				survivor: {status: apierrors.NewForbidden(corev1.Resource("pods"), survivor, errors.New("synthetic denial"))},
			}},
			reduced:      fail,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name: "lookup timeout",
			api: operatorAPI{lookups: map[string]apiResponse{
				survivor: {status: apierrors.NewServerTimeout(corev1.Resource("pods"), "get", 0)},
			}},
			reduced:      flake,
			ha:           fail,
			wantSkipped:  victims,
			wantRequests: scrape(lookupOnly(survivor)),
		},
		{
			name:         "pod list forbidden",
			api:          operatorAPI{listErr: apierrors.NewForbidden(corev1.Resource("pods"), "", errors.New("synthetic denial"))},
			reduced:      fail,
			ha:           fail,
			wantRequests: []string{infrastructurePath, "/api/v1/pods", "/api/v1/pods"},
		},
	}

	for _, tt := range tests {
		for _, topology := range []configv1.TopologyMode{configv1.SingleReplicaTopologyMode, configv1.HighlyAvailableTopologyMode} {
			want := tt.ha
			if topology == configv1.SingleReplicaTopologyMode {
				want = tt.reduced
			}
			t.Run(fmt.Sprintf("%s/%s", tt.name, topology), func(t *testing.T) {
				logs := captureLogs(t)
				api := tt.api
				api.topology = topology
				api.pods = []string{survivor, startVictim, finalVictim}
				api.deleteAfterList = victims
				server := api.serve(t)

				analyzer := &operatorLogAnalyzer{}
				recorder := monitor.NewRecorder()
				startErr := analyzer.StartCollection(context.Background(), server.config, recorder)
				if got := outcomeOf(startErr); got != want {
					t.Errorf("StartCollection() = %s (%v), want %s", got, startErr, want)
				}
				collected, _, collectErr := analyzer.CollectData(context.Background(), "", time.Time{}, time.Time{})
				if got := outcomeOf(collectErr); got != want {
					t.Errorf("CollectData() = %s (%v), want %s", got, collectErr, want)
				}

				for _, pod := range api.pods {
					if got, wantSkip := logs.skipped(pod), slices.Contains(tt.wantSkipped, pod); got != wantSkip {
						t.Errorf("logged skipping %s = %v, want %v", pod, got, wantSkip)
					}
				}
				if got := server.requestLog(); !slices.Equal(got, tt.wantRequests) {
					t.Errorf("API requests = %v, want %v", got, tt.wantRequests)
				}
				// A passing scrape must have fed the remaining pod's log to the handlers.
				if want == pass {
					if !hasLeaseFrom(recorder.Intervals(time.Time{}, time.Time{}), survivor) {
						t.Errorf("StartCollection() did not record the lease line logged by %s", survivor)
					}
					if !hasLeaseFrom(collected, survivor) {
						t.Errorf("CollectData() did not return the lease line logged by %s", survivor)
					}
				}
			})
		}
	}
}

func TestIsTransientScrapeErrorRecognizesReducedTopologyRecoveryErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "kubelet proxy authorization while a node is restarting",
			err:  errors.New("Internal error occurred: Authorization error (user=system:kube-apiserver, verb=get, resource=nodes, subresource=proxy)"),
			want: true,
		},
		{
			name: "API storage is reinitializing",
			err:  errors.New("Internal error occurred: storage is (re)initializing"),
			want: true,
		},
		{
			name: "etcd request timed out",
			err:  apierrors.NewInternalError(errors.New("etcdserver: request timed out")),
			want: true,
		},
		{
			name: "unrelated internal error remains strict",
			err:  apierrors.NewInternalError(errors.New("unexpected admission failure")),
			want: false,
		},
		{
			name: "unrelated authorization error remains strict",
			err:  errors.New("Authorization error (user=system:anonymous, verb=get, resource=secrets)"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientScrapeError(tt.err); got != tt.want {
				t.Errorf("isTransientScrapeError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransientScrapeErrorRejectsMixedJoinedErrors(t *testing.T) {
	err := errors.Join(
		apierrors.NewServiceUnavailable("apiserver is restarting"),
		errors.New("permanent log read failure"),
	)

	if isTransientScrapeError(err) {
		t.Fatal("isTransientScrapeError() classified a mixed joined error as transient")
	}
}

func TestIsTransientScrapeErrorMatchesSpecificContainerLogErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "terminated kubelet container",
			err:  errors.New(`container "operator" in pod "test-operator-abcde" is terminated`),
			want: true,
		},
		{
			name: "container unavailable from kubelet",
			err:  errors.New(`container "operator" in pod "test-operator-abcde" is not available`),
			want: true,
		},
		{
			name: "container being created",
			err:  errors.New(`container "operator" in pod "test-operator-abcde" is waiting to start: ContainerCreating`),
			want: true,
		},
		{
			name: "no container logs yet",
			err:  errors.New(`container "operator" in pod "test-operator-abcde" is waiting to start - no logs yet`),
			want: true,
		},
		{
			name: "container missing from kubelet",
			err:  errors.New(`container not found ("operator")`),
			want: true,
		},
		{
			name: "crash loop remains strict",
			err:  errors.New(`container "operator" in pod "test-operator-abcde" is waiting to start: CrashLoopBackOff`),
			want: false,
		},
		{
			name: "image pull failure remains strict",
			err:  errors.New(`container "operator" in pod "test-operator-abcde" is waiting to start: image can't be pulled`),
			want: false,
		},
		{
			name: "unrelated unavailable service remains strict",
			err:  errors.New("operator database is not available"),
			want: false,
		},
		{
			name: "unrelated container-not-found code remains strict",
			err:  errors.New("cloud storage returned ContainerNotFound"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientScrapeError(tt.err); got != tt.want {
				t.Errorf("isTransientScrapeError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizedScrapeErrorPreservesContextErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "cancellation", err: context.Canceled},
		{name: "deadline exceeded", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizedScrapeError(tt.err); !errors.Is(got, tt.err) {
				t.Errorf("sanitizedScrapeError() error = %v, want errors.Is(_, %v)", got, tt.err)
			}
		})
	}
}

const (
	testOperatorNamespace = "openshift-test-operator"
	testLeaseLine         = "successfully acquired lease openshift-test-operator/test-lock"
	infrastructurePath    = "/apis/config.openshift.io/v1/infrastructures/cluster"
)

// operatorAPI configures a minimal apiserver for the scraper. Each pod has two
// containers whose logs are testLeaseLine unless logs overrides them.
type operatorAPI struct {
	topology configv1.TopologyMode
	pods     []string
	// deleteAfterList[i] is deleted right after the (i+1)th pod list is served,
	// so the scraper lists it and then finds it gone at its own lookup.
	deleteAfterList []string
	listErr         *apierrors.StatusError
	lookups         map[string]apiResponse
	logs            map[string]apiResponse
}

// apiResponse is an API Status for status, or otherwise a plain-text body such
// as a proxy or other non-apiserver handler would return.
type apiResponse struct {
	status *apierrors.StatusError
	code   int
	body   string
}

type operatorAPIServer struct {
	operatorAPI
	config *rest.Config

	mu       sync.Mutex
	lists    int
	deleted  map[string]bool
	requests []string
}

func (a operatorAPI) serve(t *testing.T) *operatorAPIServer {
	s := &operatorAPIServer{operatorAPI: a, deleted: map[string]bool{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handle(t, w, r)
	}))
	t.Cleanup(server.Close)
	// Disable client-side throttling so request-heavy cases do not wait on it.
	s.config = &rest.Config{Host: server.URL, QPS: -1}
	return s
}

func (s *operatorAPIServer) handle(t *testing.T, w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.URL.Path)

	podPrefix := podPath("")
	name, isLog := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, podPrefix), "/log")
	switch {
	case r.URL.Path == infrastructurePath:
		writeJSON(t, w, http.StatusOK, &configv1.Infrastructure{
			TypeMeta:   metav1.TypeMeta{Kind: "Infrastructure", APIVersion: "config.openshift.io/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Status:     configv1.InfrastructureStatus{ControlPlaneTopology: s.topology},
		})
	case r.URL.Path == "/api/v1/pods":
		if s.listErr != nil {
			apiResponse{status: s.listErr}.write(t, w)
			return
		}
		list := &corev1.PodList{TypeMeta: metav1.TypeMeta{Kind: "PodList", APIVersion: "v1"}}
		for _, pod := range s.pods {
			if !s.deleted[pod] {
				list.Items = append(list.Items, *testOperatorPod(pod))
			}
		}
		writeJSON(t, w, http.StatusOK, list)
		if s.lists < len(s.deleteAfterList) {
			s.deleted[s.deleteAfterList[s.lists]] = true
		}
		s.lists++
	case !strings.HasPrefix(r.URL.Path, podPrefix) || !slices.Contains(s.pods, name):
		t.Errorf("unexpected API request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	case s.deleted[name]:
		if isLog {
			t.Errorf("requested the log of %s after its lookup found it deleted", name)
		}
		apiResponse{status: apierrors.NewNotFound(corev1.Resource("pods"), name)}.write(t, w)
	case isLog:
		if resp, ok := s.logs[name]; ok {
			resp.write(t, w)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if _, err := fmt.Fprintf(w, "2026-01-01T00:00:00Z %s\n", testLeaseLine); err != nil {
			t.Errorf("writing log: %v", err)
		}
	default:
		if resp, ok := s.lookups[name]; ok {
			resp.write(t, w)
			return
		}
		writeJSON(t, w, http.StatusOK, testOperatorPod(name))
	}
}

func (s *operatorAPIServer) requestLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func (r apiResponse) write(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	if r.status != nil {
		status := r.status.ErrStatus
		status.TypeMeta = metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}
		writeJSON(t, w, int(status.Code), &status)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(r.code)
	if _, err := io.WriteString(w, r.body); err != nil {
		t.Errorf("writing response: %v", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, code int, obj any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		t.Errorf("writing response: %v", err)
	}
}

func podPath(name string) string {
	return "/api/v1/namespaces/" + testOperatorNamespace + "/pods/" + name
}

func testOperatorPod(name string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Namespace: testOperatorNamespace, Name: name},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "operator"}, {Name: "sidecar"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func hasLeaseFrom(intervals monitorapi.Intervals, pod string) bool {
	for _, interval := range intervals {
		if interval.Message.Reason == monitorapi.LeaseAcquired &&
			interval.Message.HumanMessage == testLeaseLine &&
			interval.Locator.Keys[monitorapi.LocatorPodKey] == pod {
			return true
		}
	}
	return false
}

type outcome string

const (
	pass  outcome = "pass"
	flake outcome = "flake"
	fail  outcome = "fail"
)

func outcomeOf(err error) outcome {
	var flakeErr *monitortestframework.FlakeError
	switch {
	case err == nil:
		return pass
	case errors.As(err, &flakeErr):
		return flake
	default:
		return fail
	}
}

// logCapture records the scraper's logrus messages for the duration of a test.
type logCapture struct {
	mu       sync.Mutex
	messages []string
}

func captureLogs(t *testing.T) *logCapture {
	c := &logCapture{}
	logger := logrus.StandardLogger()
	original := logger.ReplaceHooks(logrus.LevelHooks{})
	logger.AddHook(c)
	t.Cleanup(func() { logger.ReplaceHooks(original) })
	return c
}

func (c *logCapture) Levels() []logrus.Level { return logrus.AllLevels }

func (c *logCapture) Fire(entry *logrus.Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, entry.Message)
	return nil
}

func (c *logCapture) skipped(pod string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	want := fmt.Sprintf("skipping pods/%s -n %s", pod, testOperatorNamespace)
	return slices.ContainsFunc(c.messages, func(message string) bool { return strings.Contains(message, want) })
}

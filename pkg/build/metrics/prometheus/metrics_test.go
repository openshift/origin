package prometheus

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	internalversion "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type fakeLister []*buildapi.Build

func (f fakeLister) List(labels.Selector) ([]*buildapi.Build, error) {
	return f, nil
}
func (fakeLister) Builds(string) internalversion.BuildNamespaceLister {
	return nil
}

type fakeResponseWriter struct {
	bytes.Buffer
	statusCode int
	header     http.Header
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.header
}

func (f *fakeResponseWriter) WriteHeader(statusCode int) {
	f.statusCode = statusCode
}

func TestMetrics(t *testing.T) {
	// went per line vs. a block of expected test in case assumptions on ordering are subverted, as well as defer on
	// getting newlines right
	expectedResponse := []string{
		"# HELP openshift_build_total Counts builds by phase, reason, and strategy",
		"# TYPE openshift_build_total gauge",
		"openshift_build_total{phase=\"Cancelled\",reason=\"\",strategy=\"\"} 1",
		"openshift_build_total{phase=\"Complete\",reason=\"\",strategy=\"\"} 1",
		"openshift_build_total{phase=\"Error\",reason=\"\",strategy=\"\"} 1",
		"openshift_build_total{phase=\"Failed\",reason=\"ExceededRetryTimeout\",strategy=\"\"} 1",
		"openshift_build_total{phase=\"New\",reason=\"InvalidOutputReference\",strategy=\"\"} 1",
		"# HELP openshift_build_active_time_seconds Shows the last transition time in unix epoch for running builds by namespace, name, phase, reason, and strategy",
		"# TYPE openshift_build_active_time_seconds gauge",
		"openshift_build_active_time_seconds{name=\"testname1\",namespace=\"testnamespace\",phase=\"New\",reason=\"\",strategy=\"\"} 123",
		"openshift_build_active_time_seconds{name=\"testname2\",namespace=\"testnamespace\",phase=\"Pending\",reason=\"\",strategy=\"\"} 123",
		"openshift_build_active_time_seconds{name=\"testname3\",namespace=\"testnamespace\",phase=\"Running\",reason=\"\",strategy=\"\"} 123",
	}
	registry := prometheus.NewRegistry()

	buildLister := &fakeLister{
		{
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseComplete,
			},
		},
		{
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseCancelled,
			},
		},
		{
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseError,
			},
		},
		{
			Status: buildapi.BuildStatus{
				Phase:  buildapi.BuildPhaseFailed,
				Reason: buildapi.StatusReasonExceededRetryTimeout,
			},
		},
		{
			Status: buildapi.BuildStatus{
				Phase:  buildapi.BuildPhaseNew,
				Reason: buildapi.StatusReasonInvalidOutputReference,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "testnamespace",
				Name:      "testname1",
				CreationTimestamp: metav1.Time{
					Time: time.Unix(123, 0),
				},
			},
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseNew,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "testnamespace",
				Name:      "testname2",
				CreationTimestamp: metav1.Time{
					Time: time.Unix(123, 0),
				},
			},
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhasePending,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "testnamespace",
				Name:      "testname3",
			},
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseRunning,
				StartTimestamp: &metav1.Time{
					Time: time.Unix(123, 0),
				},
			},
		},
	}

	bc := buildCollector{
		lister: buildLister,
	}

	registry.MustRegister(&bc)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})
	rw := &fakeResponseWriter{header: http.Header{}}
	h.ServeHTTP(rw, &http.Request{})

	respStr := rw.String()

	for _, s := range expectedResponse {
		if !strings.Contains(respStr, s) {
			t.Errorf("expected string %s did not appear in %s", s, respStr)
		}
	}
}

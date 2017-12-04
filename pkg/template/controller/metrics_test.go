package controller

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

type fakeLister []*templateapi.TemplateInstance

func (f fakeLister) List(labels.Selector) ([]*templateapi.TemplateInstance, error) {
	return f, nil
}
func (fakeLister) TemplateInstances(string) internalversion.TemplateInstanceNamespaceLister {
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
	expectedResponse := `# HELP openshift_template_instance_active_start_time_seconds Show the start time in unix epoch form of active TemplateInstance objects by namespace and name
# TYPE openshift_template_instance_active_start_time_seconds gauge
openshift_template_instance_active_start_time_seconds{name="testname",namespace="testnamespace"} 123
# HELP openshift_template_instance_status_condition_total Counts TemplateInstance objects by condition type and status
# TYPE openshift_template_instance_status_condition_total gauge
openshift_template_instance_status_condition_total{status="False",type="Ready"} 1
openshift_template_instance_status_condition_total{status="True",type="Ready"} 1
# HELP openshift_template_instance_total Counts TemplateInstance objects
# TYPE openshift_template_instance_total gauge
openshift_template_instance_total 2
`
	registry := prometheus.NewRegistry()

	c := &TemplateInstanceController{
		lister: &fakeLister{
			{
				Status: templateapi.TemplateInstanceStatus{
					Conditions: []templateapi.TemplateInstanceCondition{
						{
							Type:   templateapi.TemplateInstanceReady,
							Status: kapi.ConditionTrue,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testnamespace",
					Name:      "testname",
					CreationTimestamp: metav1.Time{
						Time: time.Unix(123, 0),
					},
				},
				Status: templateapi.TemplateInstanceStatus{
					Conditions: []templateapi.TemplateInstanceCondition{
						{
							Type:   templateapi.TemplateInstanceReady,
							Status: kapi.ConditionFalse,
						},
					},
				},
			},
		},
	}

	registry.MustRegister(c)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})
	rw := &fakeResponseWriter{header: http.Header{}}
	h.ServeHTTP(rw, &http.Request{})

	if rw.String() != expectedResponse {
		t.Error(rw.String())
	}
}

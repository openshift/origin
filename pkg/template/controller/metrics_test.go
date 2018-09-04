package controller

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	templateapi "github.com/openshift/api/template/v1"
	templateclient "github.com/openshift/client-go/template/clientset/versioned"
	"github.com/openshift/client-go/template/clientset/versioned/fake"
	templatelister "github.com/openshift/client-go/template/listers/template/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
)

type fakeLister struct {
	templateClient templateclient.Interface
}

func (f *fakeLister) List(labels.Selector) ([]*templateapi.TemplateInstance, error) {
	list, err := f.templateClient.Template().TemplateInstances("").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	templateInstances := make([]*templateapi.TemplateInstance, len(list.Items))
	for i := range list.Items {
		templateInstances[i] = &list.Items[i]
	}
	return templateInstances, err
}

func (f *fakeLister) Get(name string) (*templateapi.TemplateInstance, error) {
	return f.templateClient.Template().TemplateInstances("").Get(name, metav1.GetOptions{})
}

func (f *fakeLister) TemplateInstances(string) templatelister.TemplateInstanceNamespaceLister {
	return f
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
	expectedResponse := `# HELP openshift_template_instance_active_age_seconds Shows the instantaneous age distribution of active TemplateInstance objects
# TYPE openshift_template_instance_active_age_seconds histogram
openshift_template_instance_active_age_seconds_bucket{le="600"} 0
openshift_template_instance_active_age_seconds_bucket{le="1200"} 1
openshift_template_instance_active_age_seconds_bucket{le="1800"} 1
openshift_template_instance_active_age_seconds_bucket{le="2400"} 1
openshift_template_instance_active_age_seconds_bucket{le="3000"} 1
openshift_template_instance_active_age_seconds_bucket{le="3600"} 1
openshift_template_instance_active_age_seconds_bucket{le="4200"} 1
openshift_template_instance_active_age_seconds_bucket{le="+Inf"} 1
openshift_template_instance_active_age_seconds_sum 900
openshift_template_instance_active_age_seconds_count 1
# HELP openshift_template_instance_completed_total Counts completed TemplateInstance objects by condition
# TYPE openshift_template_instance_completed_total counter
openshift_template_instance_completed_total{condition="InstantiateFailure"} 2
openshift_template_instance_completed_total{condition="Ready"} 1
`

	clock := &fakeClock{now: time.Unix(0, 0)}

	registry := prometheus.NewRegistry()

	fakeTemplateClient := fake.NewSimpleClientset(
		// when sync is called on this TemplateInstance it should fail and
		// increment openshift_template_instance_completed_total
		// {condition="InstantiateFailure"}
		&templateapi.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "abouttofail",
			},
			Spec: templateapi.TemplateInstanceSpec{
				Template: templateapi.Template{
					Objects: []runtime.RawExtension{
						{Object: &corev1.ConfigMap{}},
					},
				},
			},
		},
		// when sync is called on this TemplateInstance it should timeout and
		// increment openshift_template_instance_completed_total
		// {condition="InstantiateFailure"}
		&templateapi.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "abouttotimeout",
			},
			Spec: templateapi.TemplateInstanceSpec{
				Template: templateapi.Template{
					Objects: []runtime.RawExtension{
						{Object: &corev1.ConfigMap{}},
					},
				},
				Requester: &templateapi.TemplateInstanceRequester{},
			},
			Status: templateapi.TemplateInstanceStatus{
				Objects: []templateapi.TemplateInstanceObject{
					{},
				},
			},
		},
		// when sync is called on this TemplateInstance it should succeed and
		// increment openshift_template_instance_completed_total
		// {condition="Ready"}
		&templateapi.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "abouttosucceed",
				CreationTimestamp: metav1.Time{
					Time: clock.now,
				},
			},
			Spec: templateapi.TemplateInstanceSpec{
				Template: templateapi.Template{
					Objects: []runtime.RawExtension{
						{Object: &corev1.ConfigMap{}},
					},
				},
				Requester: &templateapi.TemplateInstanceRequester{},
			},
			Status: templateapi.TemplateInstanceStatus{
				Objects: []templateapi.TemplateInstanceObject{
					{},
				},
			},
		},
		// this TemplateInstance is in-flight, not timed out.
		&templateapi.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					Time: clock.now.Add(-900 * time.Second),
				},
			},
			Status: templateapi.TemplateInstanceStatus{
				Conditions: []templateapi.TemplateInstanceCondition{
					{
						Type:   templateapi.TemplateInstanceReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
	)

	c := &TemplateInstanceController{
		lister:           &fakeLister{fakeTemplateClient},
		templateClient:   fakeTemplateClient.TemplateV1(),
		clock:            clock,
		readinessLimiter: &workqueue.BucketRateLimiter{},
	}

	registry.MustRegister(c)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})

	// We loop twice: we expect the metrics response to match after the first
	// set of sync calls, and not change after the second set.
	for i := 0; i < 2; i++ {
		for _, key := range []string{"/abouttofail", "/abouttotimeout", "/abouttosucceed"} {
			err := c.sync(key)
			if err != nil {
				t.Fatal(err)
			}
		}

		rw := &fakeResponseWriter{header: http.Header{}}
		h.ServeHTTP(rw, &http.Request{})

		if rw.String() != expectedResponse {
			t.Errorf("run %d: %s\n", i, rw.String())
		}
	}
}

package controller

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	restutil "github.com/openshift/origin/pkg/util/rest"
)

type roundtripper func(*http.Request) (*http.Response, error)

func (rt roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

// TestControllerCheckReadiness verifies the basic behaviour of
// TemplateInstanceController.checkReadiness(): that it can return ready, not
// ready and timed out correctly.
func TestControllerCheckReadiness(t *testing.T) {
	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				templateapi.WaitForReadyAnnotation: "true",
			},
		},
	}

	// fake generic API server, responds to any HTTP req with the above job
	// object
	fakerestconfig := &rest.Config{
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return roundtripper(func(req *http.Request) (*http.Response, error) {
				b, err := json.Marshal(job)
				if err != nil {
					panic(err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBuffer(b)),
				}, nil
			})
		},
	}

	// fakeclient, respond "allowed" to any subjectaccessreview
	fakeclientset := &fake.Clientset{}
	c := &TemplateInstanceController{
		restmapper: restutil.DefaultMultiRESTMapper(),
		kc:         fakeclientset,
		config:     fakerestconfig,
	}
	fakeclientset.AddReactor("create", "subjectaccessreviews", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorization.SubjectAccessReview{Status: authorization.SubjectAccessReviewStatus{Allowed: true}}, nil
	})

	now := time.Now()
	templateInstance := &templateapi.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{Time: now},
		},
		Spec: templateapi.TemplateInstanceSpec{
			Requester: &templateapi.TemplateInstanceRequester{},
		},
		Status: templateapi.TemplateInstanceStatus{
			Objects: []templateapi.TemplateInstanceObject{
				{
					Ref: kapi.ObjectReference{
						APIVersion: "batch/v1",
						Kind:       "Job",
						Namespace:  "namespace",
						Name:       "name",
					},
				},
			},
		},
	}

	// should report not ready yet
	ready, err := c.checkReadiness(templateInstance, now)
	if ready || err != nil {
		t.Error(ready, err)
	}

	// should report timed out
	ready, err = c.checkReadiness(templateInstance, now.Add(readinessTimeout+1))
	if ready || err == nil || err.Error() != "Timeout" {
		t.Error(ready, err)
	}

	// should report ready
	job.Status.CompletionTime = &metav1.Time{Time: now}
	ready, err = c.checkReadiness(templateInstance, now)
	if !ready || err != nil {
		t.Error(ready, err)
	}

	// should report failed
	job.Status.Failed = int32(1)
	ready, err = c.checkReadiness(templateInstance, now)
	if ready || err == nil || err.Error() != "Readiness failed on Job namespace/name" {
		t.Error(ready, err)
	}
}

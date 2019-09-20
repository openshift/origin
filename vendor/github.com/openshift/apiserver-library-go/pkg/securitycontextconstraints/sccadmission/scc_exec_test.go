package sccadmission

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	securityv1listers "github.com/openshift/client-go/security/listers/security/v1"
)

// scc exec is a pass through to *constraint, so we only need to test that
// it correctly limits its actions to certain conditions
func TestExecAdmit(t *testing.T) {
	goodPod := func() *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "default",
				Containers: []corev1.Container{
					{
						SecurityContext: &corev1.SecurityContext{},
					},
				},
			},
		}
	}

	testCases := map[string]struct {
		operation   admission.Operation
		resource    string
		subresource string

		pod                    *corev1.Pod
		shouldAdmit            bool
		shouldHaveClientAction bool
	}{
		"unchecked operation": {
			operation:              admission.Create,
			resource:               string(coreapi.ResourcePods),
			subresource:            "exec",
			pod:                    goodPod(),
			shouldAdmit:            true,
			shouldHaveClientAction: false,
		},
		"unchecked resource": {
			operation:              admission.Connect,
			resource:               string(coreapi.ResourceSecrets),
			subresource:            "exec",
			pod:                    goodPod(),
			shouldAdmit:            true,
			shouldHaveClientAction: false,
		},
		"unchecked subresource": {
			operation:              admission.Connect,
			resource:               string(coreapi.ResourcePods),
			subresource:            "not-exec",
			pod:                    goodPod(),
			shouldAdmit:            true,
			shouldHaveClientAction: false,
		},
		"attach check": {
			operation:              admission.Connect,
			resource:               string(coreapi.ResourcePods),
			subresource:            "attach",
			pod:                    goodPod(),
			shouldAdmit:            false,
			shouldHaveClientAction: true,
		},
		"exec check": {
			operation:              admission.Connect,
			resource:               string(coreapi.ResourcePods),
			subresource:            "exec",
			pod:                    goodPod(),
			shouldAdmit:            false,
			shouldHaveClientAction: true,
		},
	}

	for k, v := range testCases {
		tc := fake.NewSimpleClientset(v.pod)
		tc.PrependReactor("get", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, v.pod, nil
		})

		// create the admission plugin
		p := NewSCCExecRestrictions()
		indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		cache := securityv1listers.NewSecurityContextConstraintsLister(indexer)
		p.constraintAdmission.sccLister = cache
		p.SetExternalKubeClientSet(tc)

		attrs := admission.NewAttributesRecord(nil, nil, coreapi.Kind("Pod").WithVersion("version"), "namespace", "pod-name", coreapi.Resource(v.resource).WithVersion("version"), v.subresource, v.operation, nil, false, &user.DefaultInfo{})
		err := p.Validate(context.TODO(), attrs, nil)

		if v.shouldAdmit && err != nil {
			t.Errorf("%s: expected no errors but received %v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s: expected errors but received none", k)
		}

		for _, action := range tc.Actions() {
			t.Logf("%s: %#v", k, action)
		}
		if !v.shouldHaveClientAction && (len(tc.Actions()) > 0) {
			t.Errorf("%s: unexpected actions: %v", k, tc.Actions())
		}
		if v.shouldHaveClientAction && (len(tc.Actions()) == 0) {
			t.Errorf("%s: no actions found", k)
		}
	}
}

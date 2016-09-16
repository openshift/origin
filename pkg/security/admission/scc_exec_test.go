package admission

import (
	"testing"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	testingcore "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	oscache "github.com/openshift/origin/pkg/client/cache"
)

// scc exec is a pass through to *constraint, so we only need to test that
// it correctly limits its actions to certain conditions
func TestExecAdmit(t *testing.T) {
	goodPod := func() *kapi.Pod {
		return &kapi.Pod{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "default",
			},
			Spec: kapi.PodSpec{
				ServiceAccountName: "default",
				Containers: []kapi.Container{
					{
						SecurityContext: &kapi.SecurityContext{},
					},
				},
			},
		}
	}

	testCases := map[string]struct {
		operation   kadmission.Operation
		resource    string
		subresource string

		pod, oldPod            *kapi.Pod
		shouldAdmit            bool
		shouldHaveClientAction bool
	}{
		"unchecked operation": {
			operation:              kadmission.Create,
			resource:               string(kapi.ResourcePods),
			subresource:            "exec",
			pod:                    goodPod(),
			shouldAdmit:            true,
			shouldHaveClientAction: false,
		},
		"unchecked resource": {
			operation:              kadmission.Connect,
			resource:               string(kapi.ResourceSecrets),
			subresource:            "exec",
			pod:                    goodPod(),
			shouldAdmit:            true,
			shouldHaveClientAction: false,
		},
		"unchecked subresource": {
			operation:              kadmission.Connect,
			resource:               string(kapi.ResourcePods),
			subresource:            "not-exec",
			pod:                    goodPod(),
			shouldAdmit:            true,
			shouldHaveClientAction: false,
		},
		"attach check": {
			operation:              kadmission.Connect,
			resource:               string(kapi.ResourcePods),
			subresource:            "attach",
			pod:                    goodPod(),
			shouldAdmit:            false,
			shouldHaveClientAction: true,
		},
		"exec check": {
			operation:              kadmission.Connect,
			resource:               string(kapi.ResourcePods),
			subresource:            "exec",
			pod:                    goodPod(),
			shouldAdmit:            false,
			shouldHaveClientAction: true,
		},
	}

	for k, v := range testCases {
		tc := clientsetfake.NewSimpleClientset(v.pod)
		tc.PrependReactor("get", "pods", func(action testingcore.Action) (handled bool, ret runtime.Object, err error) {
			return true, v.pod, nil
		})
		tc.AddWatchReactor("*", testingcore.DefaultWatchReactor(watch.NewFake(), nil))

		// create the admission plugin
		p := NewSCCExecRestrictions(tc)
		p.constraintAdmission.sccLister = &oscache.IndexerToSecurityContextConstraintsLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}

		attrs := kadmission.NewAttributesRecord(v.pod, v.oldPod, kapi.Kind("Pod").WithVersion("version"), "namespace", "pod-name", kapi.Resource(v.resource).WithVersion("version"), v.subresource, v.operation, &user.DefaultInfo{})
		err := p.Admit(attrs)

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

package admission

import (
	"testing"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/runtime"
)

// scc exec is a pass through to *constraint, so we only need to test that
// it correctly limits its actions to certain conditions
func TestExecAdmit(t *testing.T) {

	goodPod := func() *kapi.Pod {
		return &kapi.Pod{
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

		pod                    *kapi.Pod
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
		tc := testclient.NewSimpleFake()
		tc.ReactFn = func(a testclient.Action) (runtime.Object, error) {
			if a.Matches("get", "pods") {
				return v.pod, nil
			}

			return nil, nil
		}

		// create the admission plugin
		p := NewSCCExecRestrictions(tc)

		attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "namespace", "", v.resource, v.subresource, v.operation, &user.DefaultInfo{})
		err := p.Admit(attrs)

		if v.shouldAdmit && err != nil {
			t.Errorf("%s: expected no errors but received %v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s: expected errors but received none", k)
		}

		if !v.shouldHaveClientAction && (len(tc.Actions()) > 0) {
			t.Errorf("%s: unexpected actions: %v", k, tc.Actions())
		}
		if v.shouldHaveClientAction && (len(tc.Actions()) == 0) {
			t.Errorf("%s: no actions found", k)
		}

		if v.shouldHaveClientAction {
			if len(v.pod.Spec.ServiceAccountName) != 0 {
				t.Errorf("%s: sa name should have been cleared: %v", k, v.pod.Spec.ServiceAccountName)
			}
		}
	}
}

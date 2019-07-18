package podtolerations

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

func TestPodTolerations(t *testing.T) {
	ns := metav1.NamespaceDefault
	tests := []struct {
		resource      runtime.Object
		kind          schema.GroupKind
		groupresource schema.GroupResource
		userinfo      user.Info
		expectFailure bool
	}{
		{
			resource:      podWithTolerations(),
			userinfo:      serviceaccount.UserInfo("default", "ravi", ""),
			expectFailure: true,
		},
		{
			resource:      podWithTolerations(),
			userinfo:      serviceaccount.UserInfo("openshift-kube-apiserver", "superuser", ""),
			expectFailure: false,
		},
		{
			resource:      podWithNoTolerations(),
			userinfo:      serviceaccount.UserInfo("default", "ravi", ""),
			expectFailure: false,
		},
	}
	for i, test := range tests {
		pt := NewPodTolerationsPlugin()
		pt.(initializer.WantsAuthorizer).SetAuthorizer(fakeAuthorizer(t))
		pt.(admission.InitializationValidator).ValidateInitialization()
		attrs := admission.NewAttributesRecord(test.resource, nil, coreapi.Kind("Pod").WithVersion("version"), ns, "test", coreapi.Resource("pods").WithVersion("version"), "", admission.Create, false, test.userinfo)
		actualError := false
		err := pt.(admission.ValidationInterface).Validate(attrs, nil)
		if err != nil {
			actualError = true
		}
		if actualError != test.expectFailure {
			t.Fatalf("Expected test result is different for test case %v with error %v", i, err)
		}
	}
}

func podWithTolerations() *coreapi.Pod {
	pod := &coreapi.Pod{}
	pod.Spec.Tolerations = []coreapi.Toleration{{Key: "node-role.kubernetes.io/master", Value: "", Operator: coreapi.TolerationOpExists}}
	return pod
}

func podWithNoTolerations() *coreapi.Pod {
	return &coreapi.Pod{}
}

type fakeTestAuthorizer struct {
	t *testing.T
}

func fakeAuthorizer(t *testing.T) authorizer.Authorizer {
	return &fakeTestAuthorizer{
		t: t,
	}
}

func (a *fakeTestAuthorizer) Authorize(attributes authorizer.Attributes) (authorizer.Decision, string, error) {
	ui := attributes.GetUser()
	if ui == nil {
		return authorizer.DecisionNoOpinion, "", fmt.Errorf("No valid UserInfo for Context")
	}
	// User with tolerations permission:
	if ui.GetName() == "system:serviceaccount:openshift-kube-apiserver:superuser" {
		return authorizer.DecisionAllow, "", nil
	}
	// User without tolerations permission:
	return authorizer.DecisionNoOpinion, "", nil
}

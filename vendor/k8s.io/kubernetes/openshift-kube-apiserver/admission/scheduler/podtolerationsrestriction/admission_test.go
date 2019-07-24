package podtolerationsrestriction

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
	"reflect"
)

func TestPodTolerations(t *testing.T) {
	ns := metav1.NamespaceDefault
	tests := []struct {
		resource           runtime.Object
		kind               schema.GroupKind
		serviceaccountName string
		groupresource      schema.GroupResource
		expectFailure      bool
	}{
		{
			resource:      podWithTolerations("ravi"),
			expectFailure: true,
		},
		{
			resource:      podWithTolerations("superuser"),
			expectFailure: false,
		},
		{
			resource:      podWithNoTolerations(),
			expectFailure: false,
		},
	}
	for i, test := range tests {
		pt := NewPodTolerationsPlugin()
		pt.(initializer.WantsAuthorizer).SetAuthorizer(fakeAuthorizer(t))
		pt.(admission.InitializationValidator).ValidateInitialization()
		attrs := admission.NewAttributesRecord(test.resource, nil, coreapi.Kind("Pod").WithVersion("version"), ns, "test", coreapi.Resource("pods").WithVersion("version"), "", admission.Create, false, nil)
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

func podWithTolerations(serviceaccountName string) *coreapi.Pod {
	pod := &coreapi.Pod{}
	pod.Spec.Tolerations = []coreapi.Toleration{{Key: "node-role.kubernetes.io/master", Value: "", Operator: coreapi.TolerationOpExists}}
	pod.Spec.ServiceAccountName = serviceaccountName
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
	if ui.GetName() == "system:serviceaccount:default:superuser" {
		return authorizer.DecisionAllow, "", nil
	}
	// User with tolerations permission:
	if ui.GetName() == "system:serviceaccount" {
		return authorizer.DecisionAllow, "", nil
	}
	// User without tolerations permission:
	return authorizer.DecisionNoOpinion, "", nil
}

func TestBuildAuthorizationAttributes(t *testing.T) {
	tests := []struct {
		tolerations  coreapi.Toleration
		userInfo     user.Info
		namespace    string
		expectedAttr authorizer.AttributesRecord
	}{
		{
			tolerations: coreapi.Toleration{Key: "node-role.kubernetes.io/master", Value: "", Operator: coreapi.TolerationOpExists},
			userInfo:    serviceaccount.UserInfo("default", "ravi", ""),
			namespace:   "default",
			expectedAttr: authorizer.AttributesRecord{
				User:            serviceaccount.UserInfo("default", "ravi", ""),
				Verb:            "Exists",
				Namespace:       "default",
				Resource:        "node-role.kubernetes.io/master",
				APIGroup:        "toleration.scheduling.openshift.io",
				ResourceRequest: true,
				Name:            ":",
			},
		},
		// Empty key with exists maps to all tolerations, so it should get master tolerations.
		{
			tolerations: coreapi.Toleration{Key: "", Value: "", Operator: coreapi.TolerationOpExists, Effect: "NoSchedule"},
			userInfo:    serviceaccount.UserInfo("default", "ravi", ""),
			namespace:   "default",
			expectedAttr: authorizer.AttributesRecord{
				User:            serviceaccount.UserInfo("default", "ravi", ""),
				Verb:            "Exists",
				Namespace:       "default",
				Resource:        "node-role.kubernetes.io/master",
				APIGroup:        "toleration.scheduling.openshift.io",
				ResourceRequest: true,
				Name:            "NoSchedule:",
			},
		},
		// Empty key with exists maps to all tolerations, so it should get master tolerations.
		{
			tolerations: coreapi.Toleration{Key: "test", Value: "value", Operator: coreapi.TolerationOpExists, Effect: "NoSchedule"},
			userInfo:    serviceaccount.UserInfo("default", "ravi", ""),
			namespace:   "default",
			expectedAttr: authorizer.AttributesRecord{
				User:            serviceaccount.UserInfo("default", "ravi", ""),
				Verb:            "Exists",
				Namespace:       "default",
				Resource:        "test",
				APIGroup:        "toleration.scheduling.openshift.io",
				ResourceRequest: true,
				Name:            "NoSchedule:value",
			},
		},
	}

	for _, test := range tests {
		actualAttr := buildTolerationsAuthorizationAttributes(test.userInfo, test.namespace, test.tolerations)
		if !reflect.DeepEqual(test.expectedAttr, actualAttr) {
			t.Fatalf("Got authz attributes as %v but expected %v", actualAttr, test.expectedAttr)
		}
	}
}

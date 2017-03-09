package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

func TestValidation(t *testing.T) {
	if errs := Validate(&api.ImagePolicyConfig{}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&api.ImagePolicyConfig{
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{
				ImageCondition: api.ImageCondition{
					MatchImageLabels: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"test": "other"}},
					},
				},
			},
		},
	}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&api.ImagePolicyConfig{
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{
				ImageCondition: api.ImageCondition{
					MatchImageLabels: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"": ""}},
					},
				},
			},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&api.ImagePolicyConfig{
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{Name: "test"}},
			{ImageCondition: api.ImageCondition{Name: "test"}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}

	if errs := Validate(&api.ImagePolicyConfig{
		ResolveImages: api.DoNotAttempt,
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{Name: "test", MatchDockerImageLabels: []api.ValueCondition{{}}}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&api.ImagePolicyConfig{
		ResolveImages: api.DoNotAttempt,
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{Name: "test", MatchImageLabels: []metav1.LabelSelector{{}}}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&api.ImagePolicyConfig{
		ResolveImages: api.DoNotAttempt,
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{Name: "test", MatchImageAnnotations: []api.ValueCondition{{}}}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
}

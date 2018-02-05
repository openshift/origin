package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/image/admission/apis/imagepolicy"
)

func TestValidation(t *testing.T) {
	if errs := Validate(&imagepolicy.ImagePolicyConfig{}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ExecutionRules: []imagepolicy.ImageExecutionPolicyRule{
			{
				ImageCondition: imagepolicy.ImageCondition{
					MatchImageLabels: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"test": "other"}},
					},
				},
			},
		},
	}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ExecutionRules: []imagepolicy.ImageExecutionPolicyRule{
			{
				ImageCondition: imagepolicy.ImageCondition{
					MatchImageLabels: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"": ""}},
					},
				},
			},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ExecutionRules: []imagepolicy.ImageExecutionPolicyRule{
			{ImageCondition: imagepolicy.ImageCondition{Name: "test"}},
			{ImageCondition: imagepolicy.ImageCondition{Name: "test"}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}

	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ResolveImages: imagepolicy.DoNotAttempt,
		ResolutionRules: []imagepolicy.ImageResolutionPolicyRule{
			{TargetResource: metav1.GroupResource{Resource: "test"}, Policy: imagepolicy.Attempt},
		},
	}); len(errs) != 0 {
		t.Fatal(errs)
	}

	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ResolveImages: imagepolicy.DoNotAttempt,
		ResolutionRules: []imagepolicy.ImageResolutionPolicyRule{
			{TargetResource: metav1.GroupResource{Resource: "test"}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}

	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ResolveImages: imagepolicy.DoNotAttempt,
		ResolutionRules: []imagepolicy.ImageResolutionPolicyRule{
			{Policy: imagepolicy.Attempt},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}

	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ResolveImages: imagepolicy.DoNotAttempt,
		ExecutionRules: []imagepolicy.ImageExecutionPolicyRule{
			{ImageCondition: imagepolicy.ImageCondition{Name: "test", MatchDockerImageLabels: []imagepolicy.ValueCondition{{}}}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ResolveImages: imagepolicy.DoNotAttempt,
		ExecutionRules: []imagepolicy.ImageExecutionPolicyRule{
			{ImageCondition: imagepolicy.ImageCondition{Name: "test", MatchImageLabels: []metav1.LabelSelector{{}}}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
	if errs := Validate(&imagepolicy.ImagePolicyConfig{
		ResolveImages: imagepolicy.DoNotAttempt,
		ExecutionRules: []imagepolicy.ImageExecutionPolicyRule{
			{ImageCondition: imagepolicy.ImageCondition{Name: "test", MatchImageAnnotations: []imagepolicy.ValueCondition{{}}}},
		},
	}); len(errs) == 0 {
		t.Fatal(errs)
	}
}

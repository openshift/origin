package rules

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/admission/apis/imagepolicy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func imageref(name string) imageapi.DockerImageReference {
	ref, err := imageapi.ParseDockerImageReference(name)
	if err != nil {
		panic(err)
	}
	return ref
}

type acceptResult struct {
	attr   ImagePolicyAttributes
	result bool
}

func TestAccept(t *testing.T) {
	podResource := schema.GroupResource{Resource: "pods"}

	testCases := map[string]struct {
		rules   []imagepolicy.ImageExecutionPolicyRule
		matcher RegistryMatcher
		covers  map[schema.GroupResource]bool
		accepts []acceptResult
	}{
		"empty": {
			matcher: nameSet{},
			covers: map[schema.GroupResource]bool{
				{}: false,
			},
		},
		"accepts when rules are empty": {
			rules: []imagepolicy.ImageExecutionPolicyRule{},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, true},
			},
		},
		"when all rules are deny, match everything else": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{Reject: true, ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchIntegratedRegistry: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io/namespace/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"deny rule and accept rule": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}}},
				{Reject: true, ImageCondition: imagepolicy.ImageCondition{
					OnResources:     []schema.GroupResource{podResource},
					MatchRegistries: []string{"index.docker.io"},
				}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Image: &imageapi.Image{}}, true},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Resource: podResource, Name: imageref("index.docker.io/namespace/test:latest")}, false},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"exclude a deny rule": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{Reject: true, ImageCondition: imagepolicy.ImageCondition{Name: "excluded-rule", OnResources: []schema.GroupResource{podResource}, MatchIntegratedRegistry: true, SkipOnResolutionFailure: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Resource: podResource, Name: imageref("myregistry.io/namespace/test:latest")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"invert a deny rule": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{InvertMatch: true, OnResources: []schema.GroupResource{podResource}, MatchIntegratedRegistry: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io/namespace/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"reject an inverted deny rule": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{Reject: true, ImageCondition: imagepolicy.ImageCondition{InvertMatch: true, OnResources: []schema.GroupResource{podResource}, MatchIntegratedRegistry: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io/namespace/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, false},
			},
		},
		"flags image resolution failure on matching resources": {
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, SkipOnResolutionFailure: false}},
			},
			accepts: []acceptResult{
				// allowed because they are on different resources
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry.io:5000/test:latest")}, true},
				// succeeds because no image and skip resolution is true
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
				// succeeds because an image specified
				{ImagePolicyAttributes{
					Resource: podResource,
					Name:     imageref("test:latest"),
					Image:    &imageapi.Image{},
				}, true},
			},
		},
		"accepts matching registries": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchRegistries: []string{"myregistry.io"}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io:5000/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry.io/namespace/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, false},
			},
		},
		"accepts matching image labels": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageLabels: []metav1.LabelSelector{{MatchLabels: map[string]string{"label1": "value1"}}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label1": "value2"}}}}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching multiple image label values": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageLabels: []metav1.LabelSelector{{MatchLabels: map[string]string{"label1": "value1"}}}}},
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageLabels: []metav1.LabelSelector{{MatchLabels: map[string]string{"label1": "value2"}}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching image labels by key": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageLabels: []metav1.LabelSelector{{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "label1", Operator: metav1.LabelSelectorOpExists}}}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching image annotations": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageAnnotations: []imagepolicy.ValueCondition{{Key: "label1", Value: "value1"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label1": "value2"}}}}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching multiple image annotations values": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageAnnotations: []imagepolicy.ValueCondition{{Key: "label1", Value: "value1"}}}},
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageAnnotations: []imagepolicy.ValueCondition{{Key: "label1", Value: "value2"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching image annotations by key": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchImageAnnotations: []imagepolicy.ValueCondition{{Key: "label1", Set: true}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching docker image labels": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchDockerImageLabels: []imagepolicy.ValueCondition{{Key: "label1", Value: "value1"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value1"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value2"}}}}}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label2": "value1"}}}}}, false},
			},
		},
		"accepts matching multiple docker image label values": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchDockerImageLabels: []imagepolicy.ValueCondition{{Key: "label1", Value: "value1"}}}},
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchDockerImageLabels: []imagepolicy.ValueCondition{{Key: "label1", Value: "value2"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value1"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value2"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label2": "value1"}}}}}, false},
			},
		},
		"accepts matching docker image labels by key": {
			matcher: NewRegistryMatcher([]string{"myregistry.io:5000"}),
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource}, MatchDockerImageLabels: []imagepolicy.ValueCondition{{Key: "label1", Set: true}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value1"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value2"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label2": "value1"}}}}}, false},
			},
		},
		"covers calculations": {
			rules: []imagepolicy.ImageExecutionPolicyRule{
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{podResource, {Resource: "services"}}}},
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{{Resource: "services", Group: "extra"}}}},
				{ImageCondition: imagepolicy.ImageCondition{OnResources: []schema.GroupResource{{Resource: "nodes", Group: "extra"}}}},
			},
			matcher: nameSet{},
			covers: map[schema.GroupResource]bool{
				podResource:                            true,
				{Resource: "services"}:                 true,
				{Group: "extra", Resource: "services"}: true,
				{Group: "extra", Resource: "nodes"}:    true,
				{Resource: "nodes"}:                    false,
			},
		},
	}
	for test, testCase := range testCases {
		a, err := NewExecutionRulesAccepter(testCase.rules, testCase.matcher)
		if err != nil {
			t.Fatalf("%s: %v", test, err)
		}
		for k, v := range testCase.covers {
			result := a.Covers(k)
			if result != v {
				t.Errorf("%s: expected Covers(%v)=%t, got %t", test, k, v, result)
			}
		}
		for _, v := range testCase.accepts {
			result := a.Accepts(&v.attr)
			if result != v.result {
				t.Errorf("%s: expected Accepts(%#v)=%t, got %t", test, v.attr, v.result, result)
			}
		}
	}
}

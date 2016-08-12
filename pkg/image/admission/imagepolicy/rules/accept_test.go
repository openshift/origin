package rules

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
	podResource := unversioned.GroupResource{Resource: "pods"}

	testCases := map[string]struct {
		rules         []api.ImageExecutionPolicyRule
		matcher       RegistryMatcher
		covers        map[unversioned.GroupResource]bool
		requiresImage map[unversioned.GroupResource]bool
		resolvesImage map[unversioned.GroupResource]bool
		accepts       []acceptResult
	}{
		"empty": {
			matcher: nameSet{},
			covers: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{}: false,
			},
			requiresImage: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{}: false,
			},
			resolvesImage: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{}: false,
			},
		},
		"mixed resolution": {
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource, {Resource: "services"}}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{{Resource: "services", Group: "extra"}}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{{Resource: "nodes", Group: "extra"}}}, Resolve: true},
			},
			matcher: nameSet{},
			covers: map[unversioned.GroupResource]bool{
				podResource: true,
				unversioned.GroupResource{Resource: "services"}:                 true,
				unversioned.GroupResource{Group: "extra", Resource: "services"}: true,
				unversioned.GroupResource{Group: "extra", Resource: "nodes"}:    true,
				unversioned.GroupResource{Resource: "nodes"}:                    false,
			},
			requiresImage: map[unversioned.GroupResource]bool{
				podResource: false,
				unversioned.GroupResource{Resource: "services"}:                 false,
				unversioned.GroupResource{Group: "extra", Resource: "services"}: false,
				unversioned.GroupResource{Group: "extra", Resource: "nodes"}:    true,
				unversioned.GroupResource{Resource: "nodes"}:                    false,
			},
			resolvesImage: map[unversioned.GroupResource]bool{
				podResource: false,
				unversioned.GroupResource{Resource: "services"}:                 false,
				unversioned.GroupResource{Group: "extra", Resource: "services"}: false,
				unversioned.GroupResource{Group: "extra", Resource: "nodes"}:    true,
				unversioned.GroupResource{Resource: "nodes"}:                    false,
			},
		},
		"mixed requires image": {
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{
					OnResources:            []unversioned.GroupResource{{Resource: "a"}},
					MatchDockerImageLabels: []api.ValueCondition{{Key: "test", Value: "value"}},
				}},
				{ImageCondition: api.ImageCondition{
					OnResources:           []unversioned.GroupResource{{Resource: "b"}},
					MatchImageAnnotations: []api.ValueCondition{{Key: "test", Value: "value"}},
				}},
				{ImageCondition: api.ImageCondition{
					OnResources:      []unversioned.GroupResource{{Resource: "c"}},
					MatchImageLabels: []unversioned.LabelSelector{{MatchLabels: map[string]string{"test": "value"}}},
				}},
			},
			matcher: nameSet{},
			requiresImage: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{Resource: "a"}: true,
				unversioned.GroupResource{Resource: "b"}: true,
				unversioned.GroupResource{Resource: "c"}: true,
				unversioned.GroupResource{Resource: "d"}: false,
			},
			resolvesImage: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{Resource: "a"}: false,
				unversioned.GroupResource{Resource: "b"}: false,
				unversioned.GroupResource{Resource: "c"}: false,
				unversioned.GroupResource{Resource: "d"}: false,
			},
		},
		"accepts when rules are empty": {
			rules: []api.ImageExecutionPolicyRule{},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, true},
			},
		},
		"when all rules are deny, match everything else": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{Reject: true, ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchIntegratedRegistry: true, AllowResolutionFailure: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry/namespace/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"deny rule and accept rule": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}}, Resolve: true},
				{Reject: true, ImageCondition: api.ImageCondition{
					OnResources:     []unversioned.GroupResource{podResource},
					MatchRegistries: []string{"index.docker.io"},
				}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Image: &imageapi.Image{}}, true},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Resource: podResource, Name: imageref("index.docker.io/namespace/test:latest")}, false},
				{ImagePolicyAttributes{Image: &imageapi.Image{}, Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"exclude a deny rule": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{Reject: true, ImageCondition: api.ImageCondition{Name: "excluded-rule", OnResources: []unversioned.GroupResource{podResource}, MatchIntegratedRegistry: true, AllowResolutionFailure: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Resource: podResource, Name: imageref("myregistry/namespace/test:latest")}, true},
				{ImagePolicyAttributes{ExcludedRules: sets.NewString("excluded-rule"), Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"invert a deny rule": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{InvertMatch: true, OnResources: []unversioned.GroupResource{podResource}, MatchIntegratedRegistry: true, AllowResolutionFailure: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry/namespace/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, true},
			},
		},
		"reject an inverted deny rule": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{Reject: true, ImageCondition: api.ImageCondition{InvertMatch: true, OnResources: []unversioned.GroupResource{podResource}, MatchIntegratedRegistry: true, AllowResolutionFailure: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry/namespace/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, false},
			},
		},
		"flags image resolution failure on matching resources": {
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, AllowResolutionFailure: false}},
			},
			accepts: []acceptResult{
				// allowed because they are on different resources
				{ImagePolicyAttributes{}, true},
				{ImagePolicyAttributes{Name: imageref("myregistry:5000/test:latest")}, true},
				// fails because no image
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, false},
				// succeeds because an image specified
				{ImagePolicyAttributes{
					Resource: podResource,
					Name:     imageref("test:latest"),
					Image:    &imageapi.Image{},
				}, true},
			},
		},
		"accepts matching registries": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchRegistries: []string{"myregistry"}, AllowResolutionFailure: true}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry/namespace/test:latest")}, true},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, false},
			},
		},
		"accepts matching image labels": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageLabels: []unversioned.LabelSelector{{MatchLabels: map[string]string{"label1": "value1"}}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label1": "value2"}}}}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching multiple image label values": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageLabels: []unversioned.LabelSelector{{MatchLabels: map[string]string{"label1": "value1"}}}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageLabels: []unversioned.LabelSelector{{MatchLabels: map[string]string{"label1": "value2"}}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching image labels by key": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageLabels: []unversioned.LabelSelector{{MatchExpressions: []unversioned.LabelSelectorRequirement{{Key: "label1", Operator: unversioned.LabelSelectorOpExists}}}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching image annotations": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageAnnotations: []api.ValueCondition{{Key: "label1", Value: "value1"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label1": "value2"}}}}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching multiple image annotations values": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageAnnotations: []api.ValueCondition{{Key: "label1", Value: "value1"}}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageAnnotations: []api.ValueCondition{{Key: "label1", Value: "value2"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching image annotations by key": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchImageAnnotations: []api.ValueCondition{{Key: "label1", Set: true}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label1": "value1"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label1": "value2"}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Annotations: map[string]string{"label2": "value1"}}}}, false},
			},
		},
		"accepts matching docker image labels": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchDockerImageLabels: []api.ValueCondition{{Key: "label1", Value: "value1"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value1"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value2"}}}}}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label2": "value1"}}}}}, false},
			},
		},
		"accepts matching multiple docker image label values": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchDockerImageLabels: []api.ValueCondition{{Key: "label1", Value: "value1"}}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchDockerImageLabels: []api.ValueCondition{{Key: "label1", Value: "value2"}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value1"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value2"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label2": "value1"}}}}}, false},
			},
		},
		"accepts matching docker image labels by key": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchDockerImageLabels: []api.ValueCondition{{Key: "label1", Set: true}}}},
			},
			accepts: []acceptResult{
				{ImagePolicyAttributes{Resource: podResource}, false},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value1"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label1": "value2"}}}}}, true},
				{ImagePolicyAttributes{Resource: podResource, Image: &imageapi.Image{DockerImageMetadata: imageapi.DockerImage{Config: &imageapi.DockerConfig{Labels: map[string]string{"label2": "value1"}}}}}, false},
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
		for k, v := range testCase.requiresImage {
			result := a.RequiresImage(k)
			if result != v {
				t.Errorf("%s: expected RequiresImage(%v)=%t, got %t", test, k, v, result)
			}
		}
		for k, v := range testCase.resolvesImage {
			result := a.ResolvesImage(k)
			if result != v {
				t.Errorf("%s: expected RequiresImage(%v)=%t, got %t", test, k, v, result)
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

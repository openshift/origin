package rules

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type adjustResult struct {
	attr   ImagePolicyAttributes
	before *kapi.PodSpec
	after  *kapi.PodSpec
	result bool
}

func TestConsume(t *testing.T) {
	podResource := unversioned.GroupResource{Resource: "pods"}
	simplePod := &kapi.PodSpec{
		Containers: []kapi.Container{
			{Image: "test"},
		},
		InitContainers: []kapi.Container{
			{Image: "test"},
			{Image: "test2"},
		},
	}
	afterSimplePod := &kapi.PodSpec{
		Containers: []kapi.Container{
			{
				Image: "test",
				Resources: kapi.ResourceRequirements{
					Requests: kapi.ResourceList{
						"experimental.license": resource.MustParse("1"),
					},
				},
			},
		},
		InitContainers: []kapi.Container{
			{
				Image: "test",
				Resources: kapi.ResourceRequirements{
					Requests: kapi.ResourceList{
						"experimental.license": resource.MustParse("1"),
					},
				},
			},
			{Image: "test2"},
		},
	}
	altAfterSimplePod := &kapi.PodSpec{
		Containers: []kapi.Container{
			{Image: "test"},
		},
		InitContainers: []kapi.Container{
			{Image: "test"},
			{
				Image: "test2",
				Resources: kapi.ResourceRequirements{
					Requests: kapi.ResourceList{
						"experimental.license": resource.MustParse("1"),
					},
				},
			},
		},
	}

	testCases := map[string]struct {
		rules         []api.ImageConsumptionPolicyRule
		matcher       RegistryMatcher
		covers        map[unversioned.GroupResource]bool
		requiresImage map[unversioned.GroupResource]bool
		adjusts       []adjustResult
	}{
		"empty": {
			matcher: nameSet{},
			covers: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{}: false,
			},
			requiresImage: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{}: false,
			},
		},
		"mixed resolution": {
			rules: []api.ImageConsumptionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource, {Resource: "services"}}}, Add: []api.ConsumeResourceEffect{{Name: "test-rule", Quantity: "experimental.license"}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{{Resource: "services", Group: "extra"}}}, Add: []api.ConsumeResourceEffect{{Name: "test-rule", Quantity: "experimental.license"}}},
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{{Resource: "nodes", Group: "extra"}}}, Add: []api.ConsumeResourceEffect{{Name: "test-rule", Quantity: "experimental.license"}}},
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
				unversioned.GroupResource{Group: "extra", Resource: "nodes"}:    false,
				unversioned.GroupResource{Resource: "nodes"}:                    false,
			},
		},
		"mixed requires image": {
			rules: []api.ImageConsumptionPolicyRule{
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
					MatchImageLabels: []api.ValueCondition{{Key: "test", Value: "value"}},
				}},
				{ImageCondition: api.ImageCondition{
					OnResources:     []unversioned.GroupResource{{Resource: "d"}},
					MatchSignatures: []api.SignatureMatch{{}},
				}},
			},
			matcher: nameSet{},
			requiresImage: map[unversioned.GroupResource]bool{
				unversioned.GroupResource{Resource: "a"}: true,
				unversioned.GroupResource{Resource: "b"}: true,
				unversioned.GroupResource{Resource: "c"}: true,
				unversioned.GroupResource{Resource: "d"}: true,
				unversioned.GroupResource{Resource: "e"}: false,
			},
		},
		"no adjustment when rules are empty": {
			rules: []api.ImageConsumptionPolicyRule{},
			adjusts: []adjustResult{
				{ImagePolicyAttributes{}, nil, nil, false},
				{ImagePolicyAttributes{Name: imageref("test:latest")}, nil, nil, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, nil, nil, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("myregistry:5000/test:latest")}, nil, nil, false},
			},
		},
		"without image or effect, all rules fail": {
			rules: []api.ImageConsumptionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, AllowResolutionFailure: false}},
			},
			adjusts: []adjustResult{
				// no effect registered
				{ImagePolicyAttributes{}, nil, nil, false},
				{ImagePolicyAttributes{Name: imageref("myregistry:5000/test:latest")}, nil, nil, false},
				{ImagePolicyAttributes{Resource: podResource, Name: imageref("test:latest")}, nil, nil, false},
				{ImagePolicyAttributes{
					Resource: podResource,
					Name:     imageref("test:latest"),
					Image:    &imageapi.Image{},
				}, nil, nil, false},
			},
		},
		"with image and effect": {
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, AllowResolutionFailure: false},
					Add: []api.ConsumeResourceEffect{
						{Name: "experimental.license", Quantity: "1"},
					},
				},
			},
			adjusts: []adjustResult{
				// succeeds because an image specified
				{
					attr: ImagePolicyAttributes{
						Resource:     podResource,
						Name:         imageref("test:latest"),
						OriginalName: "test",
						Image:        &imageapi.Image{},
					},
					before: simplePod,
					after:  afterSimplePod,
					result: true,
				},
				// succeeds, but matches the other init container
				{
					attr: ImagePolicyAttributes{
						Resource:     podResource,
						Name:         imageref("test:latest"),
						OriginalName: "test2",
						Image:        &imageapi.Image{},
					},
					before: simplePod,
					after:  altAfterSimplePod,
					result: true,
				},
			},
		},
		"skips when rule is excluded": {
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{Name: "excluded-rule", OnResources: []unversioned.GroupResource{podResource}, AllowResolutionFailure: false},
					Add: []api.ConsumeResourceEffect{
						{Name: "experimental.license", Quantity: "1"},
					},
				},
			},
			adjusts: []adjustResult{
				{
					attr: ImagePolicyAttributes{
						Resource:      podResource,
						Name:          imageref("test:latest"),
						OriginalName:  "test",
						Image:         &imageapi.Image{},
						ExcludedRules: sets.NewString("excluded-rule"),
					},
					before: simplePod,
					after:  simplePod,
					result: false,
				},
			},
		},
		"with resource name from docker image label": {
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}},
					Add: []api.ConsumeResourceEffect{
						{NameFromDockerImageLabel: "license.key", Quantity: "100"},
					},
				},
			},
			adjusts: []adjustResult{
				// no config section on image
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest"), OriginalName: "test",
						Image: &imageapi.Image{},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after:  &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					result: false,
				},
				// no image label found
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest"), OriginalName: "test",
						Image: &imageapi.Image{
							DockerImageMetadata: imageapi.DockerImage{
								Config: &imageapi.DockerConfig{
									Labels: map[string]string{"other": "value"},
								},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after:  &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					result: false,
				},
				// label value found
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest"), OriginalName: "test",
						Image: &imageapi.Image{
							DockerImageMetadata: imageapi.DockerImage{
								Config: &imageapi.DockerConfig{
									Labels: map[string]string{"license.key": "experimental.license"},
								},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"experimental.license": resource.MustParse("100"),
								},
							},
						},
					}},
					result: true,
				},
			},
		},
		"with resource name from image annotation": {
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}},
					Add: []api.ConsumeResourceEffect{
						{NameFromImageAnnotation: "license.key", Quantity: "100"},
					},
				},
			},
			adjusts: []adjustResult{
				// no config section on image
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest"), OriginalName: "test",
						Image: &imageapi.Image{},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after:  &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					result: false,
				},
				// no image label found
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest"), OriginalName: "test",
						Image: &imageapi.Image{
							ObjectMeta: kapi.ObjectMeta{
								Annotations: map[string]string{"other": "value"},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after:  &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					result: false,
				},
				// label value found
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest"), OriginalName: "test",
						Image: &imageapi.Image{
							ObjectMeta: kapi.ObjectMeta{
								Annotations: map[string]string{"license.key": "experimental.license"},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"experimental.license": resource.MustParse("100"),
								},
							},
						},
					}},
					result: true,
				},
			},
		},
		"with resource quantity from image annotation": {
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}},
					Add: []api.ConsumeResourceEffect{
						{Name: "cpu", Quantity: "1", QuantityFromImageAnnotation: "image.defaultCPU"},
						{Name: "memory", QuantityFromImageAnnotation: "image.defaultMemory"},
					},
				},
			},
			adjusts: []adjustResult{
				// no image annotation found, default is set
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest1"), OriginalName: "test",
						Image: &imageapi.Image{
							ObjectMeta: kapi.ObjectMeta{
								Annotations: map[string]string{"other": "value"},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"cpu": resource.MustParse("1"),
								},
							},
						},
					}},
					result: true,
				},
				// invalid quantity falls back to default
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest2"), OriginalName: "test",
						Image: &imageapi.Image{
							ObjectMeta: kapi.ObjectMeta{
								Annotations: map[string]string{"image.defaultCPU": "500zaoeunthx"},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"cpu": resource.MustParse("1"),
								},
							},
						},
					}},
					result: true,
				},
				// set both quantities
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest3"), OriginalName: "test",
						Image: &imageapi.Image{
							ObjectMeta: kapi.ObjectMeta{
								Annotations: map[string]string{
									"image.defaultCPU":    "5",
									"image.defaultMemory": "512Mi",
								},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("512Mi"),
								},
							},
						},
					}},
					result: true,
				},
			},
		},
		"with resource quantity from docker image label": {
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}},
					Add: []api.ConsumeResourceEffect{
						{Name: "cpu", Quantity: "1", QuantityFromDockerImageLabel: "image.defaultCPU"},
						{Name: "memory", QuantityFromDockerImageLabel: "image.defaultMemory"},
					},
				},
			},
			adjusts: []adjustResult{
				// no image annotation found, default is set
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest1"), OriginalName: "test",
						Image: &imageapi.Image{
							DockerImageMetadata: imageapi.DockerImage{
								Config: &imageapi.DockerConfig{
									Labels: map[string]string{"other": "value"},
								},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"cpu": resource.MustParse("1"),
								},
							},
						},
					}},
					result: true,
				},
				// invalid quantity falls back to default
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest2"), OriginalName: "test",
						Image: &imageapi.Image{
							DockerImageMetadata: imageapi.DockerImage{
								Config: &imageapi.DockerConfig{
									Labels: map[string]string{"image.defaultCPU": "500zaoeunthx"},
								},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"cpu": resource.MustParse("1"),
								},
							},
						},
					}},
					result: true,
				},
				// set both quantities
				{
					attr: ImagePolicyAttributes{
						Resource: podResource, Name: imageref("test:latest3"), OriginalName: "test",
						Image: &imageapi.Image{
							DockerImageMetadata: imageapi.DockerImage{
								Config: &imageapi.DockerConfig{
									Labels: map[string]string{
										"image.defaultCPU":    "5",
										"image.defaultMemory": "512Mi",
									},
								},
							},
						},
					},
					before: &kapi.PodSpec{Containers: []kapi.Container{{Image: "test"}}},
					after: &kapi.PodSpec{Containers: []kapi.Container{
						{
							Image: "test",
							Resources: kapi.ResourceRequirements{
								Requests: kapi.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("512Mi"),
								},
							},
						},
					}},
					result: true,
				},
			},
		},
		"adjust matching registries": {
			matcher: NewRegistryMatcher([]string{"myregistry:5000"}),
			rules: []api.ImageConsumptionPolicyRule{
				{
					ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{podResource}, MatchRegistries: []string{"myregistry"}, AllowResolutionFailure: true},
					Add: []api.ConsumeResourceEffect{
						{Name: "experimental.license", Quantity: "1"},
					},
				},
			},
			adjusts: []adjustResult{
				{ImagePolicyAttributes{Resource: podResource, OriginalName: "test", Name: imageref("myregistry:5000/test:latest")}, simplePod, simplePod, false},
				{ImagePolicyAttributes{Resource: podResource, OriginalName: "test", Name: imageref("myregistry/namespace/test:latest")}, simplePod, afterSimplePod, true},
				{ImagePolicyAttributes{Resource: podResource, OriginalName: "test2", Name: imageref("myregistry/namespace/test:latest")}, simplePod, altAfterSimplePod, true},
				{ImagePolicyAttributes{Resource: podResource, OriginalName: "test", Name: imageref("test:latest")}, simplePod, simplePod, false},
			},
		},
	}
	for test, testCase := range testCases {
		a := NewConsumptionRulesAdjuster(testCase.rules, testCase.matcher)
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
		for _, v := range testCase.adjusts {
			copied, err := kapi.Scheme.DeepCopy(v.before)
			if err != nil {
				t.Fatal(err)
			}
			result := a.Adjust(&v.attr, copied.(*kapi.PodSpec))
			if result != v.result {
				t.Errorf("%s: expected Adjust(%#v)=%t, got %t", test, v.attr, v.result, result)
			}
			if !kapi.Semantic.DeepEqual(copied, v.after) {
				t.Errorf("%s: expected Adjust(%#v) to alter spec: %s", test, v.attr, diff.ObjectReflectDiff(copied, v.after))
			}
		}
	}
}

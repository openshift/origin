package buildconfig

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestCircularDeps(t *testing.T) {
	tests := []struct {
		name     string
		cfgSet   []*buildapi.BuildConfig
		expected bool
	}{
		{
			name:     "no-circular-deps",
			cfgSet:   noCircularDeps(),
			expected: false,
		},
		{
			name:     "with-circular-deps",
			cfgSet:   withCircularDeps(),
			expected: true,
		},
	}

	var got bool
	for _, test := range tests {
		for _, cfg := range test.cfgSet {
			got = circularDeps(cfg)
			if got == true {
				break
			}
		}

		if got != test.expected {
			t.Errorf("%s: Expected circular deps: %t, got: %t", test.name, test.expected, got)
		}
		// Flush deps after a test finishes
		deps = map[string]map[string]string{}
	}
}

func noCircularDeps() []*buildapi.BuildConfig {
	return []*buildapi.BuildConfig{
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "test-repo",
						Namespace: "default",
					},
					Tag: "atag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "latest",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "img-repo",
						Namespace: "default",
					},
					Tag: "scratch",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "tip",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "test-repo",
						Namespace: "default",
					},
					Tag: "release",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "other",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "another-repo",
						Namespace: "default",
					},
					Tag: "outputtag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "other",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "repo",
						Namespace: "default",
					},
					Tag: "latest",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "test-repo",
							Namespace: "default",
						},
						Tag: "atag",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "dummy",
						Namespace: "default",
					},
					Tag: "13.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "test-repo",
							Namespace: "default",
						},
						Tag: "release",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "dummy",
						Namespace: "default",
					},
					Tag: "12.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "test-repo",
							Namespace: "default",
						},
						Tag: "latest",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "some-repo",
						Namespace: "default",
					},
					Tag: "tag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "another-repo",
							Namespace: "default",
						},
						Tag: "outputtag",
					},
				},
			},
		},
	}
}

func withCircularDeps() []*buildapi.BuildConfig {
	return []*buildapi.BuildConfig{
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "test-repo",
						Namespace: "default",
					},
					Tag: "atag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "latest",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "img-repo",
						Namespace: "default",
					},
					Tag: "scratch",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "tip",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "test-repo",
						Namespace: "default",
					},
					Tag: "release",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "other",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "another-repo",
						Namespace: "default",
					},
					Tag: "outputtag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "start",
							Namespace: "default",
						},
						Tag: "other",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "repo",
						Namespace: "default",
					},
					Tag: "latest",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "test-repo",
							Namespace: "default",
						},
						Tag: "atag",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "dummy",
						Namespace: "default",
					},
					Tag: "13.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "test-repo",
							Namespace: "default",
						},
						Tag: "release",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "dummy",
						Namespace: "default",
					},
					Tag: "12.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "test-repo",
							Namespace: "default",
						},
						Tag: "latest",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "some-repo",
						Namespace: "default",
					},
					Tag: "tag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "another-repo",
							Namespace: "default",
						},
						Tag: "outputtag",
					},
				},
			},
		},
		{
			Parameters: buildapi.BuildParameters{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "start",
						Namespace: "default",
					},
					Tag: "other",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: kapi.ObjectReference{
							Name:      "some-repo",
							Namespace: "default",
						},
						Tag: "tag",
					},
				},
			},
		},
	}
}

package buildchain

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// dockerImageReferencesList contains DockerImageReferences instead
// of Tag & To in BuildOutput. It also exercises zeroed fields like
// From.Namespace and From.Tag
//
// Structure of the tree in ast:
// (start (test-repo (repo)(dummy))
//  	  (another-repo (some-repo))
// )
func dockerImageReferencesList() []buildapi.BuildConfig {
	return []buildapi.BuildConfig{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "start-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "start:latest",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "test-repo:atag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "other-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "start:latest",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "another-repo:outputtag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "test-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "test-repo:atag",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repo:latest",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "dummy-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "test-repo:atag",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "dummy:13.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "some-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "another-repo:outputtag",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "some-repo:some-tag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
	}
}

// singleNamespaceList contains multiple edge relationships
// between a parent and a child, all in a single namespace
//
// start:latest -> test-repo:atag (via start-cfg)
// start:tip -> img-repo:scratch (via 2nd-start-cfg)
// start:other -> test-repo:release (via test-cfg)
// start:other -> another-repo:outputtag (via another-cfg)
// test-repo:atag -> repo:latest (via other-cfg)
// test-repo:release -> dummy:13.0 (via dummy-cfg)
// test-repo:latest -> dummy:12.0 (via 2nd-dummy-cfg)
// another-repo:outputtag -> some-repo:tag (via some-cfg)
func singleNamespaceList() []buildapi.BuildConfig {
	return []buildapi.BuildConfig{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "start-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:latest",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "2nd-start-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:tip",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "test-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:other",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "another-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:other",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "other-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "test-repo:atag",
							Namespace: "default",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "repo",
						Namespace: "default",
					},
					Tag: imageapi.DefaultImageTag,
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "dummy-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "test-repo:release",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "2nd-dummy-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "test-repo:latest",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "some-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "another-repo:outputtag",
							Namespace: "default",
						},
					},
				},
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
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
	}
}

// multipleNamespacesList is based on anotherDummyList
// while it adds multiple namespaces
//
// default/start:latest -> test/test-repo:atag
// default/start:tip -> img/img-repo:scratch
// default/start:other -> test/test-repo:release
// default/start:other -> default/another-repo:outputtag
// test/test-repo:atag -> bench/repo:latest (via other-cfg)
// test/test-repo:atag -> bench/dummy:13.0 (via dummy-cfg)
// test/test-repo:latest -> bench/dummy:12.0 (via 2nd-dummy-cfg)
// default/another-repo:out -> bench/some-repo:tag (via some-cfg)
func multipleNamespacesList() []buildapi.BuildConfig {
	return []buildapi.BuildConfig{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "start-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:latest",
							Namespace: "default",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "test-repo",
						Namespace: "test",
					},
					Tag: "atag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "2nd-start-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:tip",
							Namespace: "default",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "img-repo",
						Namespace: "img",
					},
					Tag: "scratch",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "test-cfg",
				Namespace: "test",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:other",
							Namespace: "default",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "test-repo",
						Namespace: "test",
					},
					Tag: "release",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "another-cfg",
				Namespace: "test",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "start:other",
							Namespace: "default",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name: "another-repo",
						// Namespace: "" (will default to the default Namespace)
					},
					Tag: "outputtag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "other-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "test-repo:atag",
							Namespace: "test",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "repo",
						Namespace: "bench",
					},
					Tag: imageapi.DefaultImageTag,
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "dummy-cfg",
				Namespace: "dummy",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "test-repo:atag",
							Namespace: "test",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "dummy",
						Namespace: "bench",
					},
					Tag: "13.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "2nd-dummy-cfg",
				Namespace: "dummy",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Name:      "test-repo:latest",
							Namespace: "test",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "dummy",
						Namespace: "bench",
					},
					Tag: "12.0",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "some-cfg",
				Namespace: "default",
			},
			Parameters: buildapi.BuildParameters{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "another-repo:out",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Name:      "some-repo",
						Namespace: "bench",
					},
					Tag: "tag",
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
	}
}

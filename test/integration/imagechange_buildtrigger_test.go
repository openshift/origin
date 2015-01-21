// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	requireEtcd()
}

func TestSimpleImageChangeBuildTrigger(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	imageRepo := &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-repo"},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"latest": "ref-1",
		},
	}

	config := imageChangeBuildConfig()

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	if imageRepo, err = openshift.Client.ImageRepositories(testNamespace).Create(imageRepo); err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	created, err := openshift.Client.BuildConfigs(testNamespace).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch2, err := openshift.Client.BuildConfigs(testNamespace).Watch(labels.Everything(), labels.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	imageRepo.Tags["latest"] = "ref-2"

	if _, err = openshift.Client.ImageRepositories(testNamespace).Update(imageRepo); err != nil {
		t.Fatalf("Error updating imageRepo: %v", err)
	}

	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)

	if newBuild.Parameters.Strategy.DockerStrategy.BaseImage != "registry:8080/openshift/test-image:ref-2" {
		t.Fatalf("Expected build with base image %s, got %s", "registry:8080/openshift/test-image:ref-2", newBuild.Parameters.Strategy.DockerStrategy.BaseImage)
	}

	event = <-watch2.ResultChan()
	event = <-watch2.ResultChan()

	updatedConfig, err := openshift.Client.BuildConfigs(testNamespace).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	if updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID != "ref-2" {
		t.Errorf("Expected imageID ref-2, got %s", updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID)
	}
}

func imageChangeBuildConfig() *buildapi.BuildConfig {
	buildcfg := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-cfg",
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: "Git",
				Git: &buildapi.GitBuildSource{
					URI: "git://github.com/openshift/ruby-hello-world.git",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "contextimage",
					BaseImage:  "registry:8080/openshift/test-image",
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "foo:tag",
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: "registry:8080/openshift/test-image",
					From: kapi.ObjectReference{
						Name: "test-image-repo",
					},
					Tag: "latest",
				},
			},
		},
	}
	return buildcfg
}

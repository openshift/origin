// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
)

const (
	streamName = "test-image-trigger-repo"
	tag        = "latest"
)

func init() {
	testutil.RequireEtcd()
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagSTI(t *testing.T) {
	clusterAdminClient := setup(t)
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := stiStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig("sti-imagestreamtag", strategy)
	runTest(t, "SimpleImageChangeBuildTriggerFromImageStreamTagSTI", clusterAdminClient, imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagDocker(t *testing.T) {
	clusterAdminClient := setup(t)
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := dockerStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig("docker-imagestreamtag", strategy)
	runTest(t, "SimpleImageChangeBuildTriggerFromImageStreamTagDocker", clusterAdminClient, imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagCustom(t *testing.T) {
	clusterAdminClient := setup(t)
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := customStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig("custom-imagestreamtag", strategy)
	runTest(t, "SimpleImageChangeBuildTriggerFromImageStreamTagCustom", clusterAdminClient, imageStream, imageStreamMapping, config, tag)
}

func dockerStrategy(kind, name string) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{
			From: &kapi.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}
func stiStrategy(kind, name string) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.SourceBuildStrategyType,
		SourceStrategy: &buildapi.SourceBuildStrategy{
			From: kapi.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}
func customStrategy(kind, name string) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}

func imageChangeBuildConfig(name string, strategy buildapi.BuildStrategy) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:      name,
			Namespace: testutil.Namespace(),
			Labels:    map[string]string{"testlabel": "testvalue"},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: "Git",
				Git: &buildapi.GitBuildSource{
					URI: "git://github.com/openshift/ruby-hello-world.git",
				},
				ContextDir: "contextimage",
			},
			Strategy: strategy,
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "test-image-trigger-repo",
				},
				Tag: "outputtag",
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
		},
	}
}

func mockImageStream2(tag string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "test-image-trigger-repo"},

		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:8080/openshift/test-image-trigger",
			Tags: map[string]imageapi.TagReference{
				tag: {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:8080/openshift/test-image-trigger:" + tag,
					},
				},
			},
		},
	}
}

func mockImageStreamMapping(stream, image, tag, reference string) *imageapi.ImageStreamMapping {
	// create a mapping to an image that doesn't exist
	return &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream},
		Tag:        tag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: image,
			},
			DockerImageReference: reference,
		},
	}
}

func setup(t *testing.T) *client.Client {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})
	return clusterAdminClient
}

func runTest(t *testing.T, testname string, clusterAdminClient *client.Client, imageStream *imageapi.ImageStream, imageStreamMapping *imageapi.ImageStreamMapping, config *buildapi.BuildConfig, tag string) {
	created, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	watch2, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	imageStream, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream)
	if err != nil {
		t.Fatalf("Couldn't create ImageStream: %v", err)
	}

	err = clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(imageStreamMapping)
	if err != nil {
		t.Fatalf("Couldn't create Image: %v", err)
	}

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)
	switch newBuild.Parameters.Strategy.Type {
	case buildapi.SourceBuildStrategyType:
		if newBuild.Parameters.Strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Parameters.Strategy.DockerStrategy.From.Name, i, bc.Triggers[0].ImageChange)
		}
	case buildapi.DockerBuildStrategyType:
		if newBuild.Parameters.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Parameters.Strategy.DockerStrategy.From.Name, i, bc.Triggers[0].ImageChange)
		}
	case buildapi.CustomBuildStrategyType:
		if newBuild.Parameters.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Parameters.Strategy.DockerStrategy.From.Name, i, bc.Triggers[0].ImageChange)
		}

	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Parameters.Output.To.Name != "test-image-trigger-repo" || newBuild.Parameters.Output.Tag != "outputtag" || newBuild.Parameters.Output.DockerImageReference != "" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Parameters.Output.To, newBuild.Parameters.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	// wait for build config to be updated
	<-watch2.ResultChan()
	updatedConfig, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	// the first tag did not have an image id, so the last trigger field is the pull spec
	if updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID != "registry:8080/openshift/test-image-trigger:"+tag {
		t.Errorf("Expected imageID equal to pull spec, got %#v", updatedConfig.Triggers[0].ImageChange)
	}

	// trigger a build by posting a new image
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(&imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: testutil.Namespace(),
			Name:      imageStream.Name,
		},
		Tag: tag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "ref-2-random",
			},
			DockerImageReference: "registry:8080/openshift/test-image-trigger:ref-2-random",
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	switch newBuild.Parameters.Strategy.Type {
	case buildapi.SourceBuildStrategyType:
		if newBuild.Parameters.Strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Parameters.Strategy.DockerStrategy.From.Name, i, bc.Triggers[3].ImageChange)
		}
	case buildapi.DockerBuildStrategyType:
		if newBuild.Parameters.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Parameters.Strategy.DockerStrategy.From.Name, i, bc.Triggers[3].ImageChange)
		}
	case buildapi.CustomBuildStrategyType:
		if newBuild.Parameters.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Parameters.Strategy.DockerStrategy.From.Name, i, bc.Triggers[3].ImageChange)
		}
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Parameters.Output.To.Name != "test-image-trigger-repo" || newBuild.Parameters.Output.Tag != "outputtag" || newBuild.Parameters.Output.DockerImageReference != "" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Parameters.Output.To, newBuild.Parameters.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	<-watch2.ResultChan()
	updatedConfig, err = clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	if e, a := "registry:8080/openshift/test-image-trigger:ref-2-random", updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID; e != a {
		t.Errorf("unexpected trigger id: expected %v, got %v", e, a)
	}
}

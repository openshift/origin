// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	watchapi "k8s.io/kubernetes/pkg/watch"

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

func TestSimpleImageChangeBuildTriggerFromImageStreamTagSTIWithConfigChange(t *testing.T) {
	clusterAdminClient := setup(t)
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := stiStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfigWithConfigChange("sti-imagestreamtag", strategy)
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

func TestSimpleImageChangeBuildTriggerFromImageStreamTagDockerWithConfigChange(t *testing.T) {
	clusterAdminClient := setup(t)
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := dockerStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfigWithConfigChange("docker-imagestreamtag", strategy)
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

func TestSimpleImageChangeBuildTriggerFromImageStreamTagCustomWithConfigChange(t *testing.T) {
	clusterAdminClient := setup(t)
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := customStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfigWithConfigChange("custom-imagestreamtag", strategy)
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
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
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
						Kind: "ImageStreamTag",
						Name: "test-image-trigger-repo:outputtag",
					},
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
		},
	}
}
func imageChangeBuildConfigWithConfigChange(name string, strategy buildapi.BuildStrategy) *buildapi.BuildConfig {
	bc := imageChangeBuildConfig(name, strategy)
	bc.Spec.Triggers = append(bc.Spec.Triggers, buildapi.BuildTriggerPolicy{Type: buildapi.ConfigChangeBuildTriggerType})
	return bc
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
	switch newBuild.Spec.Strategy.Type {
	case buildapi.SourceBuildStrategyType:
		if newBuild.Spec.Strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	case buildapi.DockerBuildStrategyType:
		if newBuild.Spec.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	case buildapi.CustomBuildStrategyType:
		if newBuild.Spec.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}

	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Spec.Output.To.Name != "test-image-trigger-repo:outputtag" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
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
	if updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID != "registry:8080/openshift/test-image-trigger:"+tag {
		t.Errorf("Expected imageID equal to pull spec, got %#v", updatedConfig.Spec.Triggers[0].ImageChange)
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
	switch newBuild.Spec.Strategy.Type {
	case buildapi.SourceBuildStrategyType:
		if newBuild.Spec.Strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	case buildapi.DockerBuildStrategyType:
		if newBuild.Spec.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	case buildapi.CustomBuildStrategyType:
		if newBuild.Spec.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Spec.Output.To.Name != "test-image-trigger-repo:outputtag" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	<-watch2.ResultChan()
	updatedConfig, err = clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	if e, a := "registry:8080/openshift/test-image-trigger:ref-2-random", updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID; e != a {
		t.Errorf("unexpected trigger id: expected %v, got %v", e, a)
	}
}

func TestMultipleImageChangeBuildTriggers(t *testing.T) {
	mockImageStream := func(name, tag string) *imageapi.ImageStream {
		return &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Name: name},
			Spec: imageapi.ImageStreamSpec{
				DockerImageRepository: "registry:5000/openshift/" + name,
				Tags: map[string]imageapi.TagReference{
					tag: {
						From: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "registry:5000/openshift/" + name + ":" + tag,
						},
					},
				},
			},
		}

	}
	mockStreamMapping := func(name, tag string) *imageapi.ImageStreamMapping {
		return &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{Name: name},
			Tag:        tag,
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: name,
				},
				DockerImageReference: "registry:5000/openshift/" + name + ":" + tag,
			},
		}

	}
	multipleImageChangeBuildConfig := func() *buildapi.BuildConfig {
		strategy := stiStrategy("ImageStreamTag", "image1:tag1")
		bc := imageChangeBuildConfig("multi-image-trigger", strategy)
		bc.Spec.BuildSpec.Output.To.Name = "image1:outputtag"
		bc.Spec.Triggers = []buildapi.BuildTriggerPolicy{
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					From: &kapi.ObjectReference{
						Name: "image2:tag2",
						Kind: "ImageStreamTag",
					},
				},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					From: &kapi.ObjectReference{
						Name: "image3:tag3",
						Kind: "ImageStreamTag",
					},
				},
			},
		}
		return bc
	}
	clusterAdminClient := setup(t)
	config := multipleImageChangeBuildConfig()
	triggersToTest := []struct {
		triggerIndex int
		name         string
		tag          string
	}{
		{
			triggerIndex: 0,
			name:         "image1",
			tag:          "tag1",
		},
		{
			triggerIndex: 1,
			name:         "image2",
			tag:          "tag2",
		},
		{
			triggerIndex: 2,
			name:         "image3",
			tag:          "tag3",
		},
	}

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

	for _, tc := range triggersToTest {
		imageStream := mockImageStream(tc.name, tc.tag)
		imageStreamMapping := mockStreamMapping(tc.name, tc.tag)
		imageStream, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream)
		if err != nil {
			t.Fatalf("Couldn't create ImageStream: %v", err)
		}

		err = clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(imageStreamMapping)
		if err != nil {
			t.Fatalf("Couldn't create Image: %v", err)
		}
		// wait for initial build event from the creation of the imagerepo
		event := <-watch.ResultChan()
		if e, a := watchapi.Added, event.Type; e != a {
			t.Fatalf("expected watch event type %s, got %s", e, a)
		}
		newBuild := event.Object.(*buildapi.Build)
		trigger := config.Spec.Triggers[tc.triggerIndex]
		if trigger.ImageChange.From == nil {
			switch newBuild.Spec.Strategy.Type {
			case buildapi.SourceBuildStrategyType:
				if newBuild.Spec.Strategy.SourceStrategy.From.Name != "registry:5000/openshift/"+tc.name+":"+tc.tag {
					i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
					bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
					t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %#v", "registry:5000/openshift/"+tc.name+":"+tc.tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[tc.triggerIndex].ImageChange)
				}
			case buildapi.DockerBuildStrategyType:
				if newBuild.Spec.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/"+tc.name+":"+tc.tag {
					i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
					bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
					t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %#v", "registry:5000/openshift/"+tc.name+":"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[tc.triggerIndex].ImageChange)
				}
			case buildapi.CustomBuildStrategyType:
				if newBuild.Spec.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/"+tc.name+":"+tag {
					i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
					bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
					t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %#v", "registry:5000/openshift/"+tc.name+":"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[tc.triggerIndex].ImageChange)
				}

			}
		}
		event = <-watch.ResultChan()
		if e, a := watchapi.Modified, event.Type; e != a {
			t.Fatalf("expected watch event type %s, got %s", e, a)
		}
		newBuild = event.Object.(*buildapi.Build)
		// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
		if newBuild.Spec.Output.To.Name != "image1:outputtag" {
			t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
		}

		// wait for build config to be updated
		<-watch2.ResultChan()
		updatedConfig, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
		if err != nil {
			t.Fatalf("Couldn't get BuildConfig: %v", err)
		}
		// the first tag did not have an image id, so the last trigger field is the pull spec
		if updatedConfig.Spec.Triggers[tc.triggerIndex].ImageChange.LastTriggeredImageID != "registry:5000/openshift/"+tc.name+":"+tc.tag {
			t.Fatalf("Expected imageID equal to pull spec, got %#v", updatedConfig.Spec.Triggers[0].ImageChange)
		}
	}
}

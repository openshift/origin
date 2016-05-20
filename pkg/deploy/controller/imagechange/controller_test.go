package imagechange

import (
	"flag"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	testapi "github.com/openshift/origin/pkg/deploy/api/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	flag.Set("v", "5")
}

// TestHandle_changeForNonAutomaticTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the trigger's
// automatic flag being set to false.
func TestHandle_changeForNonAutomaticTag(t *testing.T) {
	fake := &testclient.Fake{}
	fake.AddReactor("update", "deploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deploymentconfig update")
		return true, nil, nil
	})

	controller := &ImageChangeController{
		listDeploymentConfigs: func() ([]*deployapi.DeploymentConfig, error) {
			config := testapi.OkDeploymentConfig(1)
			config.Namespace = kapi.NamespaceDefault
			config.Spec.Triggers[0].ImageChangeParams.Automatic = false

			return []*deployapi.DeploymentConfig{config}, nil
		},
		client: fake,
	}

	// verify no-op
	tagUpdate := makeStream(testapi.ImageStreamName, imageapi.DefaultImageTag, testapi.DockerImageReference, testapi.ImageID)

	if err := controller.Handle(tagUpdate); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(fake.Actions()) > 0 {
		t.Fatalf("unexpected actions: %v", fake.Actions())
	}
}

// TestHandle_changeForInitialNonAutomaticDeployment ensures that an image update for which
// there is a matching trigger will actually update the deployment config if it hasn't been
// deployed before.
func TestHandle_changeForInitialNonAutomaticDeployment(t *testing.T) {
	fake := &testclient.Fake{}

	controller := &ImageChangeController{
		listDeploymentConfigs: func() ([]*deployapi.DeploymentConfig, error) {
			config := testapi.OkDeploymentConfig(0)
			config.Namespace = kapi.NamespaceDefault
			config.Spec.Triggers[0].ImageChangeParams.Automatic = false

			return []*deployapi.DeploymentConfig{config}, nil
		},
		client: fake,
	}

	// verify no-op
	tagUpdate := makeStream(testapi.ImageStreamName, imageapi.DefaultImageTag, testapi.DockerImageReference, testapi.ImageID)

	if err := controller.Handle(tagUpdate); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	actions := fake.Actions()
	if len(actions) != 1 {
		t.Fatalf("unexpected amount of actions: expected 1, got %d (%v)", len(actions), actions)
	}
	if !actions[0].Matches("update", "deploymentconfigs") {
		t.Fatalf("unexpected action: %v", actions[0])
	}
}

// TestHandle_changeForUnregisteredTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the tag specified on
// the trigger not matching the tags defined on the image stream.
func TestHandle_changeForUnregisteredTag(t *testing.T) {
	fake := &testclient.Fake{}
	fake.AddReactor("update", "deploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deploymentconfig update")
		return true, nil, nil
	})

	controller := &ImageChangeController{
		listDeploymentConfigs: func() ([]*deployapi.DeploymentConfig, error) {
			return []*deployapi.DeploymentConfig{testapi.OkDeploymentConfig(0)}, nil
		},
		client: fake,
	}

	// verify no-op
	tagUpdate := makeStream(testapi.ImageStreamName, "unrecognized", testapi.DockerImageReference, testapi.ImageID)

	if err := controller.Handle(tagUpdate); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(fake.Actions()) > 0 {
		t.Fatalf("unexpected actions: %v", fake.Actions())
	}
}

// TestHandle_matchScenarios comprehensively tests trigger definitions against
// image stream updates to ensure that the image change triggers match (or don't
// match) properly.
func TestHandle_matchScenarios(t *testing.T) {
	tests := []struct {
		param   *deployapi.DeploymentTriggerImageChangeParams
		matches bool
	}{
		// Update from empty last image ID to a new one with explicit namespaces
		{
			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: imageapi.JoinImageStreamTag(testapi.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			matches: true,
		},
		// Update from empty last image ID to a new one with implicit namespaces
		{
			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageapi.JoinImageStreamTag(testapi.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			matches: true,
		},
		// Update from empty last image ID to a new one, but not marked automatic
		{
			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          false,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: imageapi.JoinImageStreamTag(testapi.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			matches: false,
		},
		// Updated image ID is equal to the last triggered ID
		{
			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageapi.JoinImageStreamTag(testapi.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: testapi.DockerImageReference,
			},
			matches: false,
		},
		// Trigger stream reference doesn't match
		{
			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: imageapi.JoinImageStreamTag("other-stream", imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			matches: false,
		},
	}

	for i, test := range tests {
		updated := false

		fake := &testclient.Fake{}
		fake.AddReactor("update", "deploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			if !test.matches {
				t.Fatalf("unexpected deploymentconfig update for scenario %d", i)
			}
			updated = true
			return true, nil, nil
		})

		config := testapi.OkDeploymentConfig(1)
		config.Namespace = kapi.NamespaceDefault
		config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{
			{
				Type:              deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: test.param,
			},
		}

		controller := &ImageChangeController{
			listDeploymentConfigs: func() ([]*deployapi.DeploymentConfig, error) {
				return []*deployapi.DeploymentConfig{config}, nil
			},
			client: fake,
		}

		t.Logf("running scenario: %d", i)
		stream := makeStream(testapi.ImageStreamName, imageapi.DefaultImageTag, testapi.DockerImageReference, testapi.ImageID)
		if err := controller.Handle(stream); err != nil {
			t.Fatalf("unexpected error for scenario %v: %v", i, err)
		}

		// assert updates occurred
		if test.matches && !updated {
			t.Fatalf("expected update for scenario: %v", test)
		}
	}
}

func makeStream(name, tag, dir, image string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: kapi.NamespaceDefault},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				tag: {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: dir,
							Image:                image,
						},
					},
				},
			},
		},
	}
}

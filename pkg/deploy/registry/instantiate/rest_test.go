package instantiate

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	_ "github.com/openshift/origin/pkg/deploy/apis/apps/install"
	deploytest "github.com/openshift/origin/pkg/deploy/apis/apps/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

var codec = kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)

// TestProcess_changeForNonAutomaticTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the trigger's
// automatic flag being set to false or updates the config if forced.
func TestProcess_changeForNonAutomaticTag(t *testing.T) {
	tests := []struct {
		name     string
		force    bool
		excludes []deployapi.DeploymentTriggerType

		expected    bool
		expectedErr bool
	}{
		{
			name:  "normal update",
			force: false,

			expected:    false,
			expectedErr: false,
		},
		{
			name:     "forced update but excluded",
			force:    true,
			excludes: []deployapi.DeploymentTriggerType{deployapi.DeploymentTriggerOnImageChange},

			expected:    false,
			expectedErr: false,
		},
		{
			name:  "forced update",
			force: true,

			expected:    true,
			expectedErr: false,
		},
	}

	for _, test := range tests {
		config := deploytest.OkDeploymentConfig(1)
		config.Namespace = metav1.NamespaceDefault
		config.Spec.Triggers[0].ImageChangeParams.Automatic = false
		// The image has been resolved at least once before.
		config.Spec.Triggers[0].ImageChangeParams.LastTriggeredImage = deploytest.DockerImageReference

		stream := deploytest.OkStreamForConfig(config)
		config.Spec.Triggers[0].ImageChangeParams.LastTriggeredImage = "someotherresolveddockerimagereference"

		fake := &testclient.Fake{}
		fake.AddReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if !test.expected {
				t.Errorf("unexpected imagestream call")
			}
			return true, stream, nil
		})

		image := config.Spec.Template.Spec.Containers[0].Image

		// Force equals to false; we shouldn't update the config anyway
		err := processTriggers(config, fake, test.force, test.excludes)
		if err == nil && test.expectedErr {
			t.Errorf("%s: expected an error", test.name)
			continue
		}
		if err != nil && !test.expectedErr {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if test.expected && config.Spec.Template.Spec.Containers[0].Image == image {
			t.Errorf("%s: expected an image update but got none", test.name)
		} else if !test.expected && config.Spec.Template.Spec.Containers[0].Image != image {
			t.Errorf("%s: didn't expect an image update but got %s", test.name, image)
		}
	}
}

// TestProcess_changeForUnregisteredTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the tag specified on
// the trigger not matching the tags defined on the image stream.
func TestProcess_changeForUnregisteredTag(t *testing.T) {
	config := deploytest.OkDeploymentConfig(0)
	stream := deploytest.OkStreamForConfig(config)
	// The image has been resolved at least once before.
	config.Spec.Triggers[0].ImageChangeParams.From.Name = imageapi.JoinImageStreamTag(stream.Name, "unrelatedtag")

	fake := &testclient.Fake{}
	fake.AddReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, stream, nil
	})

	image := config.Spec.Template.Spec.Containers[0].Image

	// verify no-op; should be the same for force=true and force=false
	if err := processTriggers(config, fake, false, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image != config.Spec.Template.Spec.Containers[0].Image {
		t.Fatalf("unexpected image update: %#v", config.Spec.Template.Spec.Containers[0].Image)
	}

	if err := processTriggers(config, fake, true, nil); err != nil {
		t.Fatalf("unexpected error when forced: %v", err)
	}
	if image != config.Spec.Template.Spec.Containers[0].Image {
		t.Fatalf("unexpected image update when forced: %#v", config.Spec.Template.Spec.Containers[0].Image)
	}
}

// TestProcess_matchScenarios comprehensively tests trigger definitions against
// image stream updates to ensure that the image change triggers match (or don't
// match) properly.
func TestProcess_matchScenarios(t *testing.T) {
	tests := []struct {
		name string

		param              *deployapi.DeploymentTriggerImageChangeParams
		containerImageFunc func() string
		notFound           bool

		expected bool
	}{
		{
			name: "automatic=true, initial trigger, explicit namespace",

			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageapi.JoinImageStreamTag(deploytest.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},

			expected: true,
		},
		{
			name: "automatic=true, initial trigger, implicit namespace",

			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageapi.JoinImageStreamTag(deploytest.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},

			expected: true,
		},
		{
			name: "automatic=false, initial trigger",

			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          false,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageapi.JoinImageStreamTag(deploytest.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},

			expected: false,
		},
		{
			name: "(no-op) automatic=false, already triggered",

			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          false,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageapi.JoinImageStreamTag(deploytest.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: deploytest.DockerImageReference,
			},

			expected: false,
		},
		{
			name: "(no-op) automatic=true, image is already deployed",

			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageapi.JoinImageStreamTag(deploytest.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: deploytest.DockerImageReference,
			},

			expected: false,
		},
		{
			name: "(no-op) trigger doesn't match the stream",

			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageapi.JoinImageStreamTag("other-stream", imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			notFound: true,

			expected: false,
		},
		{
			name: "allow lastTriggeredImage to resolve",

			containerImageFunc: func() string {
				image := "registry:5000/openshift/test-image-stream@sha256:0000000000000000000000000000000000000000000000000000000000000001"
				return image
			},
			param: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageapi.JoinImageStreamTag(deploytest.ImageStreamName, imageapi.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			notFound: false,

			expected: true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Logf("running test %q", test.name)

		fake := &testclient.Fake{}
		fake.AddReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if test.notFound {
				name := action.(clientgotesting.GetAction).GetName()
				return true, nil, errors.NewNotFound(imageapi.Resource("ImageStream"), name)
			}
			stream := fakeStream(deploytest.ImageStreamName, imageapi.DefaultImageTag, deploytest.DockerImageReference, deploytest.ImageID)
			return true, stream, nil
		})

		config := deploytest.OkDeploymentConfig(1)
		config.Namespace = metav1.NamespaceDefault
		config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{
			{
				Type:              deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: test.param,
			},
		}

		if test.containerImageFunc != nil {
			config.Spec.Template.Spec.Containers[0].Image = test.containerImageFunc()
		}
		image := config.Spec.Template.Spec.Containers[0].Image

		err := processTriggers(config, fake, false, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if test.containerImageFunc == nil && test.expected && config.Spec.Template.Spec.Containers[0].Image == image {
			t.Errorf("%s: expected an image update but got none", test.name)
			continue
		}
		if !test.expected && config.Spec.Template.Spec.Containers[0].Image != image {
			t.Errorf("%s: didn't expect an image update but got %s", test.name, image)
			continue
		}
		if test.containerImageFunc != nil && image != config.Spec.Triggers[0].ImageChangeParams.LastTriggeredImage {
			t.Errorf("%s: expected a lastTriggeredImage update to %q, got none", test.name, image)
		}
	}
}

func fakeStream(name, tag, dir, image string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
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

func TestCanTrigger(t *testing.T) {
	tests := []struct {
		name string

		config  *deployapi.DeploymentConfig
		decoded *deployapi.DeploymentConfig
		force   bool

		expected       bool
		expectedCauses []deployapi.DeploymentCause
		expectedErr    bool
	}{
		{
			name: "no trigger [w/ podtemplate change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Triggers: []deployapi.DeploymentTriggerPolicy{},
					Template: deploytest.OkPodTemplateChanged(),
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Triggers: []deployapi.DeploymentTriggerPolicy{},
					Template: deploytest.OkPodTemplate(),
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "forced updated",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: true,

			expected:       true,
			expectedCauses: []deployapi.DeploymentCause{{Type: deployapi.DeploymentTriggerManual}},
		},
		{
			name: "config change trigger only [w/ podtemplate change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       true,
			expectedCauses: deploytest.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change trigger only [no change][initial]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       true,
			expectedCauses: deploytest.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change trigger only [no change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=false][w/ podtemplate change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(), // Irrelevant change
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkNonAutomaticICT(), // Image still to be resolved but it's false anyway
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkNonAutomaticICT(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
			expectedErr:    true,
		},
		{
			name: "image change trigger only [automatic=false][w/ image change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(), // Image has been updated in the template but automatic=false
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkTriggeredNonAutomatic(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkNonAutomaticICT(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=true][w/ image change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkTriggeredImageChange(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkImageChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       true,
			expectedCauses: deploytest.OkImageChangeDetails().Causes,
		},
		{
			name: "image change trigger only [automatic=true][no change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkTriggeredImageChange(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkTriggeredImageChange(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "config change and image change trigger [automatic=false][initial][w/ image change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkTriggeredNonAutomatic(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkNonAutomaticICT(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       true,
			expectedCauses: deploytest.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change and image change trigger [automatic=false][initial][no change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkNonAutomaticICT(), // Image is not resolved yet
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkNonAutomaticICT(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
			expectedErr:    true,
		},
		{
			name: "config change and image change trigger [automatic=true][initial][w/ podtemplate change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(), // Pod template has changed but the image in the template is yet to be updated
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkImageChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkImageChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
			expectedErr:    true,
		},
		{
			name: "config change and image change trigger [automatic=true][initial][w/ image change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkTriggeredImageChange(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkImageChangeTrigger(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       true,
			expectedCauses: deploytest.OkImageChangeDetails().Causes,
		},
		{
			name: "config change and image change trigger [automatic=true][no change]",

			config: &deployapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: deployapi.DeploymentConfigSpec{
					Template: deploytest.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						deploytest.OkConfigChangeTrigger(),
						deploytest.OkTriggeredImageChange(),
					},
				},
				Status: deploytest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
	}

	for _, test := range tests {
		t.Logf("running scenario %q", test.name)

		client := &fake.Clientset{}
		client.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			config := test.decoded
			if config == nil {
				config = test.config
			}
			config = deploytest.RoundTripConfig(t, config)
			deployment, _ := deployutil.MakeDeployment(config, codec)
			return true, deployment, nil
		})

		test.config = deploytest.RoundTripConfig(t, test.config)

		got, gotCauses, err := canTrigger(test.config, client.Core(), codec, test.force)
		if err != nil && !test.expectedErr {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if err == nil && test.expectedErr {
			t.Errorf("expected an error")
			continue
		}
		if test.expected != got {
			t.Errorf("expected to trigger: %t, got: %t", test.expected, got)
		}
		if !kapihelper.Semantic.DeepEqual(test.expectedCauses, gotCauses) {
			t.Errorf("expected causes:\n%#v\ngot:\n%#v", test.expectedCauses, gotCauses)
		}
	}
}

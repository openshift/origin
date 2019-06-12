package instantiate

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1fakeclient "github.com/openshift/client-go/image/clientset/versioned/fake"
	"github.com/openshift/library-go/pkg/apps/appsutil"
	"github.com/openshift/library-go/pkg/image/imageutil"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/internaltest"
	appsv1conversions "github.com/openshift/origin/pkg/apps/apis/apps/v1"
)

// TestProcess_changeForNonAutomaticTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the trigger's
// automatic flag being set to false or updates the config if forced.
func TestProcess_changeForNonAutomaticTag(t *testing.T) {
	tests := []struct {
		name     string
		force    bool
		excludes []appsapi.DeploymentTriggerType

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
			excludes: []appsapi.DeploymentTriggerType{appsapi.DeploymentTriggerOnImageChange},

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
		config := appstest.OkDeploymentConfig(1)
		config.Namespace = metav1.NamespaceDefault
		config.Spec.Triggers[0].ImageChangeParams.Automatic = false
		// The image has been resolved at least once before.
		config.Spec.Triggers[0].ImageChangeParams.LastTriggeredImage = appstest.DockerImageReference

		stream := OkStreamForConfig(config)
		config.Spec.Triggers[0].ImageChangeParams.LastTriggeredImage = "someotherresolveddockerimagereference"

		fake := &imagev1fakeclient.Clientset{}
		fake.AddReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if !test.expected {
				t.Errorf("unexpected imagestream call")
			}
			return true, stream, nil
		})

		image := config.Spec.Template.Spec.Containers[0].Image

		// Force equals to false; we shouldn't update the config anyway
		err := processTriggers(config, fake.ImageV1(), test.force, test.excludes)
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
	config := appstest.OkDeploymentConfig(0)
	stream := OkStreamForConfig(config)
	// The image has been resolved at least once before.
	config.Spec.Triggers[0].ImageChangeParams.From.Name = imageutil.JoinImageStreamTag(stream.Name, "unrelatedtag")

	fake := &imagev1fakeclient.Clientset{}
	fake.AddReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, stream, nil
	})

	image := config.Spec.Template.Spec.Containers[0].Image

	// verify no-op; should be the same for force=true and force=false
	if err := processTriggers(config, fake.ImageV1(), false, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image != config.Spec.Template.Spec.Containers[0].Image {
		t.Fatalf("unexpected image update: %#v", config.Spec.Template.Spec.Containers[0].Image)
	}

	if err := processTriggers(config, fake.ImageV1(), true, nil); err != nil {
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

		param              *appsapi.DeploymentTriggerImageChangeParams
		containerImageFunc func() string
		notFound           bool

		expected bool
	}{
		{
			name: "automatic=true, initial trigger, explicit namespace",

			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageutil.JoinImageStreamTag(appstest.ImageStreamName, imagev1.DefaultImageTag)},
				LastTriggeredImage: "",
			},

			expected: true,
		},
		{
			name: "automatic=true, initial trigger, implicit namespace",

			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageutil.JoinImageStreamTag(appstest.ImageStreamName, imagev1.DefaultImageTag)},
				LastTriggeredImage: "",
			},

			expected: true,
		},
		{
			name: "automatic=false, initial trigger",

			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          false,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageutil.JoinImageStreamTag(appstest.ImageStreamName, imagev1.DefaultImageTag)},
				LastTriggeredImage: "",
			},

			expected: false,
		},
		{
			name: "(no-op) automatic=false, already triggered",

			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          false,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageutil.JoinImageStreamTag(appstest.ImageStreamName, imagev1.DefaultImageTag)},
				LastTriggeredImage: appstest.DockerImageReference,
			},

			expected: false,
		},
		{
			name: "(no-op) automatic=true, image is already deployed",

			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageutil.JoinImageStreamTag(appstest.ImageStreamName, imagev1.DefaultImageTag)},
				LastTriggeredImage: appstest.DockerImageReference,
			},

			expected: false,
		},
		{
			name: "(no-op) trigger doesn't match the stream",

			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Namespace: metav1.NamespaceDefault, Name: imageutil.JoinImageStreamTag("other-stream", imagev1.DefaultImageTag)},
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
			param: &appsapi.DeploymentTriggerImageChangeParams{
				Automatic:          true,
				ContainerNames:     []string{"container1"},
				From:               kapi.ObjectReference{Name: imageutil.JoinImageStreamTag(appstest.ImageStreamName, imagev1.DefaultImageTag)},
				LastTriggeredImage: "",
			},
			notFound: false,

			expected: true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Logf("running test %q", test.name)

		fake := &imagev1fakeclient.Clientset{}
		fake.AddReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if test.notFound {
				name := action.(clientgotesting.GetAction).GetName()
				return true, nil, errors.NewNotFound(image.Resource("ImageStream"), name)
			}
			stream := fakeStream(appstest.ImageStreamName, imagev1.DefaultImageTag, appstest.DockerImageReference, appstest.ImageID)
			return true, stream, nil
		})

		config := appstest.OkDeploymentConfig(1)
		config.Namespace = metav1.NamespaceDefault
		config.Spec.Triggers = []appsapi.DeploymentTriggerPolicy{
			{
				Type:              appsapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: test.param,
			},
		}

		if test.containerImageFunc != nil {
			config.Spec.Template.Spec.Containers[0].Image = test.containerImageFunc()
		}
		image := config.Spec.Template.Spec.Containers[0].Image

		err := processTriggers(config, fake.ImageV1(), false, nil)
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

func fakeStream(name, tag, dir, image string) *imagev1.ImageStream {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{Tag: tag, Items: []imagev1.TagEvent{{DockerImageReference: dir, Image: image}}},
			},
		},
	}
}

func TestCanTrigger(t *testing.T) {
	tests := []struct {
		name string

		config  *appsapi.DeploymentConfig
		decoded *appsapi.DeploymentConfig
		force   bool

		expected       bool
		expectedCauses []appsapi.DeploymentCause
		expectedErr    bool
	}{
		{
			name: "no trigger [w/ podtemplate change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Triggers: []appsapi.DeploymentTriggerPolicy{},
					Template: appstest.OkPodTemplateChanged(),
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Triggers: []appsapi.DeploymentTriggerPolicy{},
					Template: appstest.OkPodTemplate(),
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "forced updated",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: true,

			expected:       true,
			expectedCauses: []appsapi.DeploymentCause{{Type: appsapi.DeploymentTriggerManual}},
		},
		{
			name: "config change trigger only [w/ podtemplate change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       true,
			expectedCauses: appstest.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change trigger only [no change][initial]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       true,
			expectedCauses: appstest.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change trigger only [no change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=false][w/ podtemplate change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(), // Irrelevant change
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkNonAutomaticICT(), // Image still to be resolved but it's false anyway
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkNonAutomaticICT(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
			expectedErr:    true,
		},
		{
			name: "image change trigger only [automatic=false][w/ image change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(), // Image has been updated in the template but automatic=false
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkTriggeredNonAutomatic(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkNonAutomaticICT(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=true][w/ image change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkTriggeredImageChange(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkImageChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       true,
			expectedCauses: appstest.OkImageChangeDetails().Causes,
		},
		{
			name: "image change trigger only [automatic=true][no change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkTriggeredImageChange(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						OkTriggeredImageChange(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "config change and image change trigger [automatic=false][initial][w/ image change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						OkTriggeredNonAutomatic(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						OkNonAutomaticICT(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       true,
			expectedCauses: appstest.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change and image change trigger [automatic=false][initial][no change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						OkNonAutomaticICT(), // Image is not resolved yet
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						OkNonAutomaticICT(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
			expectedErr:    true,
		},
		{
			name: "config change and image change trigger [automatic=true][initial][w/ podtemplate change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(), // Pod template has changed but the image in the template is yet to be updated
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						appstest.OkImageChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						appstest.OkImageChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       false,
			expectedCauses: nil,
			expectedErr:    true,
		},
		{
			name: "config change and image change trigger [automatic=true][initial][w/ image change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						OkTriggeredImageChange(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			decoded: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplate(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						appstest.OkImageChangeTrigger(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(0),
			},
			force: false,

			expected:       true,
			expectedCauses: appstest.OkImageChangeDetails().Causes,
		},
		{
			name: "config change and image change trigger [automatic=true][no change]",

			config: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: appsapi.DeploymentConfigSpec{
					Template: appstest.OkPodTemplateChanged(),
					Triggers: []appsapi.DeploymentTriggerPolicy{
						appstest.OkConfigChangeTrigger(),
						OkTriggeredImageChange(),
					},
				},
				Status: appstest.OkDeploymentConfigStatus(1),
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
			config = RoundTripConfig(t, config)
			deployment, _ := makeDeployment(config)
			return true, deployment, nil
		})

		test.config = RoundTripConfig(t, test.config)

		got, gotCauses, err := canTrigger(test.config, client.CoreV1(), test.force)
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

func makeDeployment(config *appsapi.DeploymentConfig) (*corev1.ReplicationController, error) {
	configExternal := &appsv1.DeploymentConfig{}
	if err := legacyscheme.Scheme.Convert(config, configExternal, nil); err != nil {
		return nil, err
	}
	return appsutil.MakeDeployment(configExternal)
}

func OkStreamForConfig(config *appsapi.DeploymentConfig) *imagev1.ImageStream {
	for _, t := range config.Spec.Triggers {
		if t.Type != appsapi.DeploymentTriggerOnImageChange {
			continue
		}

		ref := t.ImageChangeParams.From
		name, tag, _ := imageutil.SplitImageStreamTag(ref.Name)

		return &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ref.Namespace,
			},
			Status: imagev1.ImageStreamStatus{
				Tags: []imagev1.NamedTagEventList{
					{Tag: tag, Items: []imagev1.TagEvent{{DockerImageReference: t.ImageChangeParams.LastTriggeredImage}}},
				},
			},
		}
	}
	return nil
}

func OkNonAutomaticICT() appsapi.DeploymentTriggerPolicy {
	ict := appstest.OkImageChangeTrigger()
	ict.ImageChangeParams.Automatic = false
	return ict
}

func OkTriggeredNonAutomatic() appsapi.DeploymentTriggerPolicy {
	ict := OkNonAutomaticICT()
	ict.ImageChangeParams.LastTriggeredImage = appstest.DockerImageReference
	return ict
}

func OkTriggeredImageChange() appsapi.DeploymentTriggerPolicy {
	ict := appstest.OkImageChangeTrigger()
	ict.ImageChangeParams.LastTriggeredImage = appstest.DockerImageReference
	return ict
}

func RoundTripConfig(t *testing.T, config *appsapi.DeploymentConfig) *appsapi.DeploymentConfig {
	scheme := runtime.NewScheme()
	appsv1conversions.Install(scheme)

	versioned, err := scheme.ConvertToVersion(config, appsv1.GroupVersion)
	if err != nil {
		t.Errorf("unexpected conversion error: %v", err)
		return nil
	}
	defaulted, err := scheme.ConvertToVersion(versioned, appsapi.SchemeGroupVersion)
	if err != nil {
		t.Errorf("unexpected conversion error: %v", err)
		return nil
	}
	return defaulted.(*appsapi.DeploymentConfig)
}

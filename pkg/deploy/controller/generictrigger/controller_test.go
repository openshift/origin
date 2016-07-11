package generictrigger

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	testapi "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

var (
	codec      = kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)
	mock       = &testclient.Fake{}
	dcInformer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return mock.DeploymentConfigs(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return mock.DeploymentConfigs(kapi.NamespaceAll).Watch(options)
			},
		},
		&deployapi.DeploymentConfig{},
		2*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	streamInformer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return mock.ImageStreams(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return mock.ImageStreams(kapi.NamespaceAll).Watch(options)
			},
		},
		&imageapi.ImageStream{},
		2*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
)

// TestHandle_newConfigNoTriggers ensures that a change to a config with no
// triggers doesn't result in a new config version bump.
func TestHandle_newConfigNoTriggers(t *testing.T) {
	fake := &testclient.Fake{}
	kFake := &ktestclient.Fake{}

	controller := NewDeploymentTriggerController(dcInformer, streamInformer, fake, kFake, codec)

	config := testapi.OkDeploymentConfig(1)
	config.Namespace = kapi.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Actions()) > 0 {
		t.Fatalf("unexpected actions by the Origin client: %v", fake.Actions())
	}
	if len(kFake.Actions()) > 0 {
		t.Fatalf("unexpected actions by the Kube client: %v", kFake.Actions())
	}
}

// TestHandle_newConfigTriggers ensures that the creation of a new config
// (with version 0) with a config change trigger results in a version bump and
// cause update for initial deployment.
func TestHandle_newConfigTriggers(t *testing.T) {
	var updated *deployapi.DeploymentConfig

	fake := &testclient.Fake{}
	fake.AddReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		updated = action.(ktestclient.UpdateAction).GetObject().(*deployapi.DeploymentConfig)
		return true, updated, nil
	})
	kFake := &ktestclient.Fake{}

	controller := NewDeploymentTriggerController(dcInformer, streamInformer, fake, kFake, codec)

	config := testapi.OkDeploymentConfig(0)
	config.Namespace = kapi.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkConfigChangeTrigger()}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected config to be updated")
	}
	if e, a := int64(1), updated.Status.LatestVersion; e != a {
		t.Fatalf("expected update to latestversion=%d, got %d", e, a)
	}
	if updated.Status.Details == nil {
		t.Fatalf("expected config change details to be set")
	} else if updated.Status.Details.Causes == nil {
		t.Fatalf("expected config change causes to be set")
	} else if updated.Status.Details.Causes[0].Type != deployapi.DeploymentTriggerOnConfigChange {
		t.Fatalf("expected config change cause to be set to config change trigger, got %s", updated.Status.Details.Causes[0].Type)
	}
}

// TestHandle_changeWithTemplateDiff ensures that a pod template change to a
// config with a config change trigger results in a version bump and cause
// update.
func TestHandle_changeWithTemplateDiff(t *testing.T) {
	scenarios := []struct {
		name           string
		modify         func(*deployapi.DeploymentConfig)
		changeExpected bool
	}{
		{
			name:           "container name change",
			changeExpected: true,
			modify: func(config *deployapi.DeploymentConfig) {
				config.Spec.Template.Spec.Containers[1].Name = "modified"
			},
		},
		{
			name:           "template label change",
			changeExpected: true,
			modify: func(config *deployapi.DeploymentConfig) {
				config.Spec.Template.Labels["newkey"] = "value"
			},
		},
		{
			name:           "no diff",
			changeExpected: false,
			modify:         func(config *deployapi.DeploymentConfig) {},
		},
	}

	for _, s := range scenarios {
		t.Logf("running scenario: %s", s.name)
		fake := &testclient.Fake{}
		kFake := &ktestclient.Fake{}

		config := testapi.OkDeploymentConfig(1)
		config.Namespace = kapi.NamespaceDefault
		config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkConfigChangeTrigger()}

		versioned, err := kapi.Scheme.ConvertToVersion(config, deployv1.SchemeGroupVersion)
		if err != nil {
			t.Errorf("unexpected conversion error: %v", err)
			continue
		}
		defaulted, err := kapi.Scheme.ConvertToVersion(versioned, deployapi.SchemeGroupVersion)
		if err != nil {
			t.Errorf("unexpected conversion error: %v", err)
			continue
		}
		config = defaulted.(*deployapi.DeploymentConfig)

		deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
		var updated *deployapi.DeploymentConfig

		fake.PrependReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			updated = action.(ktestclient.UpdateAction).GetObject().(*deployapi.DeploymentConfig)
			return true, updated, nil
		})
		kFake.PrependReactor("get", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, deployment, nil
		})

		controller := NewDeploymentTriggerController(dcInformer, streamInformer, fake, kFake, codec)

		s.modify(config)
		if err := controller.Handle(config); err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}

		if !s.changeExpected {
			if updated != nil {
				t.Errorf("unexpected update to version %d: %s", updated.Status.LatestVersion, diff.ObjectReflectDiff(config, updated))
			}
			continue
		}

		// changeExpected == true
		if updated == nil {
			t.Errorf("expected config to be updated")
			continue
		}
		if e, a := int64(2), updated.Status.LatestVersion; e != a {
			t.Errorf("expected update to latestversion=%d, got %d", e, a)
			continue
		}

		if updated.Status.Details == nil {
			t.Errorf("expected config change details to be set")
		} else if updated.Status.Details.Causes == nil {
			t.Errorf("expected config change causes to be set")
		} else if updated.Status.Details.Causes[0].Type != deployapi.DeploymentTriggerOnConfigChange {
			t.Errorf("expected config change cause to be set to config change trigger, got %s", updated.Status.Details.Causes[0].Type)
		}
	}
}

// TestHandle_waitForImageController tests an initial deployment with unresolved image. The config
// change controller should never increment latestVersion, thus trigger a deployment for this config.
func TestHandle_waitForImageController(t *testing.T) {
	fake := &testclient.Fake{}
	kFake := &ktestclient.Fake{}

	fake.PrependReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("an update should never run before the template image is resolved")
		return true, nil, nil
	})

	controller := NewDeploymentTriggerController(dcInformer, streamInformer, fake, kFake, codec)

	config := testapi.OkDeploymentConfig(0)
	config.Namespace = kapi.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkConfigChangeTrigger(), testapi.OkImageChangeTrigger()}

	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_automaticImageUpdates tests automatic and non-automatic updates
// from image change triggers.
func TestHandle_automaticImageUpdates(t *testing.T) {
	tests := []struct {
		name           string
		auto           bool
		canTrigger     bool
		version        int64
		expectedUpdate bool
	}{
		{
			name:           "initial deployment with unresolved image (auto: true)",
			auto:           true,
			canTrigger:     false,
			version:        0,
			expectedUpdate: false,
		},
		{
			name:           "initial deployment with unresolved image (auto: false)",
			auto:           false,
			canTrigger:     false,
			version:        0,
			expectedUpdate: false,
		},
		{
			name:           "initial deployment with resolved image (auto: true)",
			auto:           true,
			canTrigger:     true,
			version:        0,
			expectedUpdate: true,
		},
		{
			name:           "initial deployment with resolved image (auto: false)",
			auto:           false,
			canTrigger:     true,
			version:        0,
			expectedUpdate: true,
		},
	}

	for _, test := range tests {
		updated := false

		fake := &testclient.Fake{}
		kFake := &ktestclient.Fake{}
		fake.PrependReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			updated = true
			return true, nil, nil
		})
		kFake.PrependReactor("get", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			// This will always return no template difference. We test template differences in TestHandle_changeWithTemplateDiff
			config := testapi.OkDeploymentConfig(0)
			deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
			return true, deployment, nil
		})

		controller := NewDeploymentTriggerController(dcInformer, streamInformer, fake, kFake, codec)

		config := testapi.OkDeploymentConfig(test.version)
		config.Namespace = kapi.NamespaceDefault
		ict := testapi.OkImageChangeTrigger()
		ict.ImageChangeParams.Automatic = test.auto
		if test.canTrigger {
			ict.ImageChangeParams.LastTriggeredImage = testapi.DockerImageReference
		}
		config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkConfigChangeTrigger(), ict}

		if err := controller.Handle(config); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if test.expectedUpdate != updated {
			t.Errorf("%s: expected update: %t, got update: %t", test.name, test.expectedUpdate, updated)
		}
	}
}

func TestCanTrigger(t *testing.T) {
	tests := []struct {
		name string

		config  *deployapi.DeploymentConfig
		decoded *deployapi.DeploymentConfig

		expected       bool
		expectedCauses []deployapi.DeploymentCause
	}{
		{
			name: "nil decoded config",

			config:  testapi.OkDeploymentConfig(1),
			decoded: nil,

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "no trigger",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(),
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "config change trigger only",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},

			expected:       true,
			expectedCauses: testapi.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change trigger only [no change][initial]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},

			expected:       true,
			expectedCauses: testapi.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change trigger only [no change]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=false]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(), // Irrelevant change
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkNonAutomaticICT(), // Image still to be resolved but it's false anyway
					},
				},
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkNonAutomaticICT(),
					},
				},
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=false][image triggered]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(), // Image has been updated in the template but automatic=false
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkTriggeredNonAutomatic(),
					},
				},
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkNonAutomaticICT(),
					},
				},
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "image change trigger only [automatic=true]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkTriggeredImageChange(),
					},
				},
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkImageChangeTrigger(),
					},
				},
			},

			expected:       true,
			expectedCauses: testapi.OkImageChangeDetails().Causes,
		},
		{
			name: "image change trigger only [automatic=true][no change]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkImageChangeTrigger(),
					},
				},
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkImageChangeTrigger(),
					},
				},
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "config change and image change trigger [automatic=false][initial][image resolved]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkTriggeredNonAutomatic(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkNonAutomaticICT(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},

			expected:       true,
			expectedCauses: testapi.OkConfigChangeDetails().Causes,
		},
		{
			name: "config change and image change trigger [automatic=false][initial]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkNonAutomaticICT(), // Image is not resolved yet
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkNonAutomaticICT(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "config change and image change trigger [automatic=true][initial]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(), // Pod template has changed but the image in the template is yet to be updated
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkImageChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkImageChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},

			expected:       false,
			expectedCauses: nil,
		},
		{
			name: "config change and image change trigger [automatic=true][initial][image triggered]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplateChanged(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkTriggeredImageChange(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkImageChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(0),
			},

			expected:       true,
			expectedCauses: testapi.OkImageChangeDetails().Causes,
		},
		{
			name: "config change and image change trigger [automatic=true][no change]",

			config: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkImageChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},
			decoded: &deployapi.DeploymentConfig{
				Spec: deployapi.DeploymentConfigSpec{
					Template: testapi.OkPodTemplate(),
					Triggers: []deployapi.DeploymentTriggerPolicy{
						testapi.OkConfigChangeTrigger(),
						testapi.OkImageChangeTrigger(),
					},
				},
				Status: testapi.OkDeploymentConfigStatus(1),
			},

			expected:       false,
			expectedCauses: nil,
		},
	}

	for _, test := range tests {
		got, gotCauses := canTrigger(test.config, test.decoded)
		if test.expected != got {
			t.Errorf("%s: expected to trigger: %t, got: %t", test.name, test.expected, got)
			continue
		}
		if !kapi.Semantic.DeepEqual(test.expectedCauses, gotCauses) {
			t.Errorf("%s: expected causes:\n%#v\ngot:\n%#v", test.name, test.expectedCauses, gotCauses)
		}
	}
}

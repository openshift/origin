package configchange

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_newConfigNoTriggers ensures that a change to a config with no
// triggers doesn't result in a new config version bump.
func TestHandle_newConfigNoTriggers(t *testing.T) {
	fake := &testclient.Fake{}
	kFake := &ktestclient.Fake{}
	controller := &DeploymentConfigChangeController{
		client:  fake,
		kClient: kFake,
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
		},
	}

	config := deployapitest.OkDeploymentConfig(1)
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
	fake.AddReactor("get", "generatedeploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, deployapitest.OkDeploymentConfig(1), nil
	})
	fake.AddReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		updated = action.(ktestclient.UpdateAction).GetObject().(*deployapi.DeploymentConfig)
		return true, updated, nil
	})
	kFake := &ktestclient.Fake{}

	controller := &DeploymentConfigChangeController{
		client:  fake,
		kClient: kFake,
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
		},
	}

	config := deployapitest.OkDeploymentConfig(0)
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected config to be updated")
	}
	if e, a := 1, updated.Status.LatestVersion; e != a {
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

		config := deployapitest.OkDeploymentConfig(1)
		config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}
		deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
		var updated *deployapi.DeploymentConfig

		fake.PrependReactor("get", "generatedeploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, config, nil
		})
		fake.PrependReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			updated = action.(ktestclient.UpdateAction).GetObject().(*deployapi.DeploymentConfig)
			return true, updated, nil
		})
		kFake.PrependReactor("get", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, deployment, nil
		})

		controller := &DeploymentConfigChangeController{
			client:  fake,
			kClient: kFake,
			decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
				return deployutil.DecodeDeploymentConfig(deployment, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
			},
		}

		s.modify(config)
		if err := controller.Handle(config); err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}

		if !s.changeExpected {
			if updated != nil {
				t.Errorf("unexpected update to version %d", updated.Status.LatestVersion)
			}
			continue
		}

		// changeExpected == true
		if updated == nil {
			t.Errorf("expected config to be updated")
			continue
		}
		if e, a := 2, updated.Status.LatestVersion; e != a {
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

func TestHandle_raceWithTheImageController(t *testing.T) {
	var updated *deployapi.DeploymentConfig
	fake := &testclient.Fake{}
	kFake := &ktestclient.Fake{}

	fake.PrependReactor("get", "generatedeploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		generated := deployapitest.OkDeploymentConfig(1)
		generated.Status.Details = deployapitest.OkImageChangeDetails()
		updated = generated
		return true, generated, nil
	})
	fake.PrependReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Errorf("an update should never run in the presence of races")
		updated.Status.Details = deployapitest.OkConfigChangeDetails()
		return true, updated, nil
	})

	controller := &DeploymentConfigChangeController{
		client:  fake,
		kClient: kFake,
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
		},
	}

	config := deployapitest.OkDeploymentConfig(0)
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger(), deployapitest.OkImageChangeTrigger()}

	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := 1, updated.Status.LatestVersion; e != a {
		t.Fatalf("expected update to latestversion=%d, got %d", e, a)
	}

	if updated.Status.Details == nil {
		t.Fatalf("expected config change details to be set")
	} else if updated.Status.Details.Causes == nil {
		t.Fatalf("expected config change causes to be set")
	} else if updated.Status.Details.Causes[0].Type != deployapi.DeploymentTriggerOnImageChange {
		t.Fatalf("expected config change cause to be set to image change trigger, got %s", updated.Status.Details.Causes[0].Type)
	}
}

func TestHandle_nonAutomaticImageUpdates(t *testing.T) {
	var updated *deployapi.DeploymentConfig
	fake := &testclient.Fake{}
	kFake := &ktestclient.Fake{}

	fake.PrependReactor("get", "generatedeploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		generated := deployapitest.OkDeploymentConfig(1)
		// The generator doesn't change automatic so it's ok to fake it here.
		generated.Spec.Triggers[0].ImageChangeParams.Automatic = false
		generated.Status.Details = deployapitest.OkImageChangeDetails()
		updated = generated
		return true, generated, nil
	})
	fake.PrependReactor("update", "deploymentconfigs/status", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		updated.Status.Details = deployapitest.OkConfigChangeDetails()
		return true, updated, nil
	})

	controller := &DeploymentConfigChangeController{
		client:  fake,
		kClient: kFake,
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
		},
	}

	config := deployapitest.OkDeploymentConfig(0)
	ict := deployapitest.OkImageChangeTrigger()
	ict.ImageChangeParams.Automatic = false
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger(), ict}

	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := 1, updated.Status.LatestVersion; e != a {
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

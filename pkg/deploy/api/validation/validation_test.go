package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/test"
)

// Convenience methods

func manualTrigger() []api.DeploymentTriggerPolicy {
	return []api.DeploymentTriggerPolicy{
		{
			Type: api.DeploymentTriggerManual,
		},
	}
}

// TODO: test validation errors for ReplicationControllerTemplates

func TestValidateDeploymentConfigOK(t *testing.T) {
	errs := ValidateDeploymentConfig(&api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Triggers:   manualTrigger(),
		Template:   test.OkDeploymentTemplate(),
	})

	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentConfigMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.DeploymentConfig
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: "bar"},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"metadata.name",
		},
		"missing namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: ""},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"metadata.namespace",
		},
		"invalid name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "-foo", Namespace: "bar"},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"metadata.name",
		},
		"invalid namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "-bar"},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"metadata.namespace",
		},

		"missing trigger.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"triggers[0].type",
		},
		"missing Trigger imageChangeParams.from": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"triggers[0].imageChangeParams.from",
		},
		"invalid Trigger imageChangeParams.from.kind": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							From: kapi.ObjectReference{
								Kind: "Invalid",
								Name: "name",
							},
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"triggers[0].imageChangeParams.from.kind",
		},
		"both fields illegal Trigger imageChangeParams.repositoryName": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
							RepositoryName: "name",
							From: kapi.ObjectReference{
								Name: "other",
							},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"triggers[0].imageChangeParams.repositoryName",
		},
		"missing Trigger imageChangeParams.containerNames": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							RepositoryName: "foo",
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"triggers[0].imageChangeParams.containerNames",
		},
		"missing strategy.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						CustomParams: test.OkCustomParams(),
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.type",
		},
		"missing strategy.customParams": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeCustom,
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.customParams",
		},
		"missing template.strategy.customParams.image": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type:         api.DeploymentStrategyTypeCustom,
						CustomParams: &api.CustomDeploymentStrategyParams{},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.customParams.image",
		},
		"missing template.strategy.recreateParams.pre.failurePolicy": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								ExecNewPod: &api.ExecNewPodHook{
									Command:       []string{"cmd"},
									ContainerName: "container",
								},
							},
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.recreateParams.pre.failurePolicy",
		},
		"missing template.strategy.recreateParams.pre.execNewPod": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
							},
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.recreateParams.pre.execNewPod",
		},
		"missing template.strategy.recreateParams.pre.execNewPod.command": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod: &api.ExecNewPodHook{
									ContainerName: "container",
								},
							},
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.recreateParams.pre.execNewPod.command",
		},
		"missing template.strategy.recreateParams.pre.execNewPod.containerName": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod: &api.ExecNewPodHook{
									Command: []string{"cmd"},
								},
							},
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.recreateParams.pre.execNewPod.containerName",
		},
		"invalid template.strategy.rollingParams.intervalSeconds": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRolling,
						RollingParams: &api.RollingDeploymentStrategyParams{
							IntervalSeconds:     mkintp(-20),
							UpdatePeriodSeconds: mkintp(1),
							TimeoutSeconds:      mkintp(1),
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"template.strategy.rollingParams.intervalSeconds",
		},
		"invalid template.strategy.rollingParams.updatePeriodSeconds": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRolling,
						RollingParams: &api.RollingDeploymentStrategyParams{
							IntervalSeconds:     mkintp(1),
							UpdatePeriodSeconds: mkintp(-20),
							TimeoutSeconds:      mkintp(1),
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"template.strategy.rollingParams.updatePeriodSeconds",
		},
		"invalid template.strategy.rollingParams.timeoutSeconds": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRolling,
						RollingParams: &api.RollingDeploymentStrategyParams{
							IntervalSeconds:     mkintp(1),
							UpdatePeriodSeconds: mkintp(1),
							TimeoutSeconds:      mkintp(-20),
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"template.strategy.rollingParams.timeoutSeconds",
		},
		"missing template.strategy.rollingParams.pre.failurePolicy": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRolling,
						RollingParams: &api.RollingDeploymentStrategyParams{
							IntervalSeconds:     mkintp(1),
							UpdatePeriodSeconds: mkintp(1),
							TimeoutSeconds:      mkintp(20),
							Pre: &api.LifecycleHook{
								ExecNewPod: &api.ExecNewPodHook{
									Command:       []string{"cmd"},
									ContainerName: "container",
								},
							},
						},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.rollingParams.pre.failurePolicy",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeploymentConfig(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateDeploymentConfigUpdate(t *testing.T) {
	oldConfig := &api.DeploymentConfig{
		ObjectMeta:    kapi.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"},
		Triggers:      manualTrigger(),
		Template:      test.OkDeploymentTemplate(),
		LatestVersion: 5,
	}
	newConfig := &api.DeploymentConfig{
		ObjectMeta:    kapi.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"},
		Triggers:      manualTrigger(),
		Template:      test.OkDeploymentTemplate(),
		LatestVersion: 3,
	}

	scenarios := []struct {
		oldLatestVersion int
		newLatestVersion int
	}{
		{5, 3},
		{5, 7},
		{0, -1},
	}

	for _, values := range scenarios {
		oldConfig.LatestVersion = values.oldLatestVersion
		newConfig.LatestVersion = values.newLatestVersion
		errs := ValidateDeploymentConfigUpdate(newConfig, oldConfig)
		if len(errs) == 0 {
			t.Errorf("Expected update failure")
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != fielderrors.ValidationErrorTypeInvalid {
				t.Errorf("expected update error to have type %s: %v", fielderrors.ValidationErrorTypeInvalid, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != "latestVersion" {
				t.Errorf("expected update error to have field %s: %v", "latestVersion", errs[i])
			}
		}
	}

	// testing for a successful update
	oldConfig.LatestVersion = 5
	newConfig.LatestVersion = 6
	errs := ValidateDeploymentConfigUpdate(newConfig, oldConfig)
	if len(errs) > 0 {
		t.Errorf("Unexpected update failure")
	}
}

func TestValidateDeploymentConfigRollbackOK(t *testing.T) {
	rollback := &api.DeploymentConfigRollback{
		Spec: api.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name: "deployment",
			},
		},
	}

	errs := ValidateDeploymentConfigRollback(rollback)
	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %v", errs)
	}

	if e, a := "ReplicationController", rollback.Spec.From.Kind; e != a {
		t.Errorf("expected kind %s, got %s", e, a)
	}
}

func TestValidateDeploymentConfigRollbackInvalidFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.DeploymentConfigRollback
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing spec.from.name": {
			api.DeploymentConfigRollback{
				Spec: api.DeploymentConfigRollbackSpec{
					From: kapi.ObjectReference{},
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.from.name",
		},
		"wrong spec.from.kind": {
			api.DeploymentConfigRollback{
				Spec: api.DeploymentConfigRollbackSpec{
					From: kapi.ObjectReference{
						Kind: "unknown",
						Name: "deployment",
					},
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"spec.from.kind",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeploymentConfigRollback(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateDeploymentConfigDefaultImageStreamKind(t *testing.T) {
	config := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Triggers: []api.DeploymentTriggerPolicy{
			{
				Type: api.DeploymentTriggerOnImageChange,
				ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
					From: kapi.ObjectReference{
						Name: "name",
					},
					ContainerNames: []string{"foo"},
				},
			},
		},
		Template: test.OkDeploymentTemplate(),
	}

	errs := ValidateDeploymentConfig(config)
	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %v", errs)
	}

	if e, a := "ImageStream", config.Triggers[0].ImageChangeParams.From.Kind; e != a {
		t.Errorf("expected imageChangeParams.from.kind %s, got %s", e, a)
	}
}

func TestValidateDeploymentConfigImageRepositorySupported(t *testing.T) {
	config := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Triggers: []api.DeploymentTriggerPolicy{
			{
				Type: api.DeploymentTriggerOnImageChange,
				ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
					From: kapi.ObjectReference{
						Kind: "ImageRepository",
						Name: "name",
					},
					ContainerNames: []string{"foo"},
				},
			},
		},
		Template: test.OkDeploymentTemplate(),
	}

	errs := ValidateDeploymentConfig(config)
	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %v", errs)
	}

	if e, a := "ImageRepository", config.Triggers[0].ImageChangeParams.From.Kind; e != a {
		t.Errorf("expected imageChangeParams.from.kind %s, got %s", e, a)
	}
}

func mkintp(i int) *int64 {
	v := int64(i)
	return &v
}

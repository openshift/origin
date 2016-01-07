package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"

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

func rollingConfig(interval, updatePeriod, timeout int) api.DeploymentConfig {
	return api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Triggers: manualTrigger(),
			Strategy: api.DeploymentStrategy{
				Type: api.DeploymentStrategyTypeRolling,
				RollingParams: &api.RollingDeploymentStrategyParams{
					IntervalSeconds:     mkint64p(interval),
					UpdatePeriodSeconds: mkint64p(updatePeriod),
					TimeoutSeconds:      mkint64p(timeout),
					MaxSurge:            kutil.NewIntOrStringFromInt(1),
				},
			},
			Template: test.OkPodTemplate(),
			Selector: test.OkSelector(),
		},
	}
}

func rollingConfigMax(maxSurge, maxUnavailable kutil.IntOrString) api.DeploymentConfig {
	return api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Triggers: manualTrigger(),
			Strategy: api.DeploymentStrategy{
				Type: api.DeploymentStrategyTypeRolling,
				RollingParams: &api.RollingDeploymentStrategyParams{
					IntervalSeconds:     mkint64p(1),
					UpdatePeriodSeconds: mkint64p(1),
					TimeoutSeconds:      mkint64p(1),
					MaxSurge:            maxSurge,
					MaxUnavailable:      maxUnavailable,
				},
			},
			Template: test.OkPodTemplate(),
			Selector: test.OkSelector(),
		},
	}
}

func TestValidateDeploymentConfigOK(t *testing.T) {
	errs := ValidateDeploymentConfig(&api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: manualTrigger(),
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplate(),
		},
	})

	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentConfigMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		DeploymentConfig api.DeploymentConfig
		ErrorType        fielderrors.ValidationErrorType
		Field            string
	}{
		"missing name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: "bar"},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"metadata.name",
		},
		"missing namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: ""},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"metadata.namespace",
		},
		"invalid name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "-foo", Namespace: "bar"},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"metadata.name",
		},
		"invalid namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "-bar"},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"metadata.namespace",
		},

		"missing trigger.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								ContainerNames: []string{"foo"},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.triggers[0].type",
		},
		"missing Trigger imageChangeParams.from": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							Type: api.DeploymentTriggerOnImageChange,
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								ContainerNames: []string{"foo"},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.triggers[0].imageChangeParams.from",
		},
		"invalid Trigger imageChangeParams.from.kind": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							Type: api.DeploymentTriggerOnImageChange,
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								From: kapi.ObjectReference{
									Kind: "Invalid",
									Name: "name:tag",
								},
								ContainerNames: []string{"foo"},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"spec.triggers[0].imageChangeParams.from.kind",
		},
		"missing Trigger imageChangeParams.containerNames": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							Type: api.DeploymentTriggerOnImageChange,
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								From: kapi.ObjectReference{
									Kind: "ImageStreamTag",
									Name: "foo:v1",
								},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.triggers[0].imageChangeParams.containerNames",
		},
		"missing strategy.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: manualTrigger(),
					Selector: test.OkSelector(),
					Strategy: api.DeploymentStrategy{
						CustomParams: test.OkCustomParams(),
					},
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.type",
		},
		"missing strategy.customParams": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: manualTrigger(),
					Selector: test.OkSelector(),
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeCustom,
					},
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.customParams",
		},
		"missing spec.strategy.customParams.image": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: manualTrigger(),
					Selector: test.OkSelector(),
					Strategy: api.DeploymentStrategy{
						Type:         api.DeploymentStrategyTypeCustom,
						CustomParams: &api.CustomDeploymentStrategyParams{},
					},
					Template: test.OkPodTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.customParams.image",
		},
		"missing spec.strategy.recreateParams.pre.failurePolicy": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
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
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.recreateParams.pre.failurePolicy",
		},
		"missing spec.strategy.recreateParams.pre.execNewPod": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.recreateParams.pre.execNewPod",
		},
		"missing spec.strategy.recreateParams.pre.execNewPod.command": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
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
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.recreateParams.pre.execNewPod.command",
		},
		"missing spec.strategy.recreateParams.pre.execNewPod.containerName": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
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
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.recreateParams.pre.execNewPod.containerName",
		},
		"invalid spec.strategy.recreateParams.pre.execNewPod.volumes": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod: &api.ExecNewPodHook{
									ContainerName: "container",
									Command:       []string{"cmd"},
									Volumes:       []string{"good", ""},
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.recreateParams.pre.execNewPod.volumes[1]",
		},
		"invalid spec.strategy.rollingParams.intervalSeconds": {
			rollingConfig(-20, 1, 1),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.intervalSeconds",
		},
		"invalid spec.strategy.rollingParams.updatePeriodSeconds": {
			rollingConfig(1, -20, 1),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.updatePeriodSeconds",
		},
		"invalid spec.strategy.rollingParams.timeoutSeconds": {
			rollingConfig(1, 1, -20),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.timeoutSeconds",
		},
		"missing spec.strategy.rollingParams.pre.failurePolicy": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRolling,
						RollingParams: &api.RollingDeploymentStrategyParams{
							IntervalSeconds:     mkint64p(1),
							UpdatePeriodSeconds: mkint64p(1),
							TimeoutSeconds:      mkint64p(20),
							MaxSurge:            kutil.NewIntOrStringFromInt(1),
							Pre: &api.LifecycleHook{
								ExecNewPod: &api.ExecNewPodHook{
									Command:       []string{"cmd"},
									ContainerName: "container",
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.strategy.rollingParams.pre.failurePolicy",
		},
		"both maxSurge and maxUnavailable 0 spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(kutil.NewIntOrStringFromInt(0), kutil.NewIntOrStringFromInt(0)),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid lower bound spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(kutil.NewIntOrStringFromInt(0), kutil.NewIntOrStringFromInt(-100)),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid lower bound spec.strategy.rollingParams.maxSurge": {
			rollingConfigMax(kutil.NewIntOrStringFromInt(-1), kutil.NewIntOrStringFromInt(0)),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxSurge",
		},
		"both maxSurge and maxUnavailable 0 percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(kutil.NewIntOrStringFromString("0%"), kutil.NewIntOrStringFromString("0%")),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid lower bound percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(kutil.NewIntOrStringFromInt(0), kutil.NewIntOrStringFromString("-1%")),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid upper bound percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(kutil.NewIntOrStringFromInt(0), kutil.NewIntOrStringFromString("101%")),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(kutil.NewIntOrStringFromInt(0), kutil.NewIntOrStringFromString("foo")),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid percent spec.strategy.rollingParams.maxSurge": {
			rollingConfigMax(kutil.NewIntOrStringFromString("foo"), kutil.NewIntOrStringFromString("100%")),
			fielderrors.ValidationErrorTypeInvalid,
			"spec.strategy.rollingParams.maxSurge",
		},
	}

	for testName, v := range errorCases {
		errs := ValidateDeploymentConfig(&v.DeploymentConfig)
		if len(v.ErrorType) == 0 {
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("%s: unexpected error: %s", testName, e)
				}
			}
			continue
		}
		if len(errs) == 0 {
			t.Errorf("%s: expected test failure, got success", testName)
		}
		for i := range errs {
			if got, exp := errs[i].(*fielderrors.ValidationError).Type, v.ErrorType; got != exp {
				t.Errorf("%s: expected error \"%v\" to have type %q, but got %q", testName, errs[i], exp, got)
			}
			if got, exp := errs[i].(*fielderrors.ValidationError).Field, v.Field; got != exp {
				t.Errorf("%s: expected error \"%v\" to have field %q, but got %q", testName, errs[i], exp, got)
			}
		}
	}
}

func TestValidateDeploymentConfigUpdate(t *testing.T) {
	oldConfig := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: manualTrigger(),
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplate(),
		},
		Status: api.DeploymentConfigStatus{
			LatestVersion: 5,
		},
	}
	newConfig := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: manualTrigger(),
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplate(),
		},
		Status: api.DeploymentConfigStatus{
			LatestVersion: 3,
		},
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
		oldConfig.Status.LatestVersion = values.oldLatestVersion
		newConfig.Status.LatestVersion = values.newLatestVersion
		errs := ValidateDeploymentConfigUpdate(newConfig, oldConfig)
		if len(errs) == 0 {
			t.Errorf("Expected update failure")
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != fielderrors.ValidationErrorTypeInvalid {
				t.Errorf("expected update error to have type %s: %v", fielderrors.ValidationErrorTypeInvalid, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != "status.latestVersion" {
				t.Errorf("expected update error to have field %s: %v", "latestVersion", errs[i])
			}
		}
	}

	// testing for a successful update
	oldConfig.Status.LatestVersion = 5
	newConfig.Status.LatestVersion = 6
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
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: []api.DeploymentTriggerPolicy{
				{
					Type: api.DeploymentTriggerOnImageChange,
					ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
						From: kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "name:v1",
						},
						ContainerNames: []string{"foo"},
					},
				},
			},
			Selector: test.OkSelector(),
			Template: test.OkPodTemplate(),
			Strategy: test.OkStrategy(),
		},
	}

	if errs := ValidateDeploymentConfig(config); len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %v", errs)
	}
}

func mkint64p(i int) *int64 {
	v := int64(i)
	return &v
}

func mkintp(i int) *int {
	return &i
}

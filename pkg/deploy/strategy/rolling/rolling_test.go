package rolling

import (
	"fmt"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestRolling_deployInitial(t *testing.T) {
	initialStrategyInvoked := false

	strategy := &RollingDeploymentStrategy{
		codec: api.Codec,
		client: &rollingUpdaterClient{
			GetReplicationControllerFn: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to GetReplicationController")
				return nil, nil
			},
		},
		initialStrategy: &testStrategy{
			deployFn: func(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error {
				initialStrategyInvoked = true
				return nil
			},
		},
		rollingUpdate: func(config *kubectl.RollingUpdaterConfig) error {
			t.Fatalf("unexpected call to rollingUpdate")
			return nil
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	err := strategy.Deploy(deployment, []*kapi.ReplicationController{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !initialStrategyInvoked {
		t.Fatalf("expected initial strategy to be invoked")
	}
}

func TestRolling_deployRolling(t *testing.T) {
	latest, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	config := deploytest.OkDeploymentConfig(2)
	config.Template.Strategy = deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeRolling,
		RollingParams: &deployapi.RollingDeploymentStrategyParams{
			IntervalSeconds:     mkintp(1),
			UpdatePeriodSeconds: mkintp(2),
			TimeoutSeconds:      mkintp(3),
		},
	}
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	var rollingConfig *kubectl.RollingUpdaterConfig
	deploymentUpdated := false
	strategy := &RollingDeploymentStrategy{
		codec: api.Codec,
		client: &rollingUpdaterClient{
			GetReplicationControllerFn: func(namespace, name string) (*kapi.ReplicationController, error) {
				if name != latest.Name {
					t.Fatalf("unexpected call to GetReplicationController for %s", name)
				}
				return latest, nil
			},
			UpdateReplicationControllerFn: func(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				if rc.Name != deployment.Name {
					t.Fatalf("unexpected call to UpdateReplicationController for %s", rc.Name)
				}
				deploymentUpdated = true
				return rc, nil
			},
		},
		initialStrategy: &testStrategy{
			deployFn: func(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error {
				t.Fatalf("unexpected call to initial strategy")
				return nil
			},
		},
		rollingUpdate: func(config *kubectl.RollingUpdaterConfig) error {
			rollingConfig = config
			return nil
		},
	}

	err := strategy.Deploy(deployment, []*kapi.ReplicationController{latest})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rollingConfig == nil {
		t.Fatalf("expected rolling update to be invoked")
	}

	if e, a := latest, rollingConfig.OldRc; e != a {
		t.Errorf("expected rollingConfig.OldRc %v, got %v", e, a)
	}

	if e, a := deployment, rollingConfig.NewRc; e != a {
		t.Errorf("expected rollingConfig.NewRc %v, got %v", e, a)
	}

	if e, a := 1*time.Second, rollingConfig.Interval; e != a {
		t.Errorf("expected Interval %d, got %d", e, a)
	}

	if e, a := 2*time.Second, rollingConfig.UpdatePeriod; e != a {
		t.Errorf("expected UpdatePeriod %d, got %d", e, a)
	}

	if e, a := 3*time.Second, rollingConfig.Timeout; e != a {
		t.Errorf("expected Timeout %d, got %d", e, a)
	}

	// verify hack
	if e, a := 1, rollingConfig.NewRc.Spec.Replicas; e != a {
		t.Errorf("expected rollingConfig.NewRc.Spec.Replicas %d, got %d", e, a)
	}

	// verify hack
	if !deploymentUpdated {
		t.Errorf("expected deployment to be updated for source annotation")
	}
	sid := fmt.Sprintf("%s:%s", latest.Name, latest.ObjectMeta.UID)
	if e, a := sid, rollingConfig.NewRc.Annotations[sourceIdAnnotation]; e != a {
		t.Errorf("expected sourceIdAnnotation %s, got %s", e, a)
	}
}

func TestRolling_findLatestDeployment(t *testing.T) {
	deployments := map[string]*kapi.ReplicationController{}
	for i := 1; i <= 10; i++ {
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(i), kapi.Codec)
		deployments[deployment.Name] = deployment
	}

	strategy := &RollingDeploymentStrategy{
		codec: api.Codec,
		client: &rollingUpdaterClient{
			GetReplicationControllerFn: func(namespace, name string) (*kapi.ReplicationController, error) {
				deployment, found := deployments[name]
				if !found {
					return nil, kerrors.NewNotFound("ReplicationController", name)
				}
				return deployment, nil
			},
		},
	}

	type scenario struct {
		old    []string
		latest string
	}

	scenarios := []scenario{
		{
			old: []string{
				"config-1",
				"config-2",
				"config-3",
			},
			latest: "config-3",
		},
		{
			old: []string{
				"config-3",
				"config-1",
				"config-7",
			},
			latest: "config-7",
		},
	}

	for _, scenario := range scenarios {
		old := []*kapi.ReplicationController{}
		for _, oldName := range scenario.old {
			old = append(old, deployments[oldName])
		}
		found, err := strategy.findLatestDeployment(old)
		if err != nil {
			t.Errorf("unexpected error for scenario: %v: %v", scenario, err)
			continue
		}

		if found == nil {
			t.Errorf("expected to find a deployment for scenario: %v", scenario)
			continue
		}

		if e, a := scenario.latest, found.Name; e != a {
			t.Errorf("expected latest %s, got %s for scenario: %v", e, a, scenario)
		}
	}
}

type testStrategy struct {
	deployFn func(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error
}

func (s *testStrategy) Deploy(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error {
	return s.deployFn(deployment, oldDeployments)
}

func mkintp(i int) *int64 {
	v := int64(i)
	return &v
}

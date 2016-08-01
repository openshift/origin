package deployer

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	cmdtest "github.com/openshift/origin/pkg/deploy/cmd/test"
	"github.com/openshift/origin/pkg/deploy/strategy"
	deployutil "github.com/openshift/origin/pkg/deploy/util"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

func TestDeployer_getDeploymentFail(t *testing.T) {
	deployer := &Deployer{
		strategyFor: func(config *deployapi.DeploymentConfig) (strategy.DeploymentStrategy, error) {
			t.Fatal("unexpected call")
			return nil, nil
		},
		getDeployment: func(namespace, name string) (*kapi.ReplicationController, error) {
			return nil, fmt.Errorf("get error")
		},
		getDeployments: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
			t.Fatal("unexpected call")
			return nil, nil
		},
		scaler: &cmdtest.FakeScaler{},
	}

	err := deployer.Deploy("namespace", "name")
	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %v", err)
}

func TestDeployer_deployScenarios(t *testing.T) {
	mkd := func(version int64, status deployapi.DeploymentStatus, replicas int32, desired int32) *kapi.ReplicationController {
		deployment := mkdeployment(version, status)
		deployment.Spec.Replicas = int32(replicas)
		if desired > 0 {
			deployment.Annotations[deployapi.DesiredReplicasAnnotation] = strconv.Itoa(int(desired))
		}
		return deployment
	}
	type scaleEvent struct {
		version int64
		size    int32
	}
	scenarios := []struct {
		name        string
		deployments []*kapi.ReplicationController
		fromVersion int64
		toVersion   int64
		scaleEvents []scaleEvent
	}{
		{
			"initial deployment",
			// existing deployments
			[]*kapi.ReplicationController{
				mkd(1, deployapi.DeploymentStatusNew, 0, 3),
			},
			// from and to version
			0, 1,
			// expected scale events
			[]scaleEvent{},
		},
		{
			"last deploy failed",
			// existing deployments
			[]*kapi.ReplicationController{
				mkd(1, deployapi.DeploymentStatusComplete, 3, 0),
				mkd(2, deployapi.DeploymentStatusFailed, 1, 3),
				mkd(3, deployapi.DeploymentStatusNew, 0, 3),
			},
			// from and to version
			1, 3,
			// expected scale events
			[]scaleEvent{
				{2, 0},
			},
		},
		{
			"sequential complete",
			// existing deployments
			[]*kapi.ReplicationController{
				mkd(1, deployapi.DeploymentStatusComplete, 0, 0),
				mkd(2, deployapi.DeploymentStatusComplete, 3, 0),
				mkd(3, deployapi.DeploymentStatusNew, 0, 3),
			},
			// from and to version
			2, 3,
			// expected scale events
			[]scaleEvent{},
		},
		{
			"sequential failure",
			// existing deployments
			[]*kapi.ReplicationController{
				mkd(1, deployapi.DeploymentStatusFailed, 1, 3),
				mkd(2, deployapi.DeploymentStatusFailed, 1, 3),
				mkd(3, deployapi.DeploymentStatusNew, 0, 3),
			},
			// from and to version
			0, 3,
			// expected scale events
			[]scaleEvent{
				{1, 0},
				{2, 0},
			},
		},
		{
			"version mismatch",
			// existing deployments
			[]*kapi.ReplicationController{
				mkd(1, deployapi.DeploymentStatusComplete, 0, 0),
				mkd(2, deployapi.DeploymentStatusNew, 3, 0),
				mkd(3, deployapi.DeploymentStatusComplete, 0, 3),
			},
			// from and to version
			3, 2,
			// expected scale events
			[]scaleEvent{},
		},
	}

	for _, s := range scenarios {
		t.Logf("executing scenario %s", s.name)
		findDeployment := func(version int64) *kapi.ReplicationController {
			for _, d := range s.deployments {
				if deployutil.DeploymentVersionFor(d) == version {
					return d
				}
			}
			return nil
		}

		var actualFrom, actualTo *kapi.ReplicationController
		var actualDesired int32
		to := findDeployment(s.toVersion)
		scaler := &cmdtest.FakeScaler{}

		deployer := &Deployer{
			out:    &bytes.Buffer{},
			errOut: &bytes.Buffer{},
			strategyFor: func(config *deployapi.DeploymentConfig) (strategy.DeploymentStrategy, error) {
				return &testStrategy{
					deployFunc: func(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
						actualFrom = from
						actualTo = to
						actualDesired = int32(desiredReplicas)
						return nil
					},
				}, nil
			},
			getDeployment: func(namespace, name string) (*kapi.ReplicationController, error) {
				return to, nil
			},
			getDeployments: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				list := &kapi.ReplicationControllerList{}
				for _, d := range s.deployments {
					list.Items = append(list.Items, *d)
				}
				return list, nil
			},
			scaler: scaler,
		}

		err := deployer.Deploy(to.Namespace, to.Name)
		if s.toVersion < s.fromVersion {
			if err == nil {
				t.Fatalf("expected error when toVersion is older than newVersion")
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if s.fromVersion > 0 {
			if e, a := s.fromVersion, deployutil.DeploymentVersionFor(actualFrom); e != a {
				t.Fatalf("expected from.latestVersion %d, got %d", e, a)
			}
		}
		if e, a := s.toVersion, deployutil.DeploymentVersionFor(actualTo); e != a {
			t.Fatalf("expected to.latestVersion %d, got %d", e, a)
		}
		if e, a := len(s.scaleEvents), len(scaler.Events); e != a {
			t.Fatalf("expected %d scale events, got %d", e, a)
		}
		for _, expected := range s.scaleEvents {
			expectedTo := findDeployment(expected.version)
			expectedWasScaled := false
			for _, actual := range scaler.Events {
				if actual.Name != expectedTo.Name {
					continue
				}
				if e, a := uint(expected.size), actual.Size; e != a {
					t.Fatalf("expected version %d to be scaled to %d, got %d", expected.version, e, a)
				}
				expectedWasScaled = true
			}
			if !expectedWasScaled {
				t.Fatalf("expected version %d to be scaled to %d, but it wasn't scaled at all", expected.version, expected.size)
			}
		}
	}
}

func mkdeployment(version int64, status deployapi.DeploymentStatus) *kapi.ReplicationController {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)
	return deployment
}

type testStrategy struct {
	deployFunc func(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error
}

func (t *testStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	return t.deployFunc(from, to, desiredReplicas)
}

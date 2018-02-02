package deployer

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	"github.com/openshift/origin/pkg/apps/strategy"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	cmdtest "github.com/openshift/origin/pkg/apps/util/test"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

func TestDeployer_getDeploymentFail(t *testing.T) {
	deployer := &Deployer{
		strategyFor: func(config *appsapi.DeploymentConfig) (strategy.DeploymentStrategy, error) {
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
	mkd := func(version int64, status appsapi.DeploymentStatus, replicas int32, desired int32) *kapi.ReplicationController {
		deployment := mkdeployment(version, status)
		deployment.Spec.Replicas = int32(replicas)
		if desired > 0 {
			deployment.Annotations[appsapi.DesiredReplicasAnnotation] = strconv.Itoa(int(desired))
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
				mkd(1, appsapi.DeploymentStatusNew, 0, 3),
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
				mkd(1, appsapi.DeploymentStatusComplete, 3, 0),
				mkd(2, appsapi.DeploymentStatusFailed, 1, 3),
				mkd(3, appsapi.DeploymentStatusNew, 0, 3),
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
				mkd(1, appsapi.DeploymentStatusComplete, 0, 0),
				mkd(2, appsapi.DeploymentStatusComplete, 3, 0),
				mkd(3, appsapi.DeploymentStatusNew, 0, 3),
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
				mkd(1, appsapi.DeploymentStatusFailed, 1, 3),
				mkd(2, appsapi.DeploymentStatusFailed, 1, 3),
				mkd(3, appsapi.DeploymentStatusNew, 0, 3),
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
				mkd(1, appsapi.DeploymentStatusComplete, 0, 0),
				mkd(2, appsapi.DeploymentStatusNew, 3, 0),
				mkd(3, appsapi.DeploymentStatusComplete, 0, 3),
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
				if appsutil.DeploymentVersionFor(d) == version {
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
			strategyFor: func(config *appsapi.DeploymentConfig) (strategy.DeploymentStrategy, error) {
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
			if e, a := s.fromVersion, appsutil.DeploymentVersionFor(actualFrom); e != a {
				t.Fatalf("expected from.latestVersion %d, got %d", e, a)
			}
		}
		if e, a := s.toVersion, appsutil.DeploymentVersionFor(actualTo); e != a {
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

func mkdeployment(version int64, status appsapi.DeploymentStatus) *kapi.ReplicationController {
	deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(version), legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))
	deployment.Annotations[appsapi.DeploymentStatusAnnotation] = string(status)
	return deployment
}

type testStrategy struct {
	deployFunc func(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error
}

func (t *testStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	return t.deployFunc(from, to, desiredReplicas)
}

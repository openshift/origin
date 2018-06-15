package prune

import (
	"sort"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

type mockDeleteRecorder struct {
	set sets.String
	err error
}

var _ DeploymentDeleter = &mockDeleteRecorder{}

func (m *mockDeleteRecorder) DeleteDeployment(deployment *kapi.ReplicationController) error {
	m.set.Insert(deployment.Name)
	return m.err
}

func (m *mockDeleteRecorder) Verify(t *testing.T, expected sets.String) {
	if len(m.set) != len(expected) || !m.set.HasAll(expected.List()...) {
		expectedValues := expected.List()
		actualValues := m.set.List()
		sort.Strings(expectedValues)
		sort.Strings(actualValues)
		t.Errorf("expected \n\t%v\n, actual \n\t%v\n", expectedValues, actualValues)
	}
}

func TestPruneTask(t *testing.T) {
	deploymentStatusOptions := []appsapi.DeploymentStatus{
		appsapi.DeploymentStatusComplete,
		appsapi.DeploymentStatusFailed,
		appsapi.DeploymentStatusNew,
		appsapi.DeploymentStatusPending,
		appsapi.DeploymentStatusRunning,
	}
	deploymentStatusFilter := []appsapi.DeploymentStatus{
		appsapi.DeploymentStatusComplete,
		appsapi.DeploymentStatusFailed,
	}
	deploymentStatusFilterSet := sets.String{}
	for _, deploymentStatus := range deploymentStatusFilter {
		deploymentStatusFilterSet.Insert(string(deploymentStatus))
	}

	for _, orphans := range []bool{true, false} {
		for _, deploymentStatusOption := range deploymentStatusOptions {
			keepYoungerThan := time.Hour

			now := metav1.Now()
			old := metav1.NewTime(now.Time.Add(-1 * keepYoungerThan))

			deploymentConfigs := []*appsapi.DeploymentConfig{}
			deployments := []*kapi.ReplicationController{}

			deploymentConfig := mockDeploymentConfig("a", "deployment-config")
			deploymentConfigs = append(deploymentConfigs, deploymentConfig)

			deployments = append(deployments, withCreated(withStatus(mockDeployment("a", "build-1", deploymentConfig), deploymentStatusOption), now))
			deployments = append(deployments, withCreated(withStatus(mockDeployment("a", "build-2", deploymentConfig), deploymentStatusOption), old))
			deployments = append(deployments, withSize(withCreated(withStatus(mockDeployment("a", "build-3-with-replicas", deploymentConfig), deploymentStatusOption), old), 4))
			deployments = append(deployments, withCreated(withStatus(mockDeployment("a", "orphan-build-1", nil), deploymentStatusOption), now))
			deployments = append(deployments, withCreated(withStatus(mockDeployment("a", "orphan-build-2", nil), deploymentStatusOption), old))
			deployments = append(deployments, withSize(withCreated(withStatus(mockDeployment("a", "orphan-build-3-with-replicas", nil), deploymentStatusOption), old), 4))

			keepComplete := 1
			keepFailed := 1
			expectedValues := sets.String{}
			filter := &andFilter{
				filterPredicates: []FilterPredicate{
					FilterDeploymentsPredicate,
					FilterZeroReplicaSize,
					NewFilterBeforePredicate(keepYoungerThan),
				},
			}
			dataSet := NewDataSet(deploymentConfigs, filter.Filter(deployments))
			resolver := NewPerDeploymentConfigResolver(dataSet, keepComplete, keepFailed)
			if orphans {
				resolver = &mergeResolver{
					resolvers: []Resolver{resolver, NewOrphanDeploymentResolver(dataSet, deploymentStatusFilter)},
				}
			}
			expectedDeployments, err := resolver.Resolve()
			if err != nil {
				t.Errorf("Unexpected error %v", err)
			}
			for _, item := range expectedDeployments {
				expectedValues.Insert(item.Name)
			}

			recorder := &mockDeleteRecorder{set: sets.String{}}

			options := PrunerOptions{
				KeepYoungerThan:   keepYoungerThan,
				Orphans:           orphans,
				KeepComplete:      keepComplete,
				KeepFailed:        keepFailed,
				DeploymentConfigs: deploymentConfigs,
				Deployments:       deployments,
			}
			pruner := NewPruner(options)
			if err := pruner.Prune(recorder); err != nil {
				t.Errorf("Unexpected error %v", err)
			}
			recorder.Verify(t, expectedValues)
		}
	}

}

package prune

import (
	"fmt"
	"sort"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type mockResolver struct {
	items []*kapi.ReplicationController
	err   error
}

func (m *mockResolver) Resolve() ([]*kapi.ReplicationController, error) {
	return m.items, m.err
}

func TestMergeResolver(t *testing.T) {
	resolverA := &mockResolver{
		items: []*kapi.ReplicationController{
			mockDeployment("a", "b", nil),
		},
	}
	resolverB := &mockResolver{
		items: []*kapi.ReplicationController{
			mockDeployment("c", "d", nil),
		},
	}
	resolver := &mergeResolver{resolvers: []Resolver{resolverA, resolverB}}
	results, err := resolver.Resolve()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Unexpected results %v", results)
	}
	expectedNames := sets.NewString("b", "d")
	for _, item := range results {
		if !expectedNames.Has(item.Name) {
			t.Errorf("Unexpected name %v", item.Name)
		}
	}
}

func TestOrphanDeploymentResolver(t *testing.T) {
	activeDeploymentConfig := mockDeploymentConfig("a", "active-deployment-config")
	inactiveDeploymentConfig := mockDeploymentConfig("a", "inactive-deployment-config")

	deploymentConfigs := []*deployapi.DeploymentConfig{activeDeploymentConfig}
	deployments := []*kapi.ReplicationController{}

	expectedNames := sets.String{}
	deploymentStatusOptions := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusComplete,
		deployapi.DeploymentStatusFailed,
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
	}

	deploymentStatusFilter := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusComplete,
		deployapi.DeploymentStatusFailed,
	}
	deploymentStatusFilterSet := sets.String{}
	for _, deploymentStatus := range deploymentStatusFilter {
		deploymentStatusFilterSet.Insert(string(deploymentStatus))
	}

	for _, deploymentStatusOption := range deploymentStatusOptions {
		deployments = append(deployments, withStatus(mockDeployment("a", string(deploymentStatusOption)+"-active", activeDeploymentConfig), deploymentStatusOption))
		deployments = append(deployments, withStatus(mockDeployment("a", string(deploymentStatusOption)+"-inactive", inactiveDeploymentConfig), deploymentStatusOption))
		deployments = append(deployments, withStatus(mockDeployment("a", string(deploymentStatusOption)+"-orphan", nil), deploymentStatusOption))
		if deploymentStatusFilterSet.Has(string(deploymentStatusOption)) {
			expectedNames.Insert(string(deploymentStatusOption) + "-inactive")
			expectedNames.Insert(string(deploymentStatusOption) + "-orphan")
		}
	}

	dataSet := NewDataSet(deploymentConfigs, deployments)
	resolver := NewOrphanDeploymentResolver(dataSet, deploymentStatusFilter)
	results, err := resolver.Resolve()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	foundNames := sets.String{}
	for _, result := range results {
		foundNames.Insert(result.Name)
	}
	if len(foundNames) != len(expectedNames) || !expectedNames.HasAll(foundNames.List()...) {
		t.Errorf("expected %v, actual %v", expectedNames, foundNames)
	}
}

func TestPerDeploymentConfigResolver(t *testing.T) {
	deploymentStatusOptions := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusComplete,
		deployapi.DeploymentStatusFailed,
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
	}
	deploymentConfigs := []*deployapi.DeploymentConfig{
		mockDeploymentConfig("a", "deployment-config-1"),
		mockDeploymentConfig("b", "deployment-config-2"),
	}
	deploymentsPerStatus := 100
	deployments := []*kapi.ReplicationController{}
	for _, deploymentConfig := range deploymentConfigs {
		for _, deploymentStatusOption := range deploymentStatusOptions {
			for i := 0; i < deploymentsPerStatus; i++ {
				deployment := withStatus(mockDeployment(deploymentConfig.Namespace, fmt.Sprintf("%v-%v-%v", deploymentConfig.Name, deploymentStatusOption, i), deploymentConfig), deploymentStatusOption)
				deployments = append(deployments, deployment)
			}
		}
	}

	now := util.Now()
	for i := range deployments {
		creationTimestamp := util.NewTime(now.Time.Add(-1 * time.Duration(i) * time.Hour))
		deployments[i].CreationTimestamp = creationTimestamp
	}

	// test number to keep at varying ranges
	for keep := 0; keep < deploymentsPerStatus*2; keep++ {
		dataSet := NewDataSet(deploymentConfigs, deployments)

		expectedNames := sets.String{}
		deploymentCompleteStatusFilterSet := sets.NewString(string(deployapi.DeploymentStatusComplete))
		deploymentFailedStatusFilterSet := sets.NewString(string(deployapi.DeploymentStatusFailed))

		for _, deploymentConfig := range deploymentConfigs {
			deploymentItems, err := dataSet.ListDeploymentsByDeploymentConfig(deploymentConfig)
			if err != nil {
				t.Errorf("Unexpected err %v", err)
			}
			completedDeployments, failedDeployments := []*kapi.ReplicationController{}, []*kapi.ReplicationController{}
			for _, deployment := range deploymentItems {
				status := deployment.Annotations[deployapi.DeploymentStatusAnnotation]
				if deploymentCompleteStatusFilterSet.Has(status) {
					completedDeployments = append(completedDeployments, deployment)
				} else if deploymentFailedStatusFilterSet.Has(status) {
					failedDeployments = append(failedDeployments, deployment)
				}
			}
			sort.Sort(sortableReplicationControllers(completedDeployments))
			sort.Sort(sortableReplicationControllers(failedDeployments))
			purgeCompleted := []*kapi.ReplicationController{}
			purgeFailed := []*kapi.ReplicationController{}
			if keep >= 0 && keep < len(completedDeployments) {
				purgeCompleted = completedDeployments[keep:]
			}
			if keep >= 0 && keep < len(failedDeployments) {
				purgeFailed = failedDeployments[keep:]
			}
			for _, deployment := range purgeCompleted {
				expectedNames.Insert(deployment.Name)
			}
			for _, deployment := range purgeFailed {
				expectedNames.Insert(deployment.Name)
			}
		}

		resolver := NewPerDeploymentConfigResolver(dataSet, keep, keep)
		results, err := resolver.Resolve()
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		foundNames := sets.String{}
		for _, result := range results {
			foundNames.Insert(result.Name)
		}
		if len(foundNames) != len(expectedNames) || !expectedNames.HasAll(foundNames.List()...) {
			expectedValues := expectedNames.List()
			actualValues := foundNames.List()
			sort.Strings(expectedValues)
			sort.Strings(actualValues)
			t.Errorf("keep %v\n, expected \n\t%v\n, actual \n\t%v\n", keep, expectedValues, actualValues)
		}
	}
}

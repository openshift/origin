package deployments

import (
	"fmt"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/openshift/api/apps/v1"
)

type mockResolver struct {
	items []*corev1.ReplicationController
	err   error
}

func (m *mockResolver) Resolve() ([]*corev1.ReplicationController, error) {
	return m.items, m.err
}

func TestMergeResolver(t *testing.T) {
	resolverA := &mockResolver{
		items: []*corev1.ReplicationController{
			mockDeployment("a", "b", nil),
		},
	}
	resolverB := &mockResolver{
		items: []*corev1.ReplicationController{
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

	deploymentConfigs := []*appsv1.DeploymentConfig{activeDeploymentConfig}
	deployments := []*corev1.ReplicationController{}

	expectedNames := sets.String{}
	deploymentStatusOptions := []appsv1.DeploymentStatus{
		appsv1.DeploymentStatusComplete,
		appsv1.DeploymentStatusFailed,
		appsv1.DeploymentStatusNew,
		appsv1.DeploymentStatusPending,
		appsv1.DeploymentStatusRunning,
	}

	deploymentStatusFilter := []appsv1.DeploymentStatus{
		appsv1.DeploymentStatusComplete,
		appsv1.DeploymentStatusFailed,
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
	deploymentStatusOptions := []appsv1.DeploymentStatus{
		appsv1.DeploymentStatusComplete,
		appsv1.DeploymentStatusFailed,
		appsv1.DeploymentStatusNew,
		appsv1.DeploymentStatusPending,
		appsv1.DeploymentStatusRunning,
	}
	deploymentConfigs := []*appsv1.DeploymentConfig{
		mockDeploymentConfig("a", "deployment-config-1"),
		mockDeploymentConfig("b", "deployment-config-2"),
	}
	deploymentsPerStatus := 100
	deployments := []*corev1.ReplicationController{}
	for _, deploymentConfig := range deploymentConfigs {
		for _, deploymentStatusOption := range deploymentStatusOptions {
			for i := 0; i < deploymentsPerStatus; i++ {
				deployment := withStatus(mockDeployment(deploymentConfig.Namespace, fmt.Sprintf("%v-%v-%v", deploymentConfig.Name, deploymentStatusOption, i), deploymentConfig), deploymentStatusOption)
				deployments = append(deployments, deployment)
			}
		}
	}

	now := metav1.Now()
	for i := range deployments {
		creationTimestamp := metav1.NewTime(now.Time.Add(-1 * time.Duration(i) * time.Hour))
		deployments[i].CreationTimestamp = creationTimestamp
	}

	// test number to keep at varying ranges
	for keep := 0; keep < deploymentsPerStatus*2; keep++ {
		dataSet := NewDataSet(deploymentConfigs, deployments)

		expectedNames := sets.String{}
		deploymentCompleteStatusFilterSet := sets.NewString(string(appsv1.DeploymentStatusComplete))
		deploymentFailedStatusFilterSet := sets.NewString(string(appsv1.DeploymentStatusFailed))

		for _, deploymentConfig := range deploymentConfigs {
			deploymentItems, err := dataSet.ListDeploymentsByDeploymentConfig(deploymentConfig)
			if err != nil {
				t.Errorf("Unexpected err %v", err)
			}
			completedDeployments, failedDeployments := []*corev1.ReplicationController{}, []*corev1.ReplicationController{}
			for _, deployment := range deploymentItems {
				status := deployment.Annotations[appsv1.DeploymentStatusAnnotation]
				if deploymentCompleteStatusFilterSet.Has(status) {
					completedDeployments = append(completedDeployments, deployment)
				} else if deploymentFailedStatusFilterSet.Has(status) {
					failedDeployments = append(failedDeployments, deployment)
				}
			}
			sort.Sort(ByMostRecent(completedDeployments))
			sort.Sort(ByMostRecent(failedDeployments))
			purgeCompleted := []*corev1.ReplicationController{}
			purgeFailed := []*corev1.ReplicationController{}
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

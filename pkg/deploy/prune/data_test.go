package prune

import (
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func mockDeploymentConfig(namespace, name string) *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
}

func withSize(item *kapi.ReplicationController, replicas int) *kapi.ReplicationController {
	item.Spec.Replicas = replicas
	item.Status.Replicas = replicas
	return item
}

func withCreated(item *kapi.ReplicationController, creationTimestamp util.Time) *kapi.ReplicationController {
	item.CreationTimestamp = creationTimestamp
	return item
}

func withStatus(item *kapi.ReplicationController, status deployapi.DeploymentStatus) *kapi.ReplicationController {
	item.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)
	return item
}

func mockDeployment(namespace, name string, deploymentConfig *deployapi.DeploymentConfig) *kapi.ReplicationController {
	item := &kapi.ReplicationController{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name, Annotations: map[string]string{}}}
	if deploymentConfig != nil {
		item.Annotations[deployapi.DeploymentConfigAnnotation] = deploymentConfig.Name
	}
	item.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	return item
}

func TestDeploymentByDeploymentConfigIndexFunc(t *testing.T) {
	config := mockDeploymentConfig("a", "b")
	deployment := mockDeployment("a", "c", config)
	actualKey, err := DeploymentByDeploymentConfigIndexFunc(deployment)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	expectedKey := []string{"a/b"}
	if !reflect.DeepEqual(actualKey, expectedKey) {
		t.Errorf("expected %v, actual %v", expectedKey, actualKey)
	}
	deploymentWithNoConfig := &kapi.ReplicationController{}
	actualKey, err = DeploymentByDeploymentConfigIndexFunc(deploymentWithNoConfig)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	expectedKey = []string{"orphan"}
	if !reflect.DeepEqual(actualKey, expectedKey) {
		t.Errorf("expected %v, actual %v", expectedKey, actualKey)
	}
}

func TestFilterBeforePredicate(t *testing.T) {
	youngerThan := time.Hour
	now := util.Now()
	old := util.NewTime(now.Time.Add(-1 * youngerThan))
	items := []*kapi.ReplicationController{}
	items = append(items, withCreated(mockDeployment("a", "old", nil), old))
	items = append(items, withCreated(mockDeployment("a", "new", nil), now))
	filter := &andFilter{
		filterPredicates: []FilterPredicate{NewFilterBeforePredicate(youngerThan)},
	}
	result := filter.Filter(items)
	if len(result) != 1 {
		t.Errorf("Unexpected number of results")
	}
	if expected, actual := "old", result[0].Name; expected != actual {
		t.Errorf("expected %v, actual %v", expected, actual)
	}
}

func TestEmptyDataSet(t *testing.T) {
	deployments := []*kapi.ReplicationController{}
	deploymentConfigs := []*deployapi.DeploymentConfig{}
	dataSet := NewDataSet(deploymentConfigs, deployments)
	_, exists, err := dataSet.GetDeploymentConfig(&kapi.ReplicationController{})
	if exists || err != nil {
		t.Errorf("Unexpected result %v, %v", exists, err)
	}
	deploymentConfigResults, err := dataSet.ListDeploymentConfigs()
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(deploymentConfigResults) != 0 {
		t.Errorf("Unexpected result %v", deploymentConfigResults)
	}
	deploymentResults, err := dataSet.ListDeployments()
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(deploymentResults) != 0 {
		t.Errorf("Unexpected result %v", deploymentResults)
	}
	deploymentResults, err = dataSet.ListDeploymentsByDeploymentConfig(&deployapi.DeploymentConfig{})
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(deploymentResults) != 0 {
		t.Errorf("Unexpected result %v", deploymentResults)
	}
}

func TestPopulatedDataSet(t *testing.T) {
	deploymentConfigs := []*deployapi.DeploymentConfig{
		mockDeploymentConfig("a", "deployment-config-1"),
		mockDeploymentConfig("b", "deployment-config-2"),
	}
	deployments := []*kapi.ReplicationController{
		mockDeployment("a", "deployment-1", deploymentConfigs[0]),
		mockDeployment("a", "deployment-2", deploymentConfigs[0]),
		mockDeployment("b", "deployment-3", deploymentConfigs[1]),
		mockDeployment("c", "deployment-4", nil),
	}
	dataSet := NewDataSet(deploymentConfigs, deployments)
	for _, deployment := range deployments {
		deploymentConfig, exists, err := dataSet.GetDeploymentConfig(deployment)
		config, hasConfig := deployment.Annotations[deployapi.DeploymentConfigAnnotation]
		if hasConfig {
			if err != nil {
				t.Errorf("Item %v, unexpected error: %v", deployment, err)
			}
			if !exists {
				t.Errorf("Item %v, unexpected result: %v", deployment, exists)
			}
			if expected, actual := config, deploymentConfig.Name; expected != actual {
				t.Errorf("expected %v, actual %v", expected, actual)
			}
			if expected, actual := deployment.Namespace, deploymentConfig.Namespace; expected != actual {
				t.Errorf("expected %v, actual %v", expected, actual)
			}
		} else {
			if err != nil {
				t.Errorf("Item %v, unexpected error: %v", deployment, err)
			}
			if exists {
				t.Errorf("Item %v, unexpected result: %v", deployment, exists)
			}
		}
	}
	expectedNames := util.NewStringSet("deployment-1", "deployment-2")
	deploymentResults, err := dataSet.ListDeploymentsByDeploymentConfig(deploymentConfigs[0])
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(deploymentResults) != len(expectedNames) {
		t.Errorf("Unexpected result %v", deploymentResults)
	}
	for _, deployment := range deploymentResults {
		if !expectedNames.Has(deployment.Name) {
			t.Errorf("Unexpected name: %v", deployment.Name)
		}
	}
}

package deployments

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/openshift/api/apps/v1"
)

func mockDeploymentConfig(namespace, name string) *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

func withSize(item *corev1.ReplicationController, replicas int32) *corev1.ReplicationController {
	item.Spec.Replicas = &replicas
	item.Status.Replicas = int32(replicas)
	return item
}

func withCreated(item *corev1.ReplicationController, creationTimestamp metav1.Time) *corev1.ReplicationController {
	item.CreationTimestamp = creationTimestamp
	return item
}

func withStatus(item *corev1.ReplicationController, status appsv1.DeploymentStatus) *corev1.ReplicationController {
	item.Annotations[appsv1.DeploymentStatusAnnotation] = string(status)
	return item
}

func mockDeployment(namespace, name string, deploymentConfig *appsv1.DeploymentConfig) *corev1.ReplicationController {
	zero := int32(0)
	item := &corev1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Annotations: map[string]string{}},
		Spec:       corev1.ReplicationControllerSpec{Replicas: &zero},
	}
	if deploymentConfig != nil {
		item.Annotations[appsv1.DeploymentConfigAnnotation] = deploymentConfig.Name
	}
	item.Annotations[appsv1.DeploymentStatusAnnotation] = string(appsv1.DeploymentStatusNew)
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
	deploymentWithNoConfig := &corev1.ReplicationController{}
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
	now := metav1.Now()
	old := metav1.NewTime(now.Time.Add(-1 * youngerThan))
	items := []*corev1.ReplicationController{}
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
	deployments := []*corev1.ReplicationController{}
	deploymentConfigs := []*appsv1.DeploymentConfig{}
	dataSet := NewDataSet(deploymentConfigs, deployments)
	_, exists, err := dataSet.GetDeploymentConfig(&corev1.ReplicationController{})
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
	deploymentResults, err = dataSet.ListDeploymentsByDeploymentConfig(&appsv1.DeploymentConfig{})
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(deploymentResults) != 0 {
		t.Errorf("Unexpected result %v", deploymentResults)
	}
}

func TestPopulatedDataSet(t *testing.T) {
	deploymentConfigs := []*appsv1.DeploymentConfig{
		mockDeploymentConfig("a", "deployment-config-1"),
		mockDeploymentConfig("b", "deployment-config-2"),
	}
	deployments := []*corev1.ReplicationController{
		mockDeployment("a", "deployment-1", deploymentConfigs[0]),
		mockDeployment("a", "deployment-2", deploymentConfigs[0]),
		mockDeployment("b", "deployment-3", deploymentConfigs[1]),
		mockDeployment("c", "deployment-4", nil),
	}
	dataSet := NewDataSet(deploymentConfigs, deployments)
	for _, deployment := range deployments {
		deploymentConfig, exists, err := dataSet.GetDeploymentConfig(deployment)
		config, hasConfig := deployment.Annotations[appsv1.DeploymentConfigAnnotation]
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
	expectedNames := sets.NewString("deployment-1", "deployment-2")
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

package controller

import (
	"strconv"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHandleNewDeploymentConfig(t *testing.T) {
	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with name %s", name)
				return nil, nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			deploymentConfig := manualDeploymentConfig()
			deploymentConfig.LatestVersion = 0
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()
}

func TestHandleInitialDeployment(t *testing.T) {
	deploymentConfig := manualDeploymentConfig()
	deploymentConfig.LatestVersion = 1

	var deployed *kapi.ReplicationController

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("replicationController", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}

	expectedAnnotations := map[string]string{
		deployapi.DeploymentConfigAnnotation:  deploymentConfig.Name,
		deployapi.DeploymentStatusAnnotation:  string(deployapi.DeploymentStatusNew),
		deployapi.DeploymentVersionAnnotation: strconv.Itoa(deploymentConfig.LatestVersion),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployed.Annotations[key]; actual != expected {
			t.Fatalf("expected deployment annotation %s=%s, got %s", key, expected, actual)
		}
	}

	// TODO: add stronger assertion on the encoded value once the controller methods are free
	// of side effects on the deploymentConfig
	if len(deployed.Annotations[deployapi.DeploymentEncodedConfigAnnotation]) == 0 {
		t.Fatalf("expected deployment with DeploymentEncodedConfigAnnotation annotation")
	}
}

func TestHandleConfigChangeNoPodTemplateDiff(t *testing.T) {
	deploymentConfig := manualDeploymentConfig()
	deploymentConfig.LatestVersion = 0

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return matchingDeployment(deploymentConfig), nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()
}

func TestHandleConfigChangeWithPodTemplateDiff(t *testing.T) {
	deploymentConfig := manualDeploymentConfig()
	deploymentConfig.LatestVersion = 2
	deploymentConfig.Template.ControllerTemplate.Template.Labels["foo"] = "bar"

	var deployed *kapi.ReplicationController

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("deployment", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}

	if e, a := deploymentConfig.Name, deployed.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("expected deployment annotated with deploymentConfig %s, got %s", e, a)
	}
}

type testDeploymentInterface struct {
	GetDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	CreateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *testDeploymentInterface) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.GetDeploymentFunc(namespace, name)
}

func (i *testDeploymentInterface) CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.CreateDeploymentFunc(namespace, deployment)
}

func manualDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "manual-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerManual,
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
							},
						},
					},
				},
			},
		},
	}
}

func matchingDeployment(config *deployapi.DeploymentConfig) *kapi.ReplicationController {
	encodedConfig, _ := deployutil.EncodeDeploymentConfig(config, api.Codec)
	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.LatestDeploymentIDForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
			},
			Labels: config.Labels,
		},
		Spec: kapi.ReplicationControllerSpec{
			Replicas: 1,
			Selector: map[string]string{
				"name": "test-pod",
			},
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{
						"name": "test-pod",
					},
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "container-1",
							Image: "registry:8080/openshift/test-image:ref-1",
						},
					},
				},
			},
		},
	}
}

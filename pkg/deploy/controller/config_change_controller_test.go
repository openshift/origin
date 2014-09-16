package controller

import (
  "testing"

  kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
  deployapi "github.com/openshift/origin/pkg/deploy/api"
  deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
)

// Test the controller's response to a new DeploymentConfig
func TestNewConfig(t *testing.T) {
  generated := false
  updated := false

  controller := &ConfigChangeController{
    DeploymentConfigInterface: &testDeploymentConfigInterface{
      GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
        generated = true
        return nil, nil
      },
      UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
        updated = true
        return config, nil
      },
    },
    NextDeploymentConfig: func() *deployapi.DeploymentConfig {
      return initialConfig()
    },
    DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment()),
  }

  controller.HandleDeploymentConfig()

  if generated {
    t.Error("Unexpected generation of deploymentConfig")
  }

  if updated {
    t.Error("Unexpected update of deploymentConfig")
  }
}

// Test the controller's response when the pod template is changed
func TestChangeWithTemplateDiff(t *testing.T) {
  var (
    generatedId string
    updated     *deployapi.DeploymentConfig
  )

  controller := &ConfigChangeController{
    DeploymentConfigInterface: &testDeploymentConfigInterface{
      GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
        generatedId = id
        return generatedConfig(), nil
      },
      UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
        updated = config
        return config, nil
      },
    },
    NextDeploymentConfig: func() *deployapi.DeploymentConfig {
      return diffedConfig()
    },
    DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment()),
  }

  controller.HandleDeploymentConfig()

  if generatedId != "test-deploy-config" {
    t.Fatalf("Unexpected generated config id.  Expected test-deploy-config, got: %v", generatedId)
  }

  if updated.ID != "test-deploy-config" {
    t.Fatalf("Unexpected updated config id.  Expected test-deploy-config, got: %v", updated.ID)
  }
}

func TestChangeWithoutTemplateDiff(t *testing.T) {
  generated := false
  updated := false

  controller := &ConfigChangeController{
    DeploymentConfigInterface: &testDeploymentConfigInterface{
      GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
        generated = true
        return nil, nil
      },
      UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
        updated = true
        return config, nil
      },
    },
    NextDeploymentConfig: func() *deployapi.DeploymentConfig {
      return initialConfig()
    },
    DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment()),
  }

  controller.HandleDeploymentConfig()

  if generated {
    t.Error("Unexpected generation of deploymentConfig")
  }

  if updated {
    t.Error("Unexpected update of deploymentConfig")
  }
}

type testDeploymentConfigInterface struct {
  GenerateDeploymentConfigFunc func(id string) (*deployapi.DeploymentConfig, error)
  UpdateDeploymentConfigFunc   func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (i *testDeploymentConfigInterface) GenerateDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
  return i.GenerateDeploymentConfigFunc(id)
}

func (i *testDeploymentConfigInterface) UpdateDeploymentConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
  return i.UpdateDeploymentConfigFunc(config)
}

func initialConfig() *deployapi.DeploymentConfig {
  return &deployapi.DeploymentConfig{
    JSONBase: kapi.JSONBase{ID: "test-deploy-config"},
    Triggers: []deployapi.DeploymentTriggerPolicy{
      {
        Type: deployapi.DeploymentTriggerOnConfigChange,
      },
    },
    LatestVersion: 2,
    Template: deployapi.DeploymentTemplate{
      Strategy: deployapi.DeploymentStrategy{
        Type: "customPod",
        CustomPod: &deployapi.CustomPodDeploymentStrategy{
          Image: "registry:8080/openshift/kube-deploy",
        },
      },
      ControllerTemplate: kapi.ReplicationControllerState{
        Replicas: 1,
        ReplicaSelector: map[string]string{
          "name": "test-pod",
        },
        PodTemplate: kapi.PodTemplate{
          Labels: map[string]string{
            "name": "test-pod",
          },
          DesiredState: kapi.PodState{
            Manifest: kapi.ContainerManifest{
              Version: "v1beta1",
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
    },
  }
}

func diffedConfig() *deployapi.DeploymentConfig {
  return &deployapi.DeploymentConfig{
    JSONBase: kapi.JSONBase{ID: "test-deploy-config"},
    Triggers: []deployapi.DeploymentTriggerPolicy{
      {
        Type: deployapi.DeploymentTriggerOnConfigChange,
      },
    },
    LatestVersion: 2,
    Template: deployapi.DeploymentTemplate{
      Strategy: deployapi.DeploymentStrategy{
        Type: "customPod",
        CustomPod: &deployapi.CustomPodDeploymentStrategy{
          Image: "registry:8080/openshift/kube-deploy",
        },
      },
      ControllerTemplate: kapi.ReplicationControllerState{
        Replicas: 1,
        ReplicaSelector: map[string]string{
          "name": "test-pod-2",
        },
        PodTemplate: kapi.PodTemplate{
          Labels: map[string]string{
            "name": "test-pod-2",
          },
          DesiredState: kapi.PodState{
            Manifest: kapi.ContainerManifest{
              Version: "v1beta1",
              Containers: []kapi.Container{
                {
                  Name:  "container-2",
                  Image: "registry:8080/openshift/test-image:ref-1",
                },
              },
            },
          },
        },
      },
    },
  }
}

func generatedConfig() *deployapi.DeploymentConfig {
  return &deployapi.DeploymentConfig{
    JSONBase: kapi.JSONBase{ID: "test-deploy-config"},
    Triggers: []deployapi.DeploymentTriggerPolicy{
      {
        Type: deployapi.DeploymentTriggerOnConfigChange,
      },
    },
    LatestVersion: 3,
    Template: deployapi.DeploymentTemplate{
      Strategy: deployapi.DeploymentStrategy{
        Type: "customPod",
        CustomPod: &deployapi.CustomPodDeploymentStrategy{
          Image: "registry:8080/openshift/kube-deploy",
        },
      },
      ControllerTemplate: kapi.ReplicationControllerState{
        Replicas: 1,
        ReplicaSelector: map[string]string{
          "name": "test-pod",
        },
        PodTemplate: kapi.PodTemplate{
          Labels: map[string]string{
            "name": "test-pod",
          },
          DesiredState: kapi.PodState{
            Manifest: kapi.ContainerManifest{
              Version: "v1beta1",
              Containers: []kapi.Container{
                {
                  Name:  "container-1",
                  Image: "registry:8080/openshift/test-image:ref-2",
                },
              },
            },
          },
        },
      },
    },
  }
}

func matchingInitialDeployment() *deployapi.Deployment {
  return &deployapi.Deployment{
    JSONBase: kapi.JSONBase{ID: "test-deploy-config-1"},
    State:    deployapi.DeploymentStateNew,
    Strategy: deployapi.DeploymentStrategy{
      Type: "customPod",
      CustomPod: &deployapi.CustomPodDeploymentStrategy{
        Image:       "registry:8080/repo1:ref1",
        Environment: []kapi.EnvVar{},
      },
    },
    ControllerTemplate: kapi.ReplicationControllerState{
      Replicas: 1,
      ReplicaSelector: map[string]string{
        "name": "test-pod",
      },
      PodTemplate: kapi.PodTemplate{
        Labels: map[string]string{
          "name": "test-pod",
        },
        DesiredState: kapi.PodState{
          Manifest: kapi.ContainerManifest{
            Version: "v1beta1",
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

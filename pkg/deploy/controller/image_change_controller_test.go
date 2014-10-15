package controller

import (
  "testing"

  kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
  deployapi "github.com/openshift/origin/pkg/deploy/api"
  deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
  imageapi "github.com/openshift/origin/pkg/image/api"
)

type testIcDeploymentConfigInterface struct {
  UpdateDeploymentConfigFunc   func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
  GenerateDeploymentConfigFunc func(id string) (*deployapi.DeploymentConfig, error)
}

func (i *testIcDeploymentConfigInterface) UpdateDeploymentConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
  return i.UpdateDeploymentConfigFunc(config)
}
func (i *testIcDeploymentConfigInterface) GenerateDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
  return i.GenerateDeploymentConfigFunc(id)
}

func TestImageChangeForUnregisteredTag(t *testing.T) {
  configWithManualTrigger := imageChangeDeploymentConfig()
  configWithManualTrigger.Triggers[0].ImageChangeParams.Automatic = false

  controller := &ImageChangeController{
    DeploymentConfigInterface: &testIcDeploymentConfigInterface{
      UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
        t.Fatalf("unexpected deployment config update")
        return nil, nil
      },
      GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
        t.Fatalf("unexpected generator call")
        return nil, nil
      },
    },
    NextImageRepository: func() *imageapi.ImageRepository {
      return tagUpdate()
    },
    DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(configWithManualTrigger),
  }

  // verify no-op
  controller.OneImageRepo()
}

func TestImageChange(t *testing.T) {
  var (
    generatedConfig *deployapi.DeploymentConfig
    updatedConfig   *deployapi.DeploymentConfig
  )

  controller := &ImageChangeController{
    DeploymentConfigInterface: &testIcDeploymentConfigInterface{
      UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
        updatedConfig = config
        return updatedConfig, nil
      },
      GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
        generatedConfig = regeneratedConfig()
        return generatedConfig, nil
      },
    },
    NextImageRepository: func() *imageapi.ImageRepository {
      return tagUpdate()
    },
    DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(imageChangeDeploymentConfig()),
  }

  controller.OneImageRepo()

  if generatedConfig == nil {
    t.Fatalf("expected config generation to occur")
  }

  if updatedConfig == nil {
    t.Fatalf("expected an updated deployment config")
  }

  if e, a := updatedConfig.ID, generatedConfig.ID; e != a {
    t.Fatalf("expected updated config with id %s, got %s", e, a)
  }
}

// Utilities and convenience methods

func originalImageRepo() *imageapi.ImageRepository {
  return &imageapi.ImageRepository{
    JSONBase:              kapi.JSONBase{ID: "test-image-repo"},
    DockerImageRepository: "registry:8080/openshift/test-image",
    Tags: map[string]string{
      "test-tag": "ref-1",
    },
  }
}

func unregisteredTagUpdate() *imageapi.ImageRepository {
  return &imageapi.ImageRepository{
    JSONBase:              kapi.JSONBase{ID: "test-image-repo"},
    DockerImageRepository: "registry:8080/openshift/test-image",
    Tags: map[string]string{
      "test-tag":       "ref-1",
      "other-test-tag": "ref-x",
    },
  }
}

func tagUpdate() *imageapi.ImageRepository {
  return &imageapi.ImageRepository{
    JSONBase:              kapi.JSONBase{ID: "test-image-repo"},
    DockerImageRepository: "registry:8080/openshift/test-image",
    Tags: map[string]string{
      "test-tag": "ref-2",
    },
  }
}

func imageChangeDeploymentConfig() *deployapi.DeploymentConfig {
  return &deployapi.DeploymentConfig{
    JSONBase: kapi.JSONBase{ID: "image-change-deploy-config"},
    Triggers: []deployapi.DeploymentTriggerPolicy{
      {
        Type: deployapi.DeploymentTriggerOnImageChange,
        ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
          Automatic:      true,
          ContainerNames: []string{"container-1"},
          RepositoryName: "registry:8080/openshift/test-image",
          Tag:            "test-tag",
        },
      },
    },
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

func regeneratedConfig() *deployapi.DeploymentConfig {
  return &deployapi.DeploymentConfig{
    JSONBase: kapi.JSONBase{ID: "image-change-deploy-config"},
    Triggers: []deployapi.DeploymentTriggerPolicy{
      {
        Type: deployapi.DeploymentTriggerOnImageChange,
        ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
          Automatic:      true,
          ContainerNames: []string{"container-1"},
          RepositoryName: "registry:8080/openshift/test-image",
          Tag:            "test-tag",
        },
      },
    },
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

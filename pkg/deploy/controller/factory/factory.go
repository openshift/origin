package factory

import (
  kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
  kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
  "github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
  "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
  "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
  "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

  osclient "github.com/openshift/origin/pkg/client"
  deployapi "github.com/openshift/origin/pkg/deploy/api"
  controller "github.com/openshift/origin/pkg/deploy/controller"
  imageapi "github.com/openshift/origin/pkg/image/api"
)

type DeploymentControllerFactory struct {
  Client      *osclient.Client
  KubeClient  *kclient.Client
  Environment []kapi.EnvVar
}

func (factory *DeploymentControllerFactory) Create() *controller.DeploymentController {
  queue := cache.NewFIFO()
  cache.NewReflector(&deploymentLW{factory.Client}, &deployapi.Deployment{}, queue).Run()

  return &controller.DeploymentController{
    DeploymentInterface: factory.Client,
    PodInterface:        factory.KubeClient,
    Environment:         factory.Environment,
    NextDeployment: func() *deployapi.Deployment {
      return queue.Pop().(*deployapi.Deployment)
    },
  }
}

type DeploymentConfigControllerFactory struct {
  Client *osclient.Client
}

func (factory *DeploymentConfigControllerFactory) Create() *controller.DeploymentConfigController {
  queue := cache.NewFIFO()
  cache.NewReflector(&deploymentConfigLW{factory.Client}, &deployapi.DeploymentConfig{}, queue).Run()

  return &controller.DeploymentConfigController{
    DeploymentInterface: factory.Client,
    NextDeploymentConfig: func() *deployapi.DeploymentConfig {
      return queue.Pop().(*deployapi.DeploymentConfig)
    },
  }
}

type ConfigChangeControllerFactory struct {
  Client osclient.Interface
}

func (factory *ConfigChangeControllerFactory) Create() *controller.ConfigChangeController {
  queue := cache.NewFIFO()
  cache.NewReflector(&deploymentConfigLW{factory.Client}, &deployapi.DeploymentConfig{}, queue).Run()

  store := cache.NewStore()
  cache.NewReflector(&deploymentLW{factory.Client}, &deployapi.Deployment{}, store).Run()

  return &controller.ConfigChangeController{
    DeploymentConfigInterface: factory.Client,
    NextDeploymentConfig: func() *deployapi.DeploymentConfig {
      return queue.Pop().(*deployapi.DeploymentConfig)
    },
    DeploymentStore: store,
  }
}

type ImageChangeControllerFactory struct {
  Client *osclient.Client
}

func (factory *ImageChangeControllerFactory) Create() *controller.ImageChangeController {
  queue := cache.NewFIFO()
  cache.NewReflector(&imageRepositoryLW{factory.Client}, &imageapi.ImageRepository{}, queue).Run()

  store := cache.NewStore()
  cache.NewReflector(&deploymentConfigLW{factory.Client}, &deployapi.DeploymentConfig{}, store).Run()

  return &controller.ImageChangeController{
    DeploymentConfigInterface: factory.Client,
    DeploymentConfigStore:     store,
    NextImageRepository: func() *imageapi.ImageRepository {
      return queue.Pop().(*imageapi.ImageRepository)
    },
  }
}

type deploymentLW struct {
  client osclient.Interface
}

func (lw *deploymentLW) List() (runtime.Object, error) {
  return lw.client.ListDeployments(kapi.NewContext(), labels.Everything())
}

func (lw *deploymentLW) Watch(resourceVersion uint64) (watch.Interface, error) {
  return lw.client.WatchDeployments(kapi.NewContext(), labels.Everything(), labels.Everything(), 0)
}

type deploymentConfigLW struct {
  client osclient.Interface
}

func (lw *deploymentConfigLW) List() (runtime.Object, error) {
  return lw.client.ListDeploymentConfigs(kapi.NewContext(), labels.Everything())
}

func (lw *deploymentConfigLW) Watch(resourceVersion uint64) (watch.Interface, error) {
  return lw.client.WatchDeploymentConfigs(kapi.NewContext(), labels.Everything(), labels.Everything(), 0)
}

type imageRepositoryLW struct {
  client osclient.Interface
}

func (lw *imageRepositoryLW) List() (runtime.Object, error) {
  return lw.client.ListImageRepositories(kapi.NewContext(), labels.Everything())
}

func (lw *imageRepositoryLW) Watch(resourceVersion uint64) (watch.Interface, error) {
  return lw.client.WatchImageRepositories(kapi.NewContext(), labels.Everything(), labels.Everything(), 0)
}

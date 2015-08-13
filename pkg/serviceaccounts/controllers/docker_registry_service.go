package controllers

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"
)

const DefaultOpenshiftDockerURL = "docker-registry.default.svc.cluster.local:5000"

// DockerRegistryServiceControllerOptions contains options for the DockerRegistryServiceController
type DockerRegistryServiceControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list services.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	RegistryNamespace   string
	RegistryServiceName string

	DefaultDockerURL string

	DockercfgController *DockercfgController
}

// NewDockerRegistryServiceController returns a new *DockerRegistryServiceController.
func NewDockerRegistryServiceController(cl client.Interface, options DockerRegistryServiceControllerOptions) *DockerRegistryServiceController {
	e := &DockerRegistryServiceController{
		client: cl,
	}

	_, e.serviceController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return e.client.Services(options.RegistryNamespace).List(labels.Everything())
			},
			WatchFunc: func(rv string) (watch.Interface, error) {
				return e.client.Services(options.RegistryNamespace).Watch(labels.Everything(), fields.Everything(), rv)
			},
		},
		&api.Service{},
		options.Resync,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    e.serviceAdded,
			UpdateFunc: e.serviceUpdated,
			DeleteFunc: e.serviceDeleted,
		},
	)
	e.registryServiceName = options.RegistryServiceName
	e.dockercfgController = options.DockercfgController
	e.defaultDockerURL = options.DefaultDockerURL

	return e
}

// DockerRegistryServiceController manages ServiceToken secrets for Service objects
type DockerRegistryServiceController struct {
	stopChan chan struct{}

	client client.Interface

	registryServiceName string
	defaultDockerURL    string

	dockercfgController *DockercfgController

	serviceController *framework.Controller
}

// Runs controller loops and returns immediately
func (e *DockerRegistryServiceController) Run() {
	if e.stopChan == nil {
		e.stopChan = make(chan struct{})
		go e.serviceController.Run(e.stopChan)
	}
}

// Stop gracefully shuts down this controller
func (e *DockerRegistryServiceController) Stop() {
	if e.stopChan != nil {
		close(e.stopChan)
		e.stopChan = nil
	}
}

func (e *DockerRegistryServiceController) getServiceLocation(service *api.Service) string {
	hasPortalIP := (len(service.Spec.ClusterIP) > 0) && (net.ParseIP(service.Spec.ClusterIP) != nil)
	if hasPortalIP && len(service.Spec.Ports) > 0 {
		return net.JoinHostPort(service.Spec.ClusterIP, fmt.Sprintf("%d", service.Spec.Ports[0].Port))
	}

	return e.defaultDockerURL
}

// serviceAdded reacts to the creation of a docker-registry service by updating all service account dockercfg secrets and
// changing all interestedURLs
func (e *DockerRegistryServiceController) serviceAdded(obj interface{}) {
	service := obj.(*api.Service)
	if service.Name != e.registryServiceName {
		return
	}

	if err := e.handleLocationChange(e.getServiceLocation(service)); err != nil {
		util.HandleError(err)
	}
}

// serviceUpdated reacts to the creation of a docker-registry service by updating all service account dockercfg secrets and
// changing all interestedURLs, if needed
func (e *DockerRegistryServiceController) serviceUpdated(oldObj interface{}, newObj interface{}) {
	oldService := oldObj.(*api.Service)
	newService := newObj.(*api.Service)
	if newService.Name != e.registryServiceName {
		return
	}
	if e.getServiceLocation(oldService) == e.getServiceLocation(newService) {
		return
	}

	if err := e.handleLocationChange(e.getServiceLocation(newService)); err != nil {
		util.HandleError(err)
	}
}

// serviceDeleted reacts to the docker-registry deletion by updating all the generated dockercfg secrets
func (e *DockerRegistryServiceController) serviceDeleted(obj interface{}) {
	service, ok := obj.(*api.Service)
	if !ok {
		return
	}
	if service.Name != e.registryServiceName {
		return
	}

	if err := e.handleLocationChange(e.defaultDockerURL); err != nil {
		util.HandleError(err)
	}
}

// handleLocationChange goes through all service account dockercfg secrets and updates them to point at a new docker-registry location
func (e *DockerRegistryServiceController) handleLocationChange(serviceLocation string) error {
	e.dockercfgController.SetDockerURL(serviceLocation)

	dockercfgSecrets, err := e.listDockercfgSecrets()
	if err != nil {
		return err
	}

	for _, dockercfgSecret := range dockercfgSecrets {
		dockercfg := &credentialprovider.DockerConfig{}
		if err := json.Unmarshal(dockercfgSecret.Data[api.DockerConfigKey], dockercfg); err != nil {
			util.HandleError(err)
			continue
		}

		dockercfgMap := map[string]credentialprovider.DockerConfigEntry(*dockercfg)
		keys := util.KeySet(reflect.ValueOf(dockercfgMap))
		if len(keys) != 1 {
			util.HandleError(err)
			continue
		}
		oldKey := keys.List()[0]

		// if there's no change, skip
		if oldKey == serviceLocation {
			continue
		}

		dockercfgMap[serviceLocation] = dockercfgMap[oldKey]
		delete(dockercfgMap, oldKey)
		t := credentialprovider.DockerConfig(dockercfgMap)
		dockercfg = &t

		dockercfgContent, marshalErr := json.Marshal(dockercfg)
		if marshalErr != nil {
			util.HandleError(marshalErr)
			continue
		}
		dockercfgSecret.Data[api.DockerConfigKey] = dockercfgContent

		if _, err := e.client.Secrets(dockercfgSecret.Namespace).Update(dockercfgSecret); err != nil {
			util.HandleError(err)
			continue
		}
	}

	return err
}

func (e *DockerRegistryServiceController) listDockercfgSecrets() ([]*api.Secret, error) {
	dockercfgSelector := fields.SelectorFromSet(map[string]string{client.SecretType: string(api.SecretTypeDockercfg)})
	potentialSecrets, err := e.client.Secrets(api.NamespaceAll).List(labels.Everything(), dockercfgSelector)
	if err != nil {
		return nil, err
	}

	dockercfgSecretsForThisSA := []*api.Secret{}
	for i, currSecret := range potentialSecrets.Items {
		if len(currSecret.Annotations[ServiceAccountTokenSecretNameKey]) > 0 {
			dockercfgSecretsForThisSA = append(dockercfgSecretsForThisSA, &potentialSecrets.Items[i])
		}
	}

	return dockercfgSecretsForThisSA, nil
}

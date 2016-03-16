package controllers

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"
)

// DockerRegistryServiceControllerOptions contains options for the DockerRegistryServiceController
type DockerRegistryServiceControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list services.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	RegistryNamespace   string
	RegistryServiceName string

	DockercfgController *DockercfgController
}

// NewDockerRegistryServiceController returns a new *DockerRegistryServiceController.
func NewDockerRegistryServiceController(cl client.Interface, options DockerRegistryServiceControllerOptions) *DockerRegistryServiceController {
	e := &DockerRegistryServiceController{
		client:              cl,
		dockercfgController: options.DockercfgController,
		queue:               workqueue.New(),
		serviceName:         options.RegistryServiceName,
		serviceNamespace:    options.RegistryNamespace,
	}

	e.serviceCache, e.serviceController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(opts kapi.ListOptions) (runtime.Object, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", options.RegistryServiceName)
				return e.client.Services(options.RegistryNamespace).List(opts)
			},
			WatchFunc: func(opts kapi.ListOptions) (watch.Interface, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", options.RegistryServiceName)
				return e.client.Services(options.RegistryNamespace).Watch(opts)
			},
		},
		&kapi.Service{},
		options.Resync,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := obj.(*kapi.Service)
				glog.V(4).Infof("Adding service %s", service.Name)
				e.enqueueService(service)
			},
			UpdateFunc: func(old, cur interface{}) {
				service := cur.(*kapi.Service)
				glog.V(4).Infof("Updating service %s", service.Name)
				// Resync on service object relist.
				e.enqueueService(service)
			},
			DeleteFunc: func(obj interface{}) {
				service := obj.(*kapi.Service)
				glog.V(4).Infof("Adding service %s", service.Name)
				e.enqueueService(service)
			},
		},
	)
	e.syncHandler = e.syncService
	e.servicesSynced = e.serviceController.HasSynced

	return e
}

// DockerRegistryServiceController manages ServiceToken secrets for Service objects
type DockerRegistryServiceController struct {
	client client.Interface

	serviceName      string
	serviceNamespace string

	dockercfgController *DockercfgController

	serviceController *framework.Controller
	serviceCache      cache.Store
	servicesSynced    func() bool

	queue                 *workqueue.Type
	lastRegistryLocations sets.String

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// Runs controller loops and returns immediately
func (e *DockerRegistryServiceController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	go e.serviceController.Run(stopCh)
	go e.waitForSync(stopCh)

	<-stopCh
	glog.Infof("Shutting down docker registry service controller")
	e.queue.ShutDown()
}

func (e *DockerRegistryServiceController) enqueueService(service *kapi.Service) {
	key, err := controller.KeyFunc(service)
	if err != nil {
		glog.Errorf("Couldn't get key for service %+v: %v", service, err)
		return
	}

	e.queue.Add(key)
}

// waitForSync polls to see if all the controllers are synced.  If so, it then makes a call to
// update the dockercfg secrets initially
func (e *DockerRegistryServiceController) waitForSync(stopCh <-chan struct{}) {
	// don't let panics escape
	defer utilruntime.HandleCrash()

	for {
		if e.servicesSynced() {
			// after syncing, determine the current state and assume that we're up to date for it if you don't do this,
			// you'll get an initial storm as you mess with all the dockercfg secrets every time you startup
			e.intializeServiceLocations()

			// don't start doing work until we've sync'ed
			go wait.Until(e.worker, time.Second, stopCh)
			return
		}

		select {
		case <-stopCh:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (e *DockerRegistryServiceController) intializeServiceLocations() {
	key, err := controller.KeyFunc(&kapi.Service{ObjectMeta: kapi.ObjectMeta{Name: e.serviceName, Namespace: e.serviceNamespace}})
	if err != nil {
		return
	}

	obj, exists, err := e.serviceCache.GetByKey(key)
	if err != nil {
		return
	}
	if !exists {
		return
	}
	service := obj.(*kapi.Service)

	e.lastRegistryLocations = sets.NewString(getServiceLocations(service)...)
	e.dockercfgController.SetDockerURLs(e.lastRegistryLocations.List()...)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (e *DockerRegistryServiceController) worker() {
	for {
		func() {
			key, quit := e.queue.Get()
			if quit {
				return
			}
			defer e.queue.Done(key)
			err := e.syncHandler(key.(string))
			if err != nil {
				glog.Errorf("Error syncing service: %v", err)
			}
		}()
	}
}

// getServiceLocations returns the dns form and the ip form of the secret
func getServiceLocations(service *kapi.Service) []string {
	hasPortalIP := (len(service.Spec.ClusterIP) > 0) && (net.ParseIP(service.Spec.ClusterIP) != nil)
	if hasPortalIP && len(service.Spec.Ports) > 0 {
		return []string{
			net.JoinHostPort(service.Spec.ClusterIP, fmt.Sprintf("%d", service.Spec.Ports[0].Port)),
			net.JoinHostPort(fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace), fmt.Sprintf("%d", service.Spec.Ports[0].Port)),
		}
	}

	return []string{}
}

// handleLocationChange goes through all service account dockercfg secrets and updates them to point at a new docker-registry location
func (e *DockerRegistryServiceController) syncService(key string) error {
	obj, exists, err := e.serviceCache.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve service %v from store: %v", key, err)
		e.queue.Add(key)
		return err
	}
	locationSet := sets.String{}
	if exists {
		service := obj.(*kapi.Service)
		locationSet.Insert(getServiceLocations(service)...)
	}
	// if we don't exist, then we've been deleted.  Cleanup the dockercfg secrets

	if e.lastRegistryLocations.Equal(locationSet) {
		glog.V(4).Infof("No effective update: %v", locationSet)
		return nil
	}
	// make sure that new dockercfg secrets get the correct locations
	e.dockercfgController.SetDockerURLs(locationSet.List()...)

	dockercfgSecrets, err := e.listDockercfgSecrets()
	if err != nil {
		return err
	}

	for _, dockercfgSecret := range dockercfgSecrets {
		dockercfg := &credentialprovider.DockerConfig{}
		if err := json.Unmarshal(dockercfgSecret.Data[kapi.DockerConfigKey], dockercfg); err != nil {
			utilruntime.HandleError(err)
			continue
		}

		dockercfgMap := map[string]credentialprovider.DockerConfigEntry(*dockercfg)
		existingDockercfgSecretLocations := sets.StringKeySet(dockercfgMap)
		// if the existingDockercfgSecretLocations haven't changed, don't make an update and check the next one
		if existingDockercfgSecretLocations.Equal(locationSet) {
			continue
		}

		dockerCredentials := dockercfgSecret.Annotations[ServiceAccountTokenValueAnnotation]
		if len(dockerCredentials) == 0 && len(existingDockercfgSecretLocations) > 0 {
			dockerCredentials = dockercfgMap[existingDockercfgSecretLocations.List()[0]].Password
		}
		if len(dockerCredentials) == 0 {
			tokenSecret, err := e.client.Secrets(dockercfgSecret.Namespace).Get(dockercfgSecret.Annotations[ServiceAccountTokenSecretNameKey])
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("cannot determine SA token: %v", err))
				continue
			}
			dockerCredentials = string(tokenSecret.Data[kapi.ServiceAccountTokenKey])
		}

		newDockercfgMap := map[string]credentialprovider.DockerConfigEntry{}
		for key := range locationSet {
			newDockercfgMap[key] = credentialprovider.DockerConfigEntry{
				Username: "serviceaccount",
				Password: dockerCredentials,
				Email:    "serviceaccount@example.org",
			}
		}

		t := credentialprovider.DockerConfig(newDockercfgMap)
		dockercfg = &t

		dockercfgContent, err2 := json.Marshal(dockercfg)
		if err2 != nil {
			utilruntime.HandleError(err2)
			continue
		}
		dockercfgSecret.Data[kapi.DockerConfigKey] = dockercfgContent

		if _, updateErr := e.client.Secrets(dockercfgSecret.Namespace).Update(dockercfgSecret); updateErr != nil {
			utilruntime.HandleError(updateErr)
			continue
		}
	}

	e.lastRegistryLocations = locationSet
	return nil
}

func (e *DockerRegistryServiceController) listDockercfgSecrets() ([]*kapi.Secret, error) {
	options := kapi.ListOptions{FieldSelector: fields.SelectorFromSet(map[string]string{client.SecretType: string(kapi.SecretTypeDockercfg)})}
	potentialSecrets, err := e.client.Secrets(kapi.NamespaceAll).List(options)
	if err != nil {
		return nil, err
	}

	dockercfgSecretsForThisSA := []*kapi.Secret{}
	for i, currSecret := range potentialSecrets.Items {
		// the fake clients doesn't handle filters and this isn't strictly incorrect, just unnecessary.
		if currSecret.Type != kapi.SecretTypeDockercfg {
			continue
		}
		if len(currSecret.Annotations[ServiceAccountTokenSecretNameKey]) > 0 {
			dockercfgSecretsForThisSA = append(dockercfgSecretsForThisSA, &potentialSecrets.Items[i])
		}
	}

	return dockercfgSecretsForThisSA, nil
}

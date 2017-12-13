package controllers

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

// DockerRegistryServiceControllerOptions contains options for the DockerRegistryServiceController
type DockerRegistryServiceControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list services.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	RegistryNamespace   string
	RegistryServiceName string

	DockercfgController *DockercfgController

	// DockerURLsInitialized is used to send a signal to the DockercfgController that it has the correct set of docker urls
	DockerURLsInitialized chan struct{}
}

// NewDockerRegistryServiceController returns a new *DockerRegistryServiceController.
func NewDockerRegistryServiceController(secrets informers.SecretInformer, cl kclientset.Interface, options DockerRegistryServiceControllerOptions) *DockerRegistryServiceController {
	e := &DockerRegistryServiceController{
		client:                cl,
		dockercfgController:   options.DockercfgController,
		registryLocationQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		secretsToUpdate:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		serviceName:           options.RegistryServiceName,
		serviceNamespace:      options.RegistryNamespace,
		dockerURLsInitialized: options.DockerURLsInitialized,
	}

	// does not use shared informers because we're only watching one item
	e.serviceCache, e.serviceController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", options.RegistryServiceName).String()
				return e.client.Core().Services(options.RegistryNamespace).List(opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", options.RegistryServiceName).String()
				return e.client.Core().Services(options.RegistryNamespace).Watch(opts)
			},
		},
		&v1.Service{},
		options.Resync,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				e.enqueueRegistryLocationQueue()
			},
			UpdateFunc: func(old, cur interface{}) {
				e.enqueueRegistryLocationQueue()
			},
			DeleteFunc: func(obj interface{}) {
				e.enqueueRegistryLocationQueue()
			},
		},
	)
	e.servicesSynced = e.serviceController.HasSynced
	e.syncRegistryLocationHandler = e.syncRegistryLocationChange

	e.secretCache = secrets.Informer().GetIndexer()
	e.secretsSynced = secrets.Informer().GetController().HasSynced
	e.syncSecretHandler = e.syncSecretUpdate

	return e
}

// DockerRegistryServiceController manages ServiceToken secrets for Service objects
type DockerRegistryServiceController struct {
	client kclientset.Interface

	serviceName      string
	serviceNamespace string

	dockercfgController *DockercfgController

	serviceController           cache.Controller
	serviceCache                cache.Store
	servicesSynced              func() bool
	syncRegistryLocationHandler func(key string) error

	secretCache       cache.Store
	secretsSynced     func() bool
	syncSecretHandler func(key string) error

	registryURLs          sets.String
	registryURLLock       sync.RWMutex
	registryLocationQueue workqueue.RateLimitingInterface
	secretsToUpdate       workqueue.RateLimitingInterface

	dockerURLsInitialized chan struct{}
}

// Runs controller loops and returns immediately
func (e *DockerRegistryServiceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer e.registryLocationQueue.ShutDown()

	go e.serviceController.Run(stopCh)

	// Wait for the store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go e.waitForDockerURLs(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	glog.V(5).Infof("Starting workers")
	go wait.Until(e.watchForDockerURLChanges, time.Second, stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(e.watchForDockercfgSecretUpdates, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

// enqueue adds to our queue.  We only have one entry, but we never have to check it since we already know the things
// we're watching for.
func (e *DockerRegistryServiceController) enqueueRegistryLocationQueue() {
	e.registryLocationQueue.Add("check")
}

// waitForDockerURLs waits until all information required for fully determining the set of the internal docker registry
// hostnames and IPs are complete before continuing
// Once that work is done, the dockerconfig controller will be released to do work.
func (e *DockerRegistryServiceController) waitForDockerURLs(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, e.servicesSynced, e.secretsSynced) {
		return
	}

	// after syncing, determine the current state and assume that we're up to date for it if you don't do this,
	// you'll get an initial storm as you mess with all the dockercfg secrets every time you startup
	urls := e.getDockerRegistryLocations()
	e.setRegistryURLs(urls...)
	e.dockercfgController.SetDockerURLs(urls...)
	close(e.dockerURLsInitialized)
	close(ready)

	return
}

func (e *DockerRegistryServiceController) setRegistryURLs(registryURLs ...string) {
	e.registryURLLock.Lock()
	defer e.registryURLLock.Unlock()
	e.registryURLs = sets.NewString(registryURLs...)
}

func (e *DockerRegistryServiceController) getRegistryURLs() sets.String {
	e.registryURLLock.RLock()
	defer e.registryURLLock.RUnlock()
	// return a copy to avoid any concurrent modification issues
	return sets.NewString(e.registryURLs.List()...)
}

// watchForDockerURLChanges runs a worker thread that just dequeues and processes items related to a docker URL change
func (e *DockerRegistryServiceController) watchForDockerURLChanges() {
	workFn := func() bool {
		key, quit := e.registryLocationQueue.Get()
		if quit {
			return true
		}
		defer e.registryLocationQueue.Done(key)

		if err := e.syncRegistryLocationHandler(key.(string)); err == nil {
			// this means the request was successfully handled.  We should "forget" the item so that any retry
			// later on is reset
			e.registryLocationQueue.Forget(key)

		} else {
			// if we had an error it means that we didn't handle it, which means that we want to requeue the work
			utilruntime.HandleError(fmt.Errorf("error syncing service, it will be retried: %v", err))
			e.registryLocationQueue.AddRateLimited(key)
		}

		return false
	}

	for {
		if workFn() {
			return
		}
	}
}

// getDockerRegistryLocations returns the dns form and the ip form of the secret
func (e *DockerRegistryServiceController) getDockerRegistryLocations() []string {
	key, err := controller.KeyFunc(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: e.serviceName, Namespace: e.serviceNamespace}})
	if err != nil {
		return []string{}
	}

	obj, exists, err := e.serviceCache.GetByKey(key)
	if err != nil {
		return []string{}
	}
	if !exists {
		return []string{}
	}
	service := obj.(*v1.Service)

	hasClusterIP := (len(service.Spec.ClusterIP) > 0) && (net.ParseIP(service.Spec.ClusterIP) != nil)
	if hasClusterIP && len(service.Spec.Ports) > 0 {
		return []string{
			net.JoinHostPort(service.Spec.ClusterIP, fmt.Sprintf("%d", service.Spec.Ports[0].Port)),
			net.JoinHostPort(fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace), fmt.Sprintf("%d", service.Spec.Ports[0].Port)),
		}
	}

	return []string{}
}

// syncRegistryLocationChange goes through all service account dockercfg secrets and updates them to point at a new docker-registry location
func (e *DockerRegistryServiceController) syncRegistryLocationChange(key string) error {
	newDockerRegistryLocations := sets.NewString(e.getDockerRegistryLocations()...)
	if e.getRegistryURLs().Equal(newDockerRegistryLocations) {
		glog.V(4).Infof("No effective update: %v", newDockerRegistryLocations)
		return nil
	}

	// make sure that new dockercfg secrets get the correct locations
	e.dockercfgController.SetDockerURLs(newDockerRegistryLocations.List()...)
	e.setRegistryURLs(newDockerRegistryLocations.List()...)

	// we've changed the docker registry URL.  Add items to the work queue for all known secrets
	// new secrets will already get the updated value.
	for _, obj := range e.secretCache.List() {
		switch t := obj.(type) {
		case *v1.Secret:
			if t.Type != v1.SecretTypeDockercfg {
				continue
			}
		default:
			utilruntime.HandleError(fmt.Errorf("object passed to %T that is not expected: %T", e, obj))
			continue
		}
		key, err := controller.KeyFunc(obj)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
			continue
		}
		e.secretsToUpdate.Add(key)
	}

	return nil
}

// watchForDockercfgSecretUpdates watches the work queue for entries that indicate that it should modify dockercfg secrets with new
// docker registry URLs
func (e *DockerRegistryServiceController) watchForDockercfgSecretUpdates() {
	workFn := func() bool {
		key, quit := e.secretsToUpdate.Get()
		if quit {
			return true
		}
		defer e.secretsToUpdate.Done(key)

		if err := e.syncSecretHandler(key.(string)); err == nil {
			// this means the request was successfully handled.  We should "forget" the item so that any retry
			// later on is reset
			e.secretsToUpdate.Forget(key)

		} else {
			// if we had an error it means that we didn't handle it, which means that we want to requeue the work
			utilruntime.HandleError(fmt.Errorf("error syncing service, it will be retried: %v", err))
			e.secretsToUpdate.AddRateLimited(key)
		}

		return false
	}

	for {
		if workFn() {
			return
		}
	}
}

func (e *DockerRegistryServiceController) syncSecretUpdate(key string) error {
	obj, exists, err := e.secretCache.GetByKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to retrieve secret %v from store: %v", key, err))
		return err
	}
	if !exists {
		return nil
	}

	dockerRegistryURLs := e.getRegistryURLs()
	sharedDockercfgSecret := obj.(*v1.Secret)

	dockercfg := &credentialprovider.DockerConfig{}
	// an error here doesn't matter.  If we can't deserialize this, we'll replace it with one that works.
	json.Unmarshal(sharedDockercfgSecret.Data[v1.DockerConfigKey], dockercfg)

	dockercfgMap := map[string]credentialprovider.DockerConfigEntry(*dockercfg)
	existingDockercfgSecretLocations := sets.StringKeySet(dockercfgMap)
	// if the existingDockercfgSecretLocations haven't changed, don't make an update and check the next one
	if existingDockercfgSecretLocations.Equal(dockerRegistryURLs) {
		return nil
	}

	// we need to update it, make a copy
	dockercfgSecret := obj.(runtime.Object).DeepCopyObject().(*v1.Secret)

	dockerCredentials := dockercfgSecret.Annotations[ServiceAccountTokenValueAnnotation]
	if len(dockerCredentials) == 0 && len(existingDockercfgSecretLocations) > 0 {
		dockerCredentials = dockercfgMap[existingDockercfgSecretLocations.List()[0]].Password
	}
	if len(dockerCredentials) == 0 {
		tokenSecretKey := dockercfgSecret.Namespace + "/" + dockercfgSecret.Annotations[ServiceAccountTokenSecretNameKey]
		tokenSecret, exists, err := e.secretCache.GetByKey(tokenSecretKey)
		if !exists {
			utilruntime.HandleError(fmt.Errorf("cannot determine SA token due to missing secret: %v", tokenSecretKey))
			return nil
		}
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("cannot determine SA token: %v", err))
			return nil
		}
		dockerCredentials = string(tokenSecret.(*v1.Secret).Data[v1.ServiceAccountTokenKey])
	}

	newDockercfgMap := credentialprovider.DockerConfig{}
	for key := range dockerRegistryURLs {
		newDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: dockerCredentials,
			Email:    "serviceaccount@example.org",
		}
	}

	dockercfgContent, err := json.Marshal(&newDockercfgMap)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}
	dockercfgSecret.Data[v1.DockerConfigKey] = dockercfgContent

	if _, err := e.client.Core().Secrets(dockercfgSecret.Namespace).Update(dockercfgSecret); err != nil {
		return err
	}

	return nil
}

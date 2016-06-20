package controllers

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/registry/secret"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"

	osautil "github.com/openshift/origin/pkg/serviceaccounts/util"
)

const (
	ServiceAccountTokenSecretNameKey = "openshift.io/token-secret.name"
	MaxRetriesBeforeResync           = 5
)

// DockercfgControllerOptions contains options for the DockercfgController
type DockercfgControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list service accounts.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	DefaultDockerURL string
}

// NewDockercfgController returns a new *DockercfgController.
func NewDockercfgController(cl client.Interface, options DockercfgControllerOptions) *DockercfgController {
	e := &DockercfgController{
		client: cl,
		queue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	var serviceAccountCache cache.Store
	serviceAccountCache, e.serviceAccountController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return e.client.ServiceAccounts(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return e.client.ServiceAccounts(api.NamespaceAll).Watch(options)
			},
		},
		&api.ServiceAccount{},
		options.Resync,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				serviceAccount := obj.(*api.ServiceAccount)
				glog.V(5).Infof("Adding service account %s", serviceAccount.Name)
				e.enqueueServiceAccount(serviceAccount)
			},
			UpdateFunc: func(old, cur interface{}) {
				serviceAccount := cur.(*api.ServiceAccount)
				glog.V(5).Infof("Updating service account %s", serviceAccount.Name)
				// Resync on service object relist.
				e.enqueueServiceAccount(serviceAccount)
			},
		},
	)

	e.serviceAccountCache = NewEtcdMutationCache(serviceAccountCache)
	e.syncHandler = e.syncServiceAccount
	e.dockerURL = options.DefaultDockerURL

	return e
}

// DockercfgController manages dockercfg secrets for ServiceAccount objects
type DockercfgController struct {
	client client.Interface

	dockerURL     string
	dockerURLLock sync.Mutex

	serviceAccountCache      MutationCache
	serviceAccountController *framework.Controller

	queue workqueue.RateLimitingInterface

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

func (e *DockercfgController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go e.serviceAccountController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(e.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down dockercfg secret controller")
	e.queue.ShutDown()
}

func (e *DockercfgController) enqueueServiceAccount(serviceAccount *api.ServiceAccount) {
	if !needsDockercfgSecret(serviceAccount) {
		return
	}

	key, err := controller.KeyFunc(serviceAccount)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", serviceAccount, err)
		return
	}

	e.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (e *DockercfgController) worker() {
	for {
		if !e.worker_inner() {
			return
		}
	}
}

// worker_inner returns true if the worker thread should continue
func (e *DockercfgController) worker_inner() bool {
	key, quit := e.queue.Get()
	if quit {
		return false
	}
	defer e.queue.Done(key)

	if err := e.syncHandler(key.(string)); err == nil {
		// this means the request was successfully handled.  We should "forget" the item so that any retry
		// later on is reset
		e.queue.Forget(key)

	} else {
		// if we had an error it means that we didn't handle it, which means that we want to requeue the work
		if e.queue.NumRequeues(key) > MaxRetriesBeforeResync {
			utilruntime.HandleError(fmt.Errorf("error syncing service, it will be tried again on a resync %v: %v", key, err))
			e.queue.Forget(key)
		} else {
			glog.V(4).Infof("error syncing service, it will be retried %v: %v", key, err)
			e.queue.AddRateLimited(key)
		}
	}

	return true
}

func (e *DockercfgController) SetDockerURL(newDockerURL string) {
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	e.dockerURL = newDockerURL
}

func needsDockercfgSecret(serviceAccount *api.ServiceAccount) bool {
	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(serviceAccount)

	// look for an ImagePullSecret in the form
	if len(imageDockercfgPullSecrets) > 0 && len(mountableDockercfgSecrets) > 0 {
		return false
	}

	return true
}

func (e *DockercfgController) syncServiceAccount(key string) error {
	obj, exists, err := e.serviceAccountCache.GetByKey(key)
	if err != nil {
		glog.V(4).Infof("Unable to retrieve service account %v from store: %v", key, err)
		return err
	}
	if !exists {
		glog.V(4).Infof("Service account has been deleted %v", key)
		return nil
	}
	if !needsDockercfgSecret(obj.(*api.ServiceAccount)) {
		return nil
	}

	uncastSA, err := api.Scheme.DeepCopy(obj)
	if err != nil {
		return err
	}
	serviceAccount := uncastSA.(*api.ServiceAccount)

	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(serviceAccount)

	// If we have a pull secret in one list, use it for the other.  It must only be in one list because
	// otherwise we wouldn't "needsDockercfgSecret"
	foundPullSecret := len(imageDockercfgPullSecrets) > 0
	foundMountableSecret := len(mountableDockercfgSecrets) > 0
	if foundPullSecret || foundMountableSecret {
		switch {
		case foundPullSecret:
			serviceAccount.Secrets = append(serviceAccount.Secrets, api.ObjectReference{Name: imageDockercfgPullSecrets.List()[0]})
		case foundMountableSecret:
			serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, api.LocalObjectReference{Name: mountableDockercfgSecrets.List()[0]})
		}

		updatedSA, err := e.client.ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
		if err == nil {
			e.serviceAccountCache.Mutation(updatedSA)
		}
		return err
	}

	dockercfgSecret, err := e.createDockerPullSecret(serviceAccount)
	if err != nil {
		return err
	}

	first := true
	err = client.RetryOnConflict(client.DefaultBackoff, func() error {
		if !first {
			obj, exists, err := e.serviceAccountCache.GetByKey(key)
			if err != nil {
				return err
			}
			if !exists || !needsDockercfgSecret(obj.(*api.ServiceAccount)) || serviceAccount.UID != obj.(*api.ServiceAccount).UID {
				// somehow a dockercfg secret appeared or the SA disappeared.  cleanup the secret we made and return
				glog.V(2).Infof("Deleting secret because the work is already done %s/%s", dockercfgSecret.Namespace, dockercfgSecret.Name)
				e.client.Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name)
				return nil
			}

			uncastSA, err := api.Scheme.DeepCopy(obj)
			if err != nil {
				return err
			}
			serviceAccount = uncastSA.(*api.ServiceAccount)
		}
		first = false

		serviceAccount.Secrets = append(serviceAccount.Secrets, api.ObjectReference{Name: dockercfgSecret.Name})
		serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, api.LocalObjectReference{Name: dockercfgSecret.Name})

		updatedSA, err := e.client.ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
		if err == nil {
			e.serviceAccountCache.Mutation(updatedSA)
		}
		return err
	})

	if err != nil {
		// nothing to do.  Our choice was stale or we got a conflict.  Either way that means that the service account was updated.  We simply need to return because we'll get an update notification later
		// we do need to clean up our dockercfgSecret.  token secrets are cleaned up by the controller handling service account dockercfg secret deletes
		glog.V(2).Infof("Deleting secret %s/%s (err=%v)", dockercfgSecret.Namespace, dockercfgSecret.Name, err)
		e.client.Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name)
	}
	return err
}

const (
	tokenSecretWaitInterval = 20 * time.Millisecond
	tokenSecretWaitTimes    = 100
)

// createTokenSecret creates a token secret for a given service account.  Returns the name of the token
func (e *DockercfgController) createTokenSecret(serviceAccount *api.ServiceAccount) (*api.Secret, error) {
	tokenSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      secret.Strategy.GenerateName(osautil.GetTokenSecretNamePrefix(serviceAccount)),
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				api.ServiceAccountNameKey: serviceAccount.Name,
				api.ServiceAccountUIDKey:  string(serviceAccount.UID),
			},
		},
		Type: api.SecretTypeServiceAccountToken,
		Data: map[string][]byte{},
	}

	_, err := e.client.Secrets(tokenSecret.Namespace).Create(tokenSecret)
	if err != nil {
		return nil, err
	}

	// now we have to wait for the service account token controller to make this valid
	// TODO remove this once we have a create-token endpoint
	for i := 0; i <= tokenSecretWaitTimes; i++ {
		liveTokenSecret, err2 := e.client.Secrets(tokenSecret.Namespace).Get(tokenSecret.Name)
		if err2 != nil {
			return nil, err2
		}

		if len(liveTokenSecret.Data[api.ServiceAccountTokenKey]) > 0 {
			return liveTokenSecret, nil
		}

		time.Sleep(wait.Jitter(tokenSecretWaitInterval, 0.0))

	}

	// the token wasn't ever created, attempt deletion
	glog.Warningf("Deleting unfilled token secret %s/%s", tokenSecret.Namespace, tokenSecret.Name)
	if deleteErr := e.client.Secrets(tokenSecret.Namespace).Delete(tokenSecret.Name); (deleteErr != nil) && !kapierrors.IsNotFound(deleteErr) {
		utilruntime.HandleError(deleteErr)
	}
	return nil, fmt.Errorf("token never generated for %s", tokenSecret.Name)
}

// createDockerPullSecret creates a dockercfg secret based on the token secret
func (e *DockercfgController) createDockerPullSecret(serviceAccount *api.ServiceAccount) (*api.Secret, error) {
	glog.V(4).Infof("Creating secret for %s/%s", serviceAccount.Namespace, serviceAccount.Name)

	tokenSecret, err := e.createTokenSecret(serviceAccount)
	if err != nil {
		return nil, err
	}

	dockercfgSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      secret.Strategy.GenerateName(osautil.GetDockercfgSecretNamePrefix(serviceAccount)),
			Namespace: tokenSecret.Namespace,
			Annotations: map[string]string{
				api.ServiceAccountNameKey:        serviceAccount.Name,
				api.ServiceAccountUIDKey:         string(serviceAccount.UID),
				ServiceAccountTokenSecretNameKey: string(tokenSecret.Name),
			},
		},
		Type: api.SecretTypeDockercfg,
		Data: map[string][]byte{},
	}

	// prevent updating the DockerURL until we've created the secret
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	dockercfg := &credentialprovider.DockerConfig{
		e.dockerURL: credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: string(tokenSecret.Data[api.ServiceAccountTokenKey]),
			Email:    "serviceaccount@example.org",
		},
	}
	dockercfgContent, err := json.Marshal(dockercfg)
	if err != nil {
		return nil, err
	}
	dockercfgSecret.Data[api.DockerConfigKey] = dockercfgContent

	// Save the secret
	createdSecret, err := e.client.Secrets(tokenSecret.Namespace).Create(dockercfgSecret)

	return createdSecret, err
}

func getSecretReferences(serviceAccount *api.ServiceAccount) sets.String {
	references := sets.NewString()
	for _, secret := range serviceAccount.Secrets {
		references.Insert(secret.Name)
	}
	return references
}

func getGeneratedDockercfgSecretNames(serviceAccount *api.ServiceAccount) (sets.String, sets.String) {
	mountableDockercfgSecrets := sets.String{}
	imageDockercfgPullSecrets := sets.String{}

	secretNamePrefix := osautil.GetDockercfgSecretNamePrefix(serviceAccount)

	for _, s := range serviceAccount.Secrets {
		if strings.HasPrefix(s.Name, secretNamePrefix) {
			mountableDockercfgSecrets.Insert(s.Name)
		}
	}
	for _, s := range serviceAccount.ImagePullSecrets {
		if strings.HasPrefix(s.Name, secretNamePrefix) {
			imageDockercfgPullSecrets.Insert(s.Name)
		}
	}
	return mountableDockercfgSecrets, imageDockercfgPullSecrets
}

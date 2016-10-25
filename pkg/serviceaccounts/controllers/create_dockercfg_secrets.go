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
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/secret"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
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

	// ServiceAccountTokenValueAnnotation stores the actual value of the token so that a dockercfg secret can be
	// made without having a value dockerURL
	ServiceAccountTokenValueAnnotation = "openshift.io/token-secret.value"

	// CreateDockercfgSecretsController is the name of this controller that should be
	// attached to all token secrets this controller create
	CreateDockercfgSecretsController = "openshift.io/create-dockercfg-secrets"

	// PendingTokenAnnotation contains the name of the token secret that is waiting for the
	// token data population
	PendingTokenAnnotation = "openshift.io/create-dockercfg-secrets.pending-token"
)

// DockercfgControllerOptions contains options for the DockercfgController
type DockercfgControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list service accounts.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	// DockerURLsIntialized is used to send a signal to this controller that it has the correct set of docker urls
	// This is normally signaled from the DockerRegistryServiceController which watches for updates to the internal
	// docker registry service.
	DockerURLsIntialized chan struct{}
}

// NewDockercfgController returns a new *DockercfgController.
func NewDockercfgController(cl client.Interface, options DockercfgControllerOptions) *DockercfgController {
	e := &DockercfgController{
		client:               cl,
		queue:                workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		dockerURLsIntialized: options.DockerURLsIntialized,
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

	tokenSecretSelector := fields.OneTermEqualSelector(api.SecretTypeField, string(api.SecretTypeServiceAccountToken))
	e.secretCache, e.secretController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				options.FieldSelector = tokenSecretSelector
				return e.client.Secrets(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				options.FieldSelector = tokenSecretSelector
				return e.client.Secrets(api.NamespaceAll).Watch(options)
			},
		},
		&api.Secret{},
		options.Resync,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    func(cur interface{}) { e.handleTokenSecretUpdate(nil, cur) },
			UpdateFunc: func(old, cur interface{}) { e.handleTokenSecretUpdate(old, cur) },
			DeleteFunc: e.handleTokenSecretDelete,
		},
	)

	e.syncHandler = e.syncServiceAccount

	return e
}

// DockercfgController manages dockercfg secrets for ServiceAccount objects
type DockercfgController struct {
	client client.Interface

	dockerURLLock        sync.Mutex
	dockerURLs           []string
	dockerURLsIntialized chan struct{}

	serviceAccountCache      MutationCache
	serviceAccountController *framework.Controller
	secretCache              cache.Store
	secretController         *framework.Controller

	queue workqueue.RateLimitingInterface

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// handleTokenSecretUpdate checks if the service account token secret is populated with
// token data and triggers re-sync of service account when the data are observed.
func (e *DockercfgController) handleTokenSecretUpdate(oldObj, newObj interface{}) {
	secret := newObj.(*api.Secret)
	if secret.Annotations[api.CreatedByAnnotation] != CreateDockercfgSecretsController {
		return
	}
	isPopulated := len(secret.Data[api.ServiceAccountTokenKey]) > 0

	wasPopulated := false
	if oldObj != nil {
		oldSecret := oldObj.(*api.Secret)
		wasPopulated = len(oldSecret.Data[api.ServiceAccountTokenKey]) > 0
		glog.V(5).Infof("Updating token secret %s/%s", secret.Namespace, secret.Name)
	} else {
		glog.V(5).Infof("Adding token secret %s/%s", secret.Namespace, secret.Name)
	}

	if !wasPopulated && isPopulated {
		e.enqueueServiceAccountForToken(secret)
	}
}

// handleTokenSecretDelete handles token secrets deletion and re-sync the service account
// which will cause a token to be re-created.
func (e *DockercfgController) handleTokenSecretDelete(obj interface{}) {
	secret, isSecret := obj.(*api.Secret)
	if !isSecret {
		tombstone, objIsTombstone := obj.(cache.DeletedFinalStateUnknown)
		if !objIsTombstone {
			glog.V(2).Infof("Expected tombstone object when deleting token, got %v", obj)
			return
		}
		secret, isSecret = tombstone.Obj.(*api.Secret)
		if !isSecret {
			glog.V(2).Infof("Expected tombstone object to contain secret, got: %v", obj)
			return
		}
	}
	if secret.Annotations[api.CreatedByAnnotation] != CreateDockercfgSecretsController {
		return
	}
	if len(secret.Data[api.ServiceAccountTokenKey]) > 0 {
		// Let deleted_token_secrets handle deletion of populated tokens
		return
	}
	e.enqueueServiceAccountForToken(secret)
}

func (e *DockercfgController) enqueueServiceAccountForToken(tokenSecret *api.Secret) {
	serviceAccount := &api.ServiceAccount{
		ObjectMeta: api.ObjectMeta{
			Name:      tokenSecret.Annotations[api.ServiceAccountNameKey],
			Namespace: tokenSecret.Namespace,
			UID:       types.UID(tokenSecret.Annotations[api.ServiceAccountUIDKey]),
		},
	}
	key, err := controller.KeyFunc(serviceAccount)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing token secret %s/%s: %v", tokenSecret.Namespace, tokenSecret.Name, err))
		return
	}
	e.queue.Add(key)
}

func (e *DockercfgController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go e.waitForDockerURLs(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}
	glog.Infof("Dockercfg secret controller initialized, starting.")

	go e.serviceAccountController.Run(stopCh)
	go e.secretController.Run(stopCh)
	for !e.serviceAccountController.HasSynced() || !e.secretController.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < workers; i++ {
		go wait.Until(e.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down dockercfg secret controller")
	e.queue.ShutDown()
}

func (c *DockercfgController) waitForDockerURLs(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// wait for the initialization to complete to be informed of a stop
	select {
	case <-c.dockerURLsIntialized:
	case <-stopCh:
		return
	}

	close(ready)
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
		if !e.work() {
			return
		}
	}
}

// work returns true if the worker thread should continue
func (e *DockercfgController) work() bool {
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

func (e *DockercfgController) SetDockerURLs(newDockerURLs ...string) {
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	e.dockerURLs = newDockerURLs
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
		// Clear the pending token annotation when updating
		delete(serviceAccount.Annotations, PendingTokenAnnotation)

		updatedSA, err := e.client.ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
		if err == nil {
			e.serviceAccountCache.Mutation(updatedSA)
		}
		return err
	}

	dockercfgSecret, created, err := e.createDockerPullSecret(serviceAccount)
	if err != nil {
		return err
	}
	if !created {
		glog.V(5).Infof("The dockercfg secret was not created for service account %s/%s, will retry", serviceAccount.Namespace, serviceAccount.Name)
		return nil
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
		// Clear the pending token annotation when updating
		delete(serviceAccount.Annotations, PendingTokenAnnotation)

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

// createTokenSecret creates a token secret for a given service account.  Returns the name of the token
func (e *DockercfgController) createTokenSecret(serviceAccount *api.ServiceAccount) (*api.Secret, bool, error) {
	pendingTokenName := serviceAccount.Annotations[PendingTokenAnnotation]

	// If this service account has no record of a pending token name, record one
	if len(pendingTokenName) == 0 {
		pendingTokenName = secret.Strategy.GenerateName(osautil.GetTokenSecretNamePrefix(serviceAccount))
		if serviceAccount.Annotations == nil {
			serviceAccount.Annotations = map[string]string{}
		}
		serviceAccount.Annotations[PendingTokenAnnotation] = pendingTokenName
		updatedServiceAccount, err := e.client.ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
		// Conflicts mean we'll get called to sync this service account again
		if kapierrors.IsConflict(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		serviceAccount = updatedServiceAccount
	}

	// Return the token from cache
	existingTokenSecretObj, exists, err := e.secretCache.GetByKey(serviceAccount.Namespace + "/" + pendingTokenName)
	if err != nil {
		return nil, false, err
	}
	if exists {
		existingTokenSecret := existingTokenSecretObj.(*api.Secret)
		return existingTokenSecret, len(existingTokenSecret.Data[api.ServiceAccountTokenKey]) > 0, nil
	}

	// Try to create the named pending token
	tokenSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      pendingTokenName,
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				api.ServiceAccountNameKey: serviceAccount.Name,
				api.ServiceAccountUIDKey:  string(serviceAccount.UID),
				api.CreatedByAnnotation:   CreateDockercfgSecretsController,
			},
		},
		Type: api.SecretTypeServiceAccountToken,
		Data: map[string][]byte{},
	}

	glog.V(4).Infof("Creating token secret %q for service account %s/%s", tokenSecret.Name, serviceAccount.Namespace, serviceAccount.Name)
	token, err := e.client.Secrets(tokenSecret.Namespace).Create(tokenSecret)
	// Already exists but not in cache means we'll get an add watch event and resync
	if kapierrors.IsAlreadyExists(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return token, len(token.Data[api.ServiceAccountTokenKey]) > 0, nil
}

// createDockerPullSecret creates a dockercfg secret based on the token secret
func (e *DockercfgController) createDockerPullSecret(serviceAccount *api.ServiceAccount) (*api.Secret, bool, error) {
	tokenSecret, isPopulated, err := e.createTokenSecret(serviceAccount)
	if err != nil {
		return nil, false, err
	}
	if !isPopulated {
		glog.V(5).Infof("Token secret for service account %s/%s is not populated yet", serviceAccount.Namespace, serviceAccount.Name)
		return nil, false, nil
	}

	dockercfgSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      secret.Strategy.GenerateName(osautil.GetDockercfgSecretNamePrefix(serviceAccount)),
			Namespace: tokenSecret.Namespace,
			Annotations: map[string]string{
				api.ServiceAccountNameKey:          serviceAccount.Name,
				api.ServiceAccountUIDKey:           string(serviceAccount.UID),
				ServiceAccountTokenSecretNameKey:   string(tokenSecret.Name),
				ServiceAccountTokenValueAnnotation: string(tokenSecret.Data[api.ServiceAccountTokenKey]),
			},
		},
		Type: api.SecretTypeDockercfg,
		Data: map[string][]byte{},
	}
	glog.V(4).Infof("Creating dockercfg secret %q for service account %s/%s", dockercfgSecret.Name, serviceAccount.Namespace, serviceAccount.Name)

	// prevent updating the DockerURL until we've created the secret
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	dockercfg := credentialprovider.DockerConfig{}
	for _, dockerURL := range e.dockerURLs {
		dockercfg[dockerURL] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: string(tokenSecret.Data[api.ServiceAccountTokenKey]),
			Email:    "serviceaccount@example.org",
		}
	}
	dockercfgContent, err := json.Marshal(&dockercfg)
	if err != nil {
		return nil, false, err
	}
	dockercfgSecret.Data[api.DockerConfigKey] = dockercfgContent

	// Save the secret
	createdSecret, err := e.client.Secrets(tokenSecret.Namespace).Create(dockercfgSecret)
	return createdSecret, err == nil, err
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

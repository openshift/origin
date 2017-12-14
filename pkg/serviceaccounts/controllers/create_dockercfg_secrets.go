package controllers

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/registry/core/secret"

	oapi "github.com/openshift/origin/pkg/api"
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

	// DockerURLsInitialized is used to send a signal to this controller that it has the correct set of docker urls
	// This is normally signaled from the DockerRegistryServiceController which watches for updates to the internal
	// docker registry service.
	DockerURLsInitialized chan struct{}
}

// NewDockercfgController returns a new *DockercfgController.
func NewDockercfgController(serviceAccounts informers.ServiceAccountInformer, secrets informers.SecretInformer, cl kclientset.Interface, options DockercfgControllerOptions) *DockercfgController {
	e := &DockercfgController{
		client: cl,
		queue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		dockerURLsInitialized: options.DockerURLsInitialized,
	}

	serviceAccountCache := serviceAccounts.Informer().GetStore()
	e.serviceAccountController = serviceAccounts.Informer().GetController()
	serviceAccounts.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				serviceAccount := obj.(*v1.ServiceAccount)
				glog.V(5).Infof("Adding service account %s", serviceAccount.Name)
				e.enqueueServiceAccount(serviceAccount)
			},
			UpdateFunc: func(old, cur interface{}) {
				serviceAccount := cur.(*v1.ServiceAccount)
				glog.V(5).Infof("Updating service account %s", serviceAccount.Name)
				// Resync on service object relist.
				e.enqueueServiceAccount(serviceAccount)
			},
		},
		options.Resync,
	)
	e.serviceAccountCache = NewEtcdMutationCache(serviceAccountCache)

	e.secretCache = secrets.Informer().GetIndexer()
	e.secretController = secrets.Informer().GetController()
	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Secret:
					return t.Type == v1.SecretTypeServiceAccountToken
				default:
					utilruntime.HandleError(fmt.Errorf("object passed to %T that is not expected: %T", e, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    func(cur interface{}) { e.handleTokenSecretUpdate(nil, cur) },
				UpdateFunc: func(old, cur interface{}) { e.handleTokenSecretUpdate(old, cur) },
				DeleteFunc: e.handleTokenSecretDelete,
			},
		},
		options.Resync,
	)
	e.syncHandler = e.syncServiceAccount

	return e
}

// DockercfgController manages dockercfg secrets for ServiceAccount objects
type DockercfgController struct {
	client kclientset.Interface

	dockerURLLock         sync.Mutex
	dockerURLs            []string
	dockerURLsInitialized chan struct{}

	serviceAccountCache      MutationCache
	serviceAccountController cache.Controller
	secretCache              cache.Store
	secretController         cache.Controller

	queue workqueue.RateLimitingInterface

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// handleTokenSecretUpdate checks if the service account token secret is populated with
// token data and triggers re-sync of service account when the data are observed.
func (e *DockercfgController) handleTokenSecretUpdate(oldObj, newObj interface{}) {
	secret := newObj.(*v1.Secret)
	if secret.Annotations[oapi.DeprecatedKubeCreatedByAnnotation] != CreateDockercfgSecretsController {
		return
	}
	isPopulated := len(secret.Data[v1.ServiceAccountTokenKey]) > 0

	wasPopulated := false
	if oldObj != nil {
		oldSecret := oldObj.(*v1.Secret)
		wasPopulated = len(oldSecret.Data[v1.ServiceAccountTokenKey]) > 0
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
	secret, isSecret := obj.(*v1.Secret)
	if !isSecret {
		tombstone, objIsTombstone := obj.(cache.DeletedFinalStateUnknown)
		if !objIsTombstone {
			glog.V(2).Infof("Expected tombstone object when deleting token, got %v", obj)
			return
		}
		secret, isSecret = tombstone.Obj.(*v1.Secret)
		if !isSecret {
			glog.V(2).Infof("Expected tombstone object to contain secret, got: %v", obj)
			return
		}
	}
	if secret.Annotations[oapi.DeprecatedKubeCreatedByAnnotation] != CreateDockercfgSecretsController {
		return
	}
	if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
		// Let deleted_token_secrets handle deletion of populated tokens
		return
	}
	e.enqueueServiceAccountForToken(secret)
}

func (e *DockercfgController) enqueueServiceAccountForToken(tokenSecret *v1.Secret) {
	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenSecret.Annotations[v1.ServiceAccountNameKey],
			Namespace: tokenSecret.Namespace,
			UID:       types.UID(tokenSecret.Annotations[v1.ServiceAccountUIDKey]),
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
	defer e.queue.ShutDown()

	// Wait for the store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go e.waitForDockerURLs(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, e.serviceAccountController.HasSynced, e.secretController.HasSynced) {
		return
	}

	glog.V(5).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(e.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

func (c *DockercfgController) waitForDockerURLs(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// wait for the initialization to complete to be informed of a stop
	select {
	case <-c.dockerURLsInitialized:
	case <-stopCh:
		return
	}

	close(ready)
}

func (e *DockercfgController) enqueueServiceAccount(serviceAccount *v1.ServiceAccount) {
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

func needsDockercfgSecret(serviceAccount *v1.ServiceAccount) bool {
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
	if !needsDockercfgSecret(obj.(*v1.ServiceAccount)) {
		return nil
	}

	serviceAccount := obj.(*v1.ServiceAccount).DeepCopyObject().(*v1.ServiceAccount)

	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(serviceAccount)

	// If we have a pull secret in one list, use it for the other.  It must only be in one list because
	// otherwise we wouldn't "needsDockercfgSecret"
	foundPullSecret := len(imageDockercfgPullSecrets) > 0
	foundMountableSecret := len(mountableDockercfgSecrets) > 0
	if foundPullSecret || foundMountableSecret {
		switch {
		case foundPullSecret:
			serviceAccount.Secrets = append(serviceAccount.Secrets, v1.ObjectReference{Name: imageDockercfgPullSecrets.List()[0]})
		case foundMountableSecret:
			serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, v1.LocalObjectReference{Name: mountableDockercfgSecrets.List()[0]})
		}
		// Clear the pending token annotation when updating
		delete(serviceAccount.Annotations, PendingTokenAnnotation)

		updatedSA, err := e.client.Core().ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
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
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if !first {
			obj, exists, err := e.serviceAccountCache.GetByKey(key)
			if err != nil {
				return err
			}
			if !exists || !needsDockercfgSecret(obj.(*v1.ServiceAccount)) || serviceAccount.UID != obj.(*v1.ServiceAccount).UID {
				// somehow a dockercfg secret appeared or the SA disappeared.  cleanup the secret we made and return
				glog.V(2).Infof("Deleting secret because the work is already done %s/%s", dockercfgSecret.Namespace, dockercfgSecret.Name)
				e.client.Core().Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name, nil)
				return nil
			}

			serviceAccount = obj.(*v1.ServiceAccount).DeepCopyObject().(*v1.ServiceAccount)
		}
		first = false

		serviceAccount.Secrets = append(serviceAccount.Secrets, v1.ObjectReference{Name: dockercfgSecret.Name})
		serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, v1.LocalObjectReference{Name: dockercfgSecret.Name})
		// Clear the pending token annotation when updating
		delete(serviceAccount.Annotations, PendingTokenAnnotation)

		updatedSA, err := e.client.Core().ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
		if err == nil {
			e.serviceAccountCache.Mutation(updatedSA)
		}
		return err
	})

	if err != nil {
		// nothing to do.  Our choice was stale or we got a conflict.  Either way that means that the service account was updated.  We simply need to return because we'll get an update notification later
		// we do need to clean up our dockercfgSecret.  token secrets are cleaned up by the controller handling service account dockercfg secret deletes
		glog.V(2).Infof("Deleting secret %s/%s (err=%v)", dockercfgSecret.Namespace, dockercfgSecret.Name, err)
		e.client.Core().Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name, nil)
	}
	return err
}

// createTokenSecret creates a token secret for a given service account.  Returns the name of the token
func (e *DockercfgController) createTokenSecret(serviceAccount *v1.ServiceAccount) (*v1.Secret, bool, error) {
	pendingTokenName := serviceAccount.Annotations[PendingTokenAnnotation]

	// If this service account has no record of a pending token name, record one
	if len(pendingTokenName) == 0 {
		pendingTokenName = secret.Strategy.GenerateName(osautil.GetTokenSecretNamePrefixV1(serviceAccount))
		if serviceAccount.Annotations == nil {
			serviceAccount.Annotations = map[string]string{}
		}
		serviceAccount.Annotations[PendingTokenAnnotation] = pendingTokenName
		updatedServiceAccount, err := e.client.Core().ServiceAccounts(serviceAccount.Namespace).Update(serviceAccount)
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
		existingTokenSecret := existingTokenSecretObj.(*v1.Secret)
		return existingTokenSecret, len(existingTokenSecret.Data[v1.ServiceAccountTokenKey]) > 0, nil
	}

	// Try to create the named pending token
	tokenSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pendingTokenName,
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				v1.ServiceAccountNameKey:               serviceAccount.Name,
				v1.ServiceAccountUIDKey:                string(serviceAccount.UID),
				oapi.DeprecatedKubeCreatedByAnnotation: CreateDockercfgSecretsController,
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{},
	}

	glog.V(4).Infof("Creating token secret %q for service account %s/%s", tokenSecret.Name, serviceAccount.Namespace, serviceAccount.Name)
	token, err := e.client.Core().Secrets(tokenSecret.Namespace).Create(tokenSecret)
	// Already exists but not in cache means we'll get an add watch event and resync
	if kapierrors.IsAlreadyExists(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return token, len(token.Data[v1.ServiceAccountTokenKey]) > 0, nil
}

// createDockerPullSecret creates a dockercfg secret based on the token secret
func (e *DockercfgController) createDockerPullSecret(serviceAccount *v1.ServiceAccount) (*v1.Secret, bool, error) {
	tokenSecret, isPopulated, err := e.createTokenSecret(serviceAccount)
	if err != nil {
		return nil, false, err
	}
	if !isPopulated {
		glog.V(5).Infof("Token secret for service account %s/%s is not populated yet", serviceAccount.Namespace, serviceAccount.Name)
		return nil, false, nil
	}

	dockercfgSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Strategy.GenerateName(osautil.GetDockercfgSecretNamePrefixV1(serviceAccount)),
			Namespace: tokenSecret.Namespace,
			Annotations: map[string]string{
				v1.ServiceAccountNameKey:           serviceAccount.Name,
				v1.ServiceAccountUIDKey:            string(serviceAccount.UID),
				ServiceAccountTokenSecretNameKey:   string(tokenSecret.Name),
				ServiceAccountTokenValueAnnotation: string(tokenSecret.Data[v1.ServiceAccountTokenKey]),
			},
		},
		Type: v1.SecretTypeDockercfg,
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
			Password: string(tokenSecret.Data[v1.ServiceAccountTokenKey]),
			Email:    "serviceaccount@example.org",
		}
	}
	dockercfgContent, err := json.Marshal(&dockercfg)
	if err != nil {
		return nil, false, err
	}
	dockercfgSecret.Data[v1.DockerConfigKey] = dockercfgContent

	// Save the secret
	createdSecret, err := e.client.Core().Secrets(tokenSecret.Namespace).Create(dockercfgSecret)
	return createdSecret, err == nil, err
}

func getGeneratedDockercfgSecretNames(serviceAccount *v1.ServiceAccount) (sets.String, sets.String) {
	mountableDockercfgSecrets := sets.String{}
	imageDockercfgPullSecrets := sets.String{}

	secretNamePrefix := osautil.GetDockercfgSecretNamePrefixV1(serviceAccount)

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

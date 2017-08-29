package controllers

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"reflect"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions/core/v1"
	corelisters "k8s.io/kubernetes/pkg/client/listers/core/v1"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

const (
	StableSATokenNameKey          = "openshift.io/stable-serviceaccount-token-secret"
	StableSATokenSARefNameKey     = "openshift.io/stable-serviceaccount-token-secret-sa-ref"
	StableSATokenSecretRefNameKey = "openshift.io/stable-serviceaccount-token-secret-secret-ref"
)

// StableSATokenController provides a secret that contains an SA token at a stable name
type StableSATokenController struct {
	name   string
	client kclientset.Interface

	saLister     corelisters.ServiceAccountLister
	saSynced     cache.InformerSynced
	secretLister corelisters.SecretLister
	secretSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

func NewStableSATokenSecretController(serviceAccounts informers.ServiceAccountInformer, secrets informers.SecretInformer, cl kclientset.Interface) *StableSATokenController {
	c := &StableSATokenController{
		name:         "stable-serviceaccount-token-controller",
		client:       cl,
		saLister:     serviceAccounts.Lister(),
		saSynced:     serviceAccounts.Informer().HasSynced,
		secretLister: secrets.Lister(),
		secretSynced: secrets.Informer().HasSynced,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "stable-serviceaccount-token-controller"),
	}

	serviceAccounts.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.ServiceAccount:
					// only queue SAs that are requesting stable names
					if _, ok := t.Annotations[StableSATokenNameKey]; ok {
						return true
					}
					return false
				default:
					utilruntime.HandleError(fmt.Errorf("object passed to %T that is not expected: %T", c, obj))
					return false
				}
			},
			Handler: naiveEventHandler(c.queue),
		},
	)

	secrets.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Secret:
					if t.Type == v1.SecretTypeServiceAccountToken {
						return true
					}
					if isStableSATokenSecret(t) {
						return true
					}
					return false

				default:
					utilruntime.HandleError(fmt.Errorf("object passed to %T that is not expected: %T", c, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    func(cur interface{}) { c.handleTokenSecretUpdate(nil, cur) },
				UpdateFunc: func(old, cur interface{}) { c.handleTokenSecretUpdate(old, cur) },
				DeleteFunc: c.handleTokenSecretDelete,
			},
		},
	)

	return c
}

// syncHandler gets triggered with an SA key.  It needs to check to:
// 1. see if should remove previously controlled keys
// 2. see if the stable secret already exists with a valid token
// 3. check to see if we have a valid SA token to use
// 4. if we don't have a valid SA token, requeue rate limited and try again later once one is created
func (c *StableSATokenController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	sa, err := c.saLister.ServiceAccounts(namespace).Get(name)
	if kapierrors.IsNotFound(err) {
		c.removePreviouslyControlledTokenSecrets(key)
		return nil
	}
	if err != nil {
		return err
	}

	stableSecretName := sa.Annotations[StableSATokenNameKey]
	if len(stableSecretName) == 0 {
		c.removePreviouslyControlledTokenSecrets(key)
		return nil
	}

	existingStableSecret, err := c.secretLister.Secrets(namespace).Get(stableSecretName)
	if kapierrors.IsNotFound(err) {
		c.createStableSecret(sa)
		return nil
	}
	if err != nil {
		return err
	}

	return c.syncStableSecret(existingStableSecret, sa)
}

func (c *StableSATokenController) syncStableSecret(secret *v1.Secret, sa *v1.ServiceAccount) error {
	// first check to see if the stable SA token is valid
	originalSecretName := secret.Annotations[StableSATokenSecretRefNameKey]
	originalSecret, err := c.secretLister.Secrets(secret.Namespace).Get(originalSecretName)
	if err == nil && serviceaccount.IsServiceAccountToken(originalSecret, sa) && reflect.DeepEqual(originalSecret.Data[v1.ServiceAccountTokenKey], secret.Data[v1.ServiceAccountTokenKey]) {
		return nil
	}
	// if not everything was perfect, we're going to update either to empty or to a new value
	uncast, err := kapi.Scheme.Copy(secret)
	if err != nil {
		return nil
	}
	secretCopy := uncast.(*v1.Secret)

	saTokenSecretName, saToken := c.getSAToken(sa)
	secretCopy.Annotations[StableSATokenSecretRefNameKey] = saTokenSecretName
	secretCopy.Data[v1.ServiceAccountTokenKey] = saToken

	// if we don't yet have a token for this SA, requeue and try again later
	if len(saToken) == 0 {
		key, err := controller.KeyFunc(sa)
		if err != nil {
			utilruntime.HandleError(err)
			return nil
		}
		c.queue.AddRateLimited(key)
	}

	// we want to update regardless to clear old values
	_, err = c.client.CoreV1().Secrets(secret.Namespace).Update(secretCopy)
	return err
}

func (c *StableSATokenController) getSAToken(sa *v1.ServiceAccount) (string, []byte) {
	secrets, err := c.secretLister.Secrets(sa.Namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return "", nil
	}

	var saToken []byte
	var saTokenSecretName string
	for _, secret := range secrets {
		if !serviceaccount.IsServiceAccountToken(secret, sa) {
			continue
		}
		saToken = secret.Data[v1.ServiceAccountTokenKey]
		if len(saToken) > 0 {
			saTokenSecretName = secret.Name
			break
		}
	}

	return saTokenSecretName, saToken
}

func (c *StableSATokenController) createStableSecret(sa *v1.ServiceAccount) error {
	stableSecretName := sa.Annotations[StableSATokenNameKey]

	saTokenSecretName, saToken := c.getSAToken(sa)
	// if we don't yet have a token for this SA, requeue and try again later
	if len(saToken) == 0 {
		key, err := controller.KeyFunc(sa)
		if err != nil {
			utilruntime.HandleError(err)
			return nil
		}
		c.queue.AddRateLimited(key)
		return nil
	}

	stableSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sa.Namespace,
			Name:      stableSecretName,
			Annotations: map[string]string{
				StableSATokenSARefNameKey:     sa.Name,
				StableSATokenSecretRefNameKey: saTokenSecretName,
			},
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			v1.ServiceAccountTokenKey: saToken,
		},
	}
	_, err := c.client.CoreV1().Secrets(stableSecret.Namespace).Create(stableSecret)
	return err
}

func (c *StableSATokenController) removePreviouslyControlledTokenSecrets(key string) {
	namespace, saName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	secrets, err := c.secretLister.Secrets(namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	for _, secret := range secrets {
		if !isStableSATokenSecret(secret) {
			continue
		}
		currSAName := secret.Annotations[StableSATokenSARefNameKey]
		if currSAName != saName {
			continue
		}
		if err := c.client.CoreV1().Secrets(namespace).Delete(secret.Name, nil); err != nil {
			utilruntime.HandleError(err)
			continue
		}
	}
}

func naiveEventHandler(queue workqueue.Interface) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := controller.KeyFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			queue.Add(key)
		},
		UpdateFunc: func(old, cur interface{}) {
			key, err := controller.KeyFunc(cur)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := getDeleteKey(obj)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			queue.Add(key)
		},
	}
}

func getDeleteKey(uncast interface{}) (string, error) {
	obj, ok := uncast.(runtime.Object)
	if !ok {
		tombstone, ok := uncast.(cache.DeletedFinalStateUnknown)
		if !ok {
			return "", fmt.Errorf("Couldn't get object from tombstone %#v", uncast)
		}
		obj, ok = tombstone.Obj.(runtime.Object)
		if !ok {
			return "", fmt.Errorf("Tombstone contained object that is not a runtime.Object %#v", uncast)
		}
	}
	return controller.KeyFunc(obj)
}

func (c StableSATokenController) handleTokenSecretUpdate(oldObj, newObj interface{}) {
	secret := newObj.(*v1.Secret)
	if secret.Annotations[v1.CreatedByAnnotation] != CreateDockercfgSecretsController {
		return
	}
	isPopulated := len(secret.Data[v1.ServiceAccountTokenKey]) > 0

	wasPopulated := false
	if oldObj != nil {
		oldSecret := oldObj.(*v1.Secret)
		wasPopulated = len(oldSecret.Data[v1.ServiceAccountTokenKey]) > 0
	} else {
	}

	if !wasPopulated && isPopulated {
		c.enqueueServiceAccountForToken(secret)
	}
}

func (c StableSATokenController) handleTokenSecretDelete(obj interface{}) {
	secret, isSecret := obj.(*v1.Secret)
	if !isSecret {
		tombstone, objIsTombstone := obj.(cache.DeletedFinalStateUnknown)
		if !objIsTombstone {
			return
		}
		secret, isSecret = tombstone.Obj.(*v1.Secret)
		if !isSecret {
			return
		}
	}
	c.enqueueServiceAccountForToken(secret)
}

func (c *StableSATokenController) enqueueServiceAccountForToken(secret *v1.Secret) {
	saName := ""
	if isStableSATokenSecret(secret) {
		saName = secret.Annotations[StableSATokenSARefNameKey]
	} else {
		saName = secret.Annotations[v1.ServiceAccountNameKey]
	}

	sa, err := c.saLister.ServiceAccounts(secret.Namespace).Get(saName)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing token secret %s/%s: %v", secret.Namespace, secret.Name, err))
		return
	}
	// only queue SAs that are requesting stable names
	if _, ok := sa.Annotations[StableSATokenNameKey]; !ok {
		return
	}
	key, err := controller.KeyFunc(sa)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing token secret %s/%s: %v", secret.Namespace, secret.Name, err))
		return
	}
	c.queue.Add(key)
}

func (c *StableSATokenController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer glog.Infof("Shutting %v controller", c.name)

	glog.Infof("Starting %v controller", c.name)

	if !cache.WaitForCacheSync(stopCh, c.saSynced, c.secretSynced) {
		utilruntime.HandleError(fmt.Errorf("%v: timed out waiting for caches to sync", c.name))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *StableSATokenController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *StableSATokenController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v: %v failed with : %v", c.name, key, err))
	c.queue.AddRateLimited(key)

	return true
}

func isStableSATokenSecret(secret *v1.Secret) bool {
	if _, ok := secret.Annotations[StableSATokenSARefNameKey]; ok {
		return true
	}
	return false
}

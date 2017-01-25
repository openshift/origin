package servingcert

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/crypto/extensions"
)

// ServiceServingCertUpdateController is responsible for synchronizing Service objects stored
// in the system with actual running replica sets and pods.
type ServiceServingCertUpdateController struct {
	secretClient kcoreclient.SecretsGetter

	// Services that need to be checked
	queue workqueue.RateLimitingInterface

	serviceCache      cache.Store
	serviceController *cache.Controller
	serviceHasSynced  informerSynced

	secretCache      cache.Store
	secretController *cache.Controller
	secretHasSynced  informerSynced

	ca         *crypto.CA
	publicCert string
	dnsSuffix  string
	// minTimeLeftForCert is how much time is remaining for the serving cert before regenerating it.
	minTimeLeftForCert time.Duration

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// NewServiceServingCertUpdateController creates a new ServiceServingCertUpdateController.
// TODO this should accept a shared informer
func NewServiceServingCertUpdateController(serviceClient kcoreclient.ServicesGetter, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertUpdateController {
	sc := &ServiceServingCertUpdateController{
		secretClient: secretClient,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		ca:        ca,
		dnsSuffix: dnsSuffix,
		// TODO base the expiry time on a percentage of the time for the lifespan of the cert
		minTimeLeftForCert: 1 * time.Hour,
	}

	sc.serviceCache, sc.serviceController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return serviceClient.Services(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return serviceClient.Services(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.Service{},
		resyncInterval,
		cache.ResourceEventHandlerFuncs{},
	)
	sc.serviceHasSynced = sc.serviceController.HasSynced

	sc.secretCache, sc.secretController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return sc.secretClient.Secrets(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return sc.secretClient.Secrets(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.Secret{},
		resyncInterval,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.addSecret,
			UpdateFunc: sc.updateSecret,
		},
	)
	sc.secretHasSynced = sc.secretController.HasSynced

	sc.syncHandler = sc.syncSecret

	return sc
}

// Run begins watching and syncing.
func (sc *ServiceServingCertUpdateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer glog.Infof("Shutting down service signing cert update controller")
	defer sc.queue.ShutDown()

	glog.Infof("starting service signing cert update controller")
	go sc.serviceController.Run(stopCh)
	go sc.secretController.Run(stopCh)

	if !waitForCacheSync(stopCh, sc.serviceHasSynced, sc.secretHasSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(sc.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

// TODO this is all in the kube library after the 1.5 rebase

// informerSynced is a function that can be used to determine if an informer has synced.  This is useful for determining if caches have synced.
type informerSynced func() bool

// syncedPollPeriod controls how often you look at the status of your sync funcs
const syncedPollPeriod = 100 * time.Millisecond

func waitForCacheSync(stopCh <-chan struct{}, cacheSyncs ...informerSynced) bool {
	err := wait.PollUntil(syncedPollPeriod,
		func() (bool, error) {
			for _, syncFunc := range cacheSyncs {
				if !syncFunc() {
					return false, nil
				}
			}
			return true, nil
		},
		stopCh)
	if err != nil {
		glog.V(2).Infof("stop requested")
		return false
	}

	glog.V(4).Infof("caches populated")
	return true
}

func (sc *ServiceServingCertUpdateController) enqueueSecret(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	sc.queue.Add(key)
}

func (sc *ServiceServingCertUpdateController) addSecret(obj interface{}) {
	secret := obj.(*kapi.Secret)
	if len(secret.Annotations[ServiceNameAnnotation]) == 0 {
		return
	}

	glog.V(4).Infof("adding %s", secret.Name)
	sc.enqueueSecret(secret)
}

func (sc *ServiceServingCertUpdateController) updateSecret(old, cur interface{}) {
	secret := cur.(*kapi.Secret)
	if len(secret.Annotations[ServiceNameAnnotation]) == 0 {
		// if the current doesn't have a service name, check the old
		secret = old.(*kapi.Secret)
		if len(secret.Annotations[ServiceNameAnnotation]) == 0 {
			return
		}
	}

	glog.V(4).Infof("updating %s", secret.Name)
	sc.enqueueSecret(secret)
}

func (sc *ServiceServingCertUpdateController) runWorker() {
	for sc.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (sc *ServiceServingCertUpdateController) processNextWorkItem() bool {
	key, quit := sc.queue.Get()
	if quit {
		return false
	}
	defer sc.queue.Done(key)

	err := sc.syncHandler(key.(string))
	if err == nil {
		sc.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	sc.queue.AddRateLimited(key)

	return true
}

// syncSecret will sync the service with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (sc *ServiceServingCertUpdateController) syncSecret(key string) error {
	obj, exists, err := sc.secretCache.GetByKey(key)
	if err != nil {
		glog.V(4).Infof("Unable to retrieve service %v from store: %v", key, err)
		return err
	}
	if !exists {
		glog.V(4).Infof("Secret has been deleted %v", key)
		return nil
	}

	regenerate, service := sc.requiresRegeneration(obj.(*kapi.Secret))
	if !regenerate {
		return nil
	}

	// make a copy to avoid mutating cache state
	t, err := kapi.Scheme.DeepCopy(obj)
	if err != nil {
		return err
	}
	secret := t.(*kapi.Secret)

	dnsName := service.Name + "." + secret.Namespace + ".svc"
	fqDNSName := dnsName + "." + sc.dnsSuffix
	certificateLifetime := 365 * 2 // 2 years
	servingCert, err := sc.ca.MakeServerCert(
		sets.NewString(dnsName, fqDNSName),
		certificateLifetime,
		extensions.ServiceServerCertificateExtension(service),
	)
	if err != nil {
		return err
	}
	secret.Annotations[ServingCertExpiryAnnotation] = servingCert.Certs[0].NotAfter.Format(time.RFC3339)
	secret.Data[kapi.TLSCertKey], secret.Data[kapi.TLSPrivateKeyKey], err = servingCert.GetPEMBytes()
	if err != nil {
		return err
	}

	_, err = sc.secretClient.Secrets(secret.Namespace).Update(secret)
	return err
}

func (sc *ServiceServingCertUpdateController) requiresRegeneration(secret *kapi.Secret) (bool, *kapi.Service) {
	serviceName := secret.Annotations[ServiceNameAnnotation]
	if len(serviceName) == 0 {
		return false, nil
	}

	serviceObj, exists, err := sc.serviceCache.GetByKey(secret.Namespace + "/" + serviceName)
	if err != nil {
		return false, nil
	}
	if !exists {
		return false, nil
	}

	service := serviceObj.(*kapi.Service)
	if service.Annotations[ServingCertSecretAnnotation] != secret.Name {
		return false, nil
	}
	if secret.Annotations[ServiceUIDAnnotation] != string(service.UID) {
		return false, nil
	}

	// if we don't have the annotation for expiry, just go ahead and regenerate.  It's easier than writing a
	// secondary logic flow that creates the expiry dates
	expiryString, ok := secret.Annotations[ServingCertExpiryAnnotation]
	if !ok {
		return true, service
	}
	expiry, err := time.Parse(time.RFC3339, expiryString)
	if err != nil {
		return true, service
	}

	if time.Now().Add(sc.minTimeLeftForCert).After(expiry) {
		return true, service
	}

	return false, nil
}

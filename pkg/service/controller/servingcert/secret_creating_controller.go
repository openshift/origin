package servingcert

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
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

const (
	// ServingCertSecretAnnotation stores the name of the secret to generate into.
	ServingCertSecretAnnotation = "service.alpha.openshift.io/serving-cert-secret-name"
	// ServingCertCreatedByAnnotation stores the of the signer common name.  This could be used later to see if the
	// services need to have the the serving certs regenerated.  The presence and matching of this annotation prevents
	// regeneration
	ServingCertCreatedByAnnotation = "service.alpha.openshift.io/serving-cert-signed-by"
	// ServingCertErrorAnnotation stores the error that caused cert generation failures.
	ServingCertErrorAnnotation = "service.alpha.openshift.io/serving-cert-generation-error"
	// ServingCertErrorNumAnnotation stores how many consecutive errors we've hit.  A value of the maxRetries will prevent
	// the controller from reattempting until it is cleared.
	ServingCertErrorNumAnnotation = "service.alpha.openshift.io/serving-cert-generation-error-num"
	// ServiceUIDAnnotation is an annotation on a secret that indicates which service created it, by UID
	ServiceUIDAnnotation = "service.alpha.openshift.io/originating-service-uid"
	// ServiceNameAnnotation is an annotation on a secret that indicates which service created it, by Name to allow reverse lookups on services
	// for comparison against UIDs
	ServiceNameAnnotation = "service.alpha.openshift.io/originating-service-name"
	// ServingCertExpiryAnnotation is an annotation that holds the expiry time of the certificate.  It accepts time in the
	// RFC3339 format: 2018-11-29T17:44:39Z
	ServingCertExpiryAnnotation = "service.alpha.openshift.io/expiry"
)

// ServiceServingCertController is responsible for synchronizing Service objects stored
// in the system with actual running replica sets and pods.
type ServiceServingCertController struct {
	serviceClient kcoreclient.ServicesGetter
	secretClient  kcoreclient.SecretsGetter

	// Services that need to be checked
	queue      workqueue.RateLimitingInterface
	maxRetries int

	serviceCache      cache.Store
	serviceController *cache.Controller
	serviceHasSynced  informerSynced

	secretCache      cache.Store
	secretController *cache.Controller
	secretHasSynced  informerSynced

	ca         *crypto.CA
	publicCert string
	dnsSuffix  string

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// NewServiceServingCertController creates a new ServiceServingCertController.
// TODO this should accept a shared informer
func NewServiceServingCertController(serviceClient kcoreclient.ServicesGetter, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertController {
	sc := &ServiceServingCertController{
		serviceClient: serviceClient,
		secretClient:  secretClient,

		queue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		maxRetries: 10,

		ca:        ca,
		dnsSuffix: dnsSuffix,
	}

	sc.serviceCache, sc.serviceController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return sc.serviceClient.Services(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return sc.serviceClient.Services(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.Service{},
		resyncInterval,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := obj.(*kapi.Service)
				glog.V(4).Infof("Adding service %s", service.Name)
				sc.enqueueService(obj)
			},
			UpdateFunc: func(old, cur interface{}) {
				service := cur.(*kapi.Service)
				glog.V(4).Infof("Updating service %s", service.Name)
				// Resync on service object relist.
				sc.enqueueService(cur)
			},
		},
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
			DeleteFunc: sc.deleteSecret,
		},
	)
	sc.secretHasSynced = sc.secretController.HasSynced

	sc.syncHandler = sc.syncService

	return sc
}

// Run begins watching and syncing.
func (sc *ServiceServingCertController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go sc.serviceController.Run(stopCh)
	go sc.secretController.Run(stopCh)
	if !waitForCacheSync(stopCh, sc.serviceHasSynced, sc.secretHasSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(sc.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down service signing cert controller")
	sc.queue.ShutDown()
}

// deleteSecret handles the case when the service certificate secret is manually removed.
// In that case the secret will be automatically recreated.
func (sc *ServiceServingCertController) deleteSecret(obj interface{}) {
	secret, ok := obj.(*kapi.Secret)
	if !ok {
		return
	}
	if _, exists := secret.Annotations[ServiceNameAnnotation]; !exists {
		return
	}
	serviceObj, exists, err := sc.serviceCache.GetByKey(secret.Namespace + "/" + secret.Annotations[ServiceNameAnnotation])
	if !exists {
		return
	}
	if err != nil {
		return
	}
	service := serviceObj.(*kapi.Service)
	glog.V(4).Infof("Recreating secret for service %q", service.Namespace+"/"+service.Name)

	sc.enqueueService(serviceObj)
}

func (sc *ServiceServingCertController) enqueueService(obj interface{}) {
	_, ok := obj.(*kapi.Service)
	if !ok {
		return
	}
	key, err := controller.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	sc.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (sc *ServiceServingCertController) worker() {
	for {
		if !sc.work() {
			return
		}
	}
}

// work returns true if the worker thread should continue
func (sc *ServiceServingCertController) work() bool {
	key, quit := sc.queue.Get()
	if quit {
		return false
	}
	defer sc.queue.Done(key)

	if err := sc.syncHandler(key.(string)); err == nil {
		// this means the request was successfully handled.  We should "forget" the item so that any retry
		// later on is reset
		sc.queue.Forget(key)

	} else {
		// if we had an error it means that we didn't handle it, which means that we want to requeue the work
		utilruntime.HandleError(fmt.Errorf("error syncing service, it will be retried: %v", err))
		sc.queue.AddRateLimited(key)
	}

	return true
}

// syncService will sync the service with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (sc *ServiceServingCertController) syncService(key string) error {
	obj, exists, err := sc.serviceCache.GetByKey(key)
	if err != nil {
		glog.V(4).Infof("Unable to retrieve service %v from store: %v", key, err)
		return err
	}
	if !exists {
		glog.V(4).Infof("Service has been deleted %v", key)
		return nil
	}

	if !sc.requiresCertGeneration(obj.(*kapi.Service)) {
		return nil
	}

	// make a copy to avoid mutating cache state
	t, err := kapi.Scheme.DeepCopy(obj)
	if err != nil {
		return err
	}
	service := t.(*kapi.Service)
	if service.Annotations == nil {
		service.Annotations = map[string]string{}
	}

	dnsName := service.Name + "." + service.Namespace + ".svc"
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
	certBytes, keyBytes, err := servingCert.GetPEMBytes()
	if err != nil {
		return err
	}

	secret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: service.Namespace,
			Name:      service.Annotations[ServingCertSecretAnnotation],
			Annotations: map[string]string{
				ServiceUIDAnnotation:        string(service.UID),
				ServiceNameAnnotation:       service.Name,
				ServingCertExpiryAnnotation: servingCert.Certs[0].NotAfter.Format(time.RFC3339),
			},
		},
		Type: kapi.SecretTypeTLS,
		Data: map[string][]byte{
			kapi.TLSCertKey:       certBytes,
			kapi.TLSPrivateKeyKey: keyBytes,
		},
	}

	_, err = sc.secretClient.Secrets(service.Namespace).Create(secret)
	if err != nil && !kapierrors.IsAlreadyExists(err) {
		// if we have an error creating the secret, then try to update the service with that information.  If it fails,
		// then we'll just try again later on  re-list or because the service had already been updated and we'll get triggered again.
		service.Annotations[ServingCertErrorAnnotation] = err.Error()
		service.Annotations[ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(service) + 1)
		_, updateErr := sc.serviceClient.Services(service.Namespace).Update(service)

		// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
		if updateErr == nil && getNumFailures(service) >= sc.maxRetries {
			return nil
		}
		return err
	}
	if kapierrors.IsAlreadyExists(err) {
		actualSecret, err := sc.secretClient.Secrets(service.Namespace).Get(secret.Name)
		if err != nil {
			// if we have an error creating the secret, then try to update the service with that information.  If it fails,
			// then we'll just try again later on  re-list or because the service had already been updated and we'll get triggered again.
			service.Annotations[ServingCertErrorAnnotation] = err.Error()
			service.Annotations[ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(service) + 1)
			_, updateErr := sc.serviceClient.Services(service.Namespace).Update(service)

			// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
			if updateErr == nil && getNumFailures(service) >= sc.maxRetries {
				return nil
			}
			return err
		}

		if actualSecret.Annotations[ServiceUIDAnnotation] != string(service.UID) {
			service.Annotations[ServingCertErrorAnnotation] = fmt.Sprintf("secret/%v references serviceUID %v, which does not match %v", actualSecret.Name, actualSecret.Annotations[ServiceUIDAnnotation], service.UID)
			service.Annotations[ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(service) + 1)
			_, updateErr := sc.serviceClient.Services(service.Namespace).Update(service)

			// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
			if updateErr == nil && getNumFailures(service) >= sc.maxRetries {
				return nil
			}
			return errors.New(service.Annotations[ServingCertErrorAnnotation])
		}
	}

	service.Annotations[ServingCertCreatedByAnnotation] = sc.ca.Config.Certs[0].Subject.CommonName
	delete(service.Annotations, ServingCertErrorAnnotation)
	delete(service.Annotations, ServingCertErrorNumAnnotation)
	_, err = sc.serviceClient.Services(service.Namespace).Update(service)

	return err
}

func getNumFailures(service *kapi.Service) int {
	numFailuresString := service.Annotations[ServingCertErrorNumAnnotation]
	if len(numFailuresString) == 0 {
		return 0
	}

	numFailures, err := strconv.Atoi(numFailuresString)
	if err != nil {
		return 0
	}
	return numFailures
}

func (sc *ServiceServingCertController) requiresCertGeneration(service *kapi.Service) bool {
	secretName := service.Annotations[ServingCertSecretAnnotation]
	if len(secretName) == 0 {
		return false
	}
	if getNumFailures(service) >= sc.maxRetries {
		return false
	}
	if service.Annotations[ServingCertCreatedByAnnotation] == sc.ca.Config.Certs[0].Subject.CommonName {
		return false
	}
	// TODO: use the lister here
	if _, exists, _ := sc.secretCache.GetByKey(service.Namespace + "/" + secretName); !exists {
		return true
	}
	return true
}

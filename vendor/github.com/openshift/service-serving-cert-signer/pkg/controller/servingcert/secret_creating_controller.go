package servingcert

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/openshift/library-go/pkg/crypto"
	ocontroller "github.com/openshift/library-go/pkg/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert/cryptoextensions"
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

	serviceLister    listers.ServiceLister
	serviceHasSynced cache.InformerSynced

	secretLister    listers.SecretLister
	secretHasSynced cache.InformerSynced

	ca        *crypto.CA
	dnsSuffix string

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// NewServiceServingCertController creates a new ServiceServingCertController.
// TODO this should accept a shared informer
func NewServiceServingCertController(services informers.ServiceInformer, secrets informers.SecretInformer, serviceClient kcoreclient.ServicesGetter, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertController {
	sc := &ServiceServingCertController{
		serviceClient: serviceClient,
		secretClient:  secretClient,

		queue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		maxRetries: 10,

		ca:        ca,
		dnsSuffix: dnsSuffix,
	}

	services.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := obj.(*v1.Service)
				glog.V(4).Infof("Adding service %s", service.Name)
				sc.enqueueService(obj)
			},
			UpdateFunc: func(old, cur interface{}) {
				service := cur.(*v1.Service)
				glog.V(4).Infof("Updating service %s", service.Name)
				// Resync on service object relist.
				sc.enqueueService(cur)
			},
		},
		resyncInterval,
	)
	sc.serviceLister = services.Lister()
	sc.serviceHasSynced = services.Informer().GetController().HasSynced

	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			DeleteFunc: sc.deleteSecret,
		},
		resyncInterval,
	)
	sc.secretHasSynced = services.Informer().GetController().HasSynced
	sc.secretLister = secrets.Lister()

	sc.syncHandler = sc.syncService

	return sc
}

// Run begins watching and syncing.
func (sc *ServiceServingCertController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer sc.queue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, sc.serviceHasSynced, sc.secretHasSynced) {
		return
	}

	glog.V(5).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(sc.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

// deleteSecret handles the case when the service certificate secret is manually removed.
// In that case the secret will be automatically recreated.
func (sc *ServiceServingCertController) deleteSecret(obj interface{}) {
	secret, ok := obj.(*v1.Secret)
	if !ok {
		return
	}
	if _, exists := secret.Annotations[ServiceNameAnnotation]; !exists {
		return
	}
	service, err := sc.serviceLister.Services(secret.Namespace).Get(secret.Annotations[ServiceNameAnnotation])
	if kapierrors.IsNotFound(err) {
		return
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get service %s/%s: %v", secret.Namespace, secret.Annotations[ServiceNameAnnotation], err))
		return
	}
	glog.V(4).Infof("Recreating secret for service %q", service.Namespace+"/"+service.Name)
	sc.enqueueService(service)
}

func (sc *ServiceServingCertController) enqueueService(obj interface{}) {
	_, ok := obj.(*v1.Service)
	if !ok {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
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
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	sharedService, err := sc.serviceLister.Services(namespace).Get(name)
	if kapierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if !sc.requiresCertGeneration(sharedService) {
		return nil
	}

	// make a copy to avoid mutating cache state
	serviceCopy := sharedService.DeepCopy()
	if serviceCopy.Annotations == nil {
		serviceCopy.Annotations = map[string]string{}
	}

	dnsName := serviceCopy.Name + "." + serviceCopy.Namespace + ".svc"
	fqDNSName := dnsName + "." + sc.dnsSuffix
	certificateLifetime := 365 * 2 // 2 years
	servingCert, err := sc.ca.MakeServerCert(
		sets.NewString(dnsName, fqDNSName),
		certificateLifetime,
		cryptoextensions.ServiceServerCertificateExtensionV1(serviceCopy),
	)
	if err != nil {
		return err
	}
	certBytes, keyBytes, err := servingCert.GetPEMBytes()
	if err != nil {
		return err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceCopy.Namespace,
			Name:      serviceCopy.Annotations[ServingCertSecretAnnotation],
			Annotations: map[string]string{
				ServiceUIDAnnotation:        string(serviceCopy.UID),
				ServiceNameAnnotation:       serviceCopy.Name,
				ServingCertExpiryAnnotation: servingCert.Certs[0].NotAfter.Format(time.RFC3339),
			},
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			v1.TLSCertKey:       certBytes,
			v1.TLSPrivateKeyKey: keyBytes,
		},
	}
	ocontroller.EnsureOwnerRef(secret, ownerRef(serviceCopy))

	_, err = sc.secretClient.Secrets(serviceCopy.Namespace).Create(secret)
	if err != nil && !kapierrors.IsAlreadyExists(err) {
		// if we have an error creating the secret, then try to update the service with that information.  If it fails,
		// then we'll just try again later on  re-list or because the service had already been updated and we'll get triggered again.
		serviceCopy.Annotations[ServingCertErrorAnnotation] = err.Error()
		serviceCopy.Annotations[ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(serviceCopy) + 1)
		_, updateErr := sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

		// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
		if updateErr == nil && getNumFailures(serviceCopy) >= sc.maxRetries {
			return nil
		}
		return err
	}
	if kapierrors.IsAlreadyExists(err) {
		actualSecret, err := sc.secretClient.Secrets(serviceCopy.Namespace).Get(secret.Name, metav1.GetOptions{})
		if err != nil {
			// if we have an error creating the secret, then try to update the service with that information.  If it fails,
			// then we'll just try again later on  re-list or because the service had already been updated and we'll get triggered again.
			serviceCopy.Annotations[ServingCertErrorAnnotation] = err.Error()
			serviceCopy.Annotations[ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(serviceCopy) + 1)
			_, updateErr := sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

			// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
			if updateErr == nil && getNumFailures(serviceCopy) >= sc.maxRetries {
				return nil
			}
			return err
		}

		if actualSecret.Annotations[ServiceUIDAnnotation] != string(serviceCopy.UID) {
			serviceCopy.Annotations[ServingCertErrorAnnotation] = fmt.Sprintf("secret/%v references serviceUID %v, which does not match %v", actualSecret.Name, actualSecret.Annotations[ServiceUIDAnnotation], serviceCopy.UID)
			serviceCopy.Annotations[ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(serviceCopy) + 1)
			_, updateErr := sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

			// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
			if updateErr == nil && getNumFailures(serviceCopy) >= sc.maxRetries {
				return nil
			}
			return errors.New(serviceCopy.Annotations[ServingCertErrorAnnotation])
		}
	}

	serviceCopy.Annotations[ServingCertCreatedByAnnotation] = sc.ca.Config.Certs[0].Subject.CommonName
	delete(serviceCopy.Annotations, ServingCertErrorAnnotation)
	delete(serviceCopy.Annotations, ServingCertErrorNumAnnotation)
	_, err = sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

	return err
}

func getNumFailures(service *v1.Service) int {
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

func (sc *ServiceServingCertController) requiresCertGeneration(service *v1.Service) bool {
	secretName := service.Annotations[ServingCertSecretAnnotation]
	if len(secretName) == 0 {
		return false
	}
	if getNumFailures(service) >= sc.maxRetries {
		return false
	}
	_, err := sc.secretLister.Secrets(service.Namespace).Get(secretName)
	if kapierrors.IsNotFound(err) {
		return true
	}
	if service.Annotations[ServingCertCreatedByAnnotation] == sc.ca.Config.Certs[0].Subject.CommonName {
		return false
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get the secret %s/%s: %v", service.Namespace, secretName, err))
		return false
	}
	return true
}

func ownerRef(service *v1.Service) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       service.Name,
		UID:        service.UID,
	}
}

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
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
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
)

// ServiceServingCertController is responsible for synchronizing Service objects stored
// in the system with actual running replica sets and pods.
type ServiceServingCertController struct {
	serviceClient kclient.ServicesNamespacer
	secretClient  kclient.SecretsNamespacer

	// Services that need to be checked
	queue      workqueue.RateLimitingInterface
	maxRetries int

	serviceCache      cache.Store
	serviceController *framework.Controller

	ca         *crypto.CA
	publicCert string
	dnsSuffix  string

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// NewServiceServingCertController creates a new ServiceServingCertController.
// TODO this should accept a shared informer
func NewServiceServingCertController(serviceClient kclient.ServicesNamespacer, secretClient kclient.SecretsNamespacer, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertController {
	sc := &ServiceServingCertController{
		serviceClient: serviceClient,
		secretClient:  secretClient,

		queue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		maxRetries: 10,

		ca:        ca,
		dnsSuffix: dnsSuffix,
	}

	sc.serviceCache, sc.serviceController = framework.NewInformer(
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
		framework.ResourceEventHandlerFuncs{
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

	sc.syncHandler = sc.syncService

	return sc
}

// Run begins watching and syncing.
func (sc *ServiceServingCertController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go sc.serviceController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(sc.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down service signing cert controller")
	sc.queue.ShutDown()
}

func (sc *ServiceServingCertController) enqueueService(obj interface{}) {
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
	servingCert, err := sc.ca.MakeServerCert(sets.NewString(dnsName, fqDNSName))
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
				ServiceUIDAnnotation:  string(service.UID),
				ServiceNameAnnotation: service.Name,
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
	if secretName := service.Annotations[ServingCertSecretAnnotation]; len(secretName) == 0 {
		return false
	}
	if getNumFailures(service) >= sc.maxRetries {
		return false
	}
	if service.Annotations[ServingCertCreatedByAnnotation] == sc.ca.Config.Certs[0].Subject.CommonName {
		return false
	}

	return true
}

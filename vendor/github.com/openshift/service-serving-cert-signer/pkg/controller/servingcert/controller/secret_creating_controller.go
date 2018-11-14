package controller

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
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	ocontroller "github.com/openshift/library-go/pkg/controller"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert/cryptoextensions"
)

type ServiceServingCertController struct {
	serviceClient kcoreclient.ServicesGetter
	secretClient  kcoreclient.SecretsGetter

	serviceLister listers.ServiceLister
	secretLister  listers.SecretLister

	ca         *crypto.CA
	dnsSuffix  string
	maxRetries int

	// services that need to be checked
	queue workqueue.RateLimitingInterface

	// standard controller loop
	*controller.Controller

	// syncHandler does the work. It's factored out for unit testing
	syncHandler cache.ProcessFunc
}

func NewServiceServingCertController(services informers.ServiceInformer, secrets informers.SecretInformer, serviceClient kcoreclient.ServicesGetter, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertController {
	sc := &ServiceServingCertController{
		serviceClient: serviceClient,
		secretClient:  secretClient,

		serviceLister: services.Lister(),
		secretLister:  secrets.Lister(),

		ca:         ca,
		dnsSuffix:  dnsSuffix,
		maxRetries: 10,
	}

	services.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) { // TODO we should filter these based on annotations
				service := obj.(*v1.Service)
				glog.V(4).Infof("Adding service %s", service.Name)
				sc.enqueueService(service)
			},
			UpdateFunc: func(old, cur interface{}) { // TODO we should filter these based on annotations
				service := cur.(*v1.Service)
				glog.V(4).Infof("Updating service %s", service.Name)
				// Resync on service object relist.
				sc.enqueueService(service)
			},
		}, // TODO we may want to best effort handle deletes and clean up the secrets
		resyncInterval,
	)

	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			DeleteFunc: sc.deleteSecret,
		},
		resyncInterval,
	)

	sc.syncHandler = sc.syncService

	// need another layer of indirection so that tests can stub out syncHandler
	wrapSyncHandler := func(obj interface{}) error {
		return sc.syncHandler(obj)
	}

	internalController, queue := controller.New("ServiceServingCertController", wrapSyncHandler,
		services.Informer().GetController().HasSynced, secrets.Informer().GetController().HasSynced)

	sc.Controller = internalController
	sc.queue = queue

	return sc
}

// deleteSecret handles the case when the service certificate secret is manually removed.
// In that case the secret will be automatically recreated.
func (sc *ServiceServingCertController) deleteSecret(obj interface{}) {
	secret, ok := obj.(*v1.Secret)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("could not get object from tombstone: %+v", obj))
			return
		}
		secret, ok = tombstone.Obj.(*v1.Secret)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a secret: %+v", obj))
			return
		}
	}

	serviceName, ok := toServiceName(secret)
	if !ok {
		return
	}
	service, err := sc.serviceLister.Services(secret.Namespace).Get(serviceName)
	if kapierrors.IsNotFound(err) {
		return
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get service %s/%s: %v", secret.Namespace, serviceName, err))
		return
	}
	glog.V(4).Infof("recreating secret for service %s/%s", service.Namespace, service.Name)
	sc.enqueueService(service)
}

func (sc *ServiceServingCertController) enqueueService(obj *v1.Service) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not get key for object %+v: %v", obj, err))
		return
	}

	sc.queue.Add(key)
}

func (sc *ServiceServingCertController) syncService(obj interface{}) error {
	key := obj.(string)
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
	return sc.generateCert(serviceCopy)
}

func (sc *ServiceServingCertController) generateCert(serviceCopy *v1.Service) error {
	if serviceCopy.Annotations == nil {
		serviceCopy.Annotations = map[string]string{}
	}

	secret := toBaseSecret(serviceCopy)
	if err := toRequiredSecret(sc.dnsSuffix, sc.ca, serviceCopy, secret); err != nil {
		return err
	}

	_, err := sc.secretClient.Secrets(serviceCopy.Namespace).Create(secret)
	if err != nil && !kapierrors.IsAlreadyExists(err) {
		// if we have an error creating the secret, then try to update the service with that information.  If it fails,
		// then we'll just try again later on re-list or because the service had already been updated and we'll get triggered again.
		serviceCopy.Annotations[api.ServingCertErrorAnnotation] = err.Error()
		serviceCopy.Annotations[api.ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(serviceCopy) + 1)
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
			serviceCopy.Annotations[api.ServingCertErrorAnnotation] = err.Error()
			serviceCopy.Annotations[api.ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(serviceCopy) + 1)
			_, updateErr := sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

			// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
			if updateErr == nil && getNumFailures(serviceCopy) >= sc.maxRetries {
				return nil
			}
			return err
		}

		if actualSecret.Annotations[api.ServiceUIDAnnotation] != string(serviceCopy.UID) {
			serviceCopy.Annotations[api.ServingCertErrorAnnotation] = fmt.Sprintf("secret/%v references serviceUID %v, which does not match %v", actualSecret.Name, actualSecret.Annotations[api.ServiceUIDAnnotation], serviceCopy.UID)
			serviceCopy.Annotations[api.ServingCertErrorNumAnnotation] = strconv.Itoa(getNumFailures(serviceCopy) + 1)
			_, updateErr := sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

			// if we're past the max retries and we successfully updated, then the sync loop successfully handled this service and we want to forget it
			if updateErr == nil && getNumFailures(serviceCopy) >= sc.maxRetries {
				return nil
			}
			return errors.New(serviceCopy.Annotations[api.ServingCertErrorAnnotation])
		}
	}

	serviceCopy.Annotations[api.ServingCertCreatedByAnnotation] = sc.commonName()
	delete(serviceCopy.Annotations, api.ServingCertErrorAnnotation)
	delete(serviceCopy.Annotations, api.ServingCertErrorNumAnnotation)
	_, err = sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

	return err
}

func getNumFailures(service *v1.Service) int {
	numFailuresString := service.Annotations[api.ServingCertErrorNumAnnotation]
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
	// check the secret since it could not have been created yet
	secretName := service.Annotations[api.ServingCertSecretAnnotation]
	if len(secretName) == 0 {
		return false
	}
	_, err := sc.secretLister.Secrets(service.Namespace).Get(secretName)
	if kapierrors.IsNotFound(err) {
		// we have not created the secret yet
		return true
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get the secret %s/%s: %v", service.Namespace, secretName, err))
		return false
	}

	// check to see if the service was updated by us
	if service.Annotations[api.ServingCertCreatedByAnnotation] == sc.commonName() {
		return false
	}
	// we have failed too many times on this service, give up
	if getNumFailures(service) >= sc.maxRetries {
		return false
	}

	// the secret exists but the service was either not updated to include the correct created
	// by annotation or it does not match what we expect (i.e. the certificate has been rotated)
	return true
}

func (sc *ServiceServingCertController) commonName() string {
	return sc.ca.Config.Certs[0].Subject.CommonName
}

func ownerRef(service *v1.Service) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       service.Name,
		UID:        service.UID,
	}
}

func toBaseSecret(service *v1.Service) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Annotations[api.ServingCertSecretAnnotation],
			Namespace: service.Namespace,
			Annotations: map[string]string{
				api.ServiceUIDAnnotation:  string(service.UID),
				api.ServiceNameAnnotation: service.Name,
			},
		},
		Type: v1.SecretTypeTLS,
	}
}

func toRequiredSecret(dnsSuffix string, ca *crypto.CA, service *v1.Service, secretCopy *v1.Secret) error {
	dnsName := service.Name + "." + service.Namespace + ".svc"
	fqDNSName := dnsName + "." + dnsSuffix
	certificateLifetime := 365 * 2 // 2 years
	servingCert, err := ca.MakeServerCert(
		sets.NewString(dnsName, fqDNSName),
		certificateLifetime,
		cryptoextensions.ServiceServerCertificateExtensionV1(service),
	)
	if err != nil {
		return err
	}
	certBytes, keyBytes, err := servingCert.GetPEMBytes()
	if err != nil {
		return err
	}

	if secretCopy.Annotations == nil {
		secretCopy.Annotations = map[string]string{}
	}
	if secretCopy.Data == nil {
		secretCopy.Data = map[string][]byte{}
	}

	secretCopy.Annotations[api.ServingCertExpiryAnnotation] = servingCert.Certs[0].NotAfter.Format(time.RFC3339)
	secretCopy.Data[v1.TLSCertKey] = certBytes
	secretCopy.Data[v1.TLSPrivateKeyKey] = keyBytes

	ocontroller.EnsureOwnerRef(secretCopy, ownerRef(service))

	return nil
}

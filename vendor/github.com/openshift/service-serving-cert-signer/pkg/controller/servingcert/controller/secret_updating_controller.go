package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	ocontroller "github.com/openshift/library-go/pkg/controller"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

type ServiceServingCertUpdateController struct {
	secretClient kcoreclient.SecretsGetter

	serviceLister listers.ServiceLister
	secretLister  listers.SecretLister

	ca        *crypto.CA
	dnsSuffix string
	// minTimeLeftForCert is how much time is remaining for the serving cert before regenerating it.
	minTimeLeftForCert time.Duration

	// secrets that need to be checked
	queue workqueue.RateLimitingInterface

	// standard controller loop
	*controller.Controller
}

func NewServiceServingCertUpdateController(services informers.ServiceInformer, secrets informers.SecretInformer, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertUpdateController {
	sc := &ServiceServingCertUpdateController{
		secretClient:  secretClient,
		serviceLister: services.Lister(),
		secretLister:  secrets.Lister(),

		ca:        ca,
		dnsSuffix: dnsSuffix,
		// TODO base the expiry time on a percentage of the time for the lifespan of the cert
		minTimeLeftForCert: 1 * time.Hour,
	}

	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.addSecret,
			UpdateFunc: sc.updateSecret,
		},
		resyncInterval,
	)

	internalController, queue := controller.New("ServiceServingCertUpdateController", sc.syncSecret,
		services.Informer().GetController().HasSynced, secrets.Informer().GetController().HasSynced)

	sc.Controller = internalController
	sc.queue = queue

	return sc
}

func (sc *ServiceServingCertUpdateController) enqueueSecret(obj *v1.Secret) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	sc.queue.Add(key)
}

func (sc *ServiceServingCertUpdateController) addSecret(obj interface{}) {
	secret := obj.(*v1.Secret)
	if _, ok := toServiceName(secret); !ok {
		return
	}

	glog.V(4).Infof("adding %s", secret.Name)
	sc.enqueueSecret(secret)
}

func (sc *ServiceServingCertUpdateController) updateSecret(old, cur interface{}) {
	secret := cur.(*v1.Secret)
	if _, ok := toServiceName(secret); !ok {
		// if the current doesn't have a service name, check the old
		secret = old.(*v1.Secret)
		if _, ok := toServiceName(secret); !ok {
			return
		}
	}

	glog.V(4).Infof("updating %s", secret.Name)
	sc.enqueueSecret(secret)
}

func (sc *ServiceServingCertUpdateController) syncSecret(obj interface{}) error {
	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	sharedSecret, err := sc.secretLister.Secrets(namespace).Get(name)
	if kapierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	regenerate, service := sc.requiresRegeneration(sharedSecret)
	if !regenerate {
		return nil
	}

	// make a copy to avoid mutating cache state
	secretCopy := sharedSecret.DeepCopy()

	if err := toRequiredSecret(sc.dnsSuffix, sc.ca, service, secretCopy); err != nil {
		return err
	}

	_, err = sc.secretClient.Secrets(secretCopy.Namespace).Update(secretCopy)
	return err
}

func (sc *ServiceServingCertUpdateController) requiresRegeneration(secret *v1.Secret) (bool, *v1.Service) {
	serviceName, ok := toServiceName(secret)
	if !ok {
		return false, nil
	}

	sharedService, err := sc.serviceLister.Services(secret.Namespace).Get(serviceName)
	if kapierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get service %s/%s: %v", secret.Namespace, serviceName, err))
		return false, nil
	}

	if sharedService.Annotations[api.ServingCertSecretAnnotation] != secret.Name {
		return false, nil
	}
	if secret.Annotations[api.ServiceUIDAnnotation] != string(sharedService.UID) {
		return false, nil
	}

	// if we don't have an ownerref, just go ahead and regenerate.  It's easier than writing a
	// secondary logic flow.
	if !ocontroller.HasOwnerRef(secret, ownerRef(sharedService)) {
		return true, sharedService
	}

	// if we don't have the annotation for expiry, just go ahead and regenerate.  It's easier than writing a
	// secondary logic flow that creates the expiry dates
	expiryString, ok := secret.Annotations[api.ServingCertExpiryAnnotation]
	if !ok {
		return true, sharedService
	}
	expiry, err := time.Parse(time.RFC3339, expiryString)
	if err != nil {
		return true, sharedService
	}

	if time.Now().Add(sc.minTimeLeftForCert).After(expiry) {
		return true, sharedService
	}

	return false, nil
}

func toServiceName(secret *v1.Secret) (string, bool) {
	serviceName := secret.Annotations[api.ServiceNameAnnotation]
	return serviceName, len(serviceName) != 0
}

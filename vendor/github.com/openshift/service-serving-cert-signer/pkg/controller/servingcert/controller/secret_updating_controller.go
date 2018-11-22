package controller

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"

	ocontroller "github.com/openshift/library-go/pkg/controller"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

type serviceServingCertUpdateController struct {
	secretClient kcoreclient.SecretsGetter

	serviceLister listers.ServiceLister
	secretLister  listers.SecretLister

	ca        *crypto.CA
	dnsSuffix string
	// minTimeLeftForCert is how much time is remaining for the serving cert before regenerating it.
	minTimeLeftForCert time.Duration
}

func NewServiceServingCertUpdateController(services informers.ServiceInformer, secrets informers.SecretInformer, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string) controller.Runner {
	sc := &serviceServingCertUpdateController{
		secretClient:  secretClient,
		serviceLister: services.Lister(),
		secretLister:  secrets.Lister(),

		ca:        ca,
		dnsSuffix: dnsSuffix,
		// TODO base the expiry time on a percentage of the time for the lifespan of the cert
		minTimeLeftForCert: 1 * time.Hour,
	}

	return controller.New("ServiceServingCertUpdateController", sc,
		controller.WithInformerSynced(services),
		controller.WithInformer(secrets, controller.FilterFuncs{
			AddFunc:    sc.addSecret,
			UpdateFunc: sc.updateSecret,
		}),
	)
}

func (sc *serviceServingCertUpdateController) addSecret(obj metav1.Object) bool {
	secret := obj.(*v1.Secret)
	_, ok := toServiceName(secret)
	return ok
}

func (sc *serviceServingCertUpdateController) updateSecret(old, cur metav1.Object) bool {
	// if the current doesn't have a service name, check the old
	// TODO drop this
	return sc.addSecret(cur) || sc.addSecret(old)
}

func (sc *serviceServingCertUpdateController) Key(namespace, name string) (metav1.Object, error) {
	return sc.secretLister.Secrets(namespace).Get(name)
}

func (sc *serviceServingCertUpdateController) Sync(obj metav1.Object) error {
	sharedSecret := obj.(*v1.Secret)

	regenerate, service := sc.requiresRegeneration(sharedSecret)
	if !regenerate {
		return nil
	}

	// make a copy to avoid mutating cache state
	secretCopy := sharedSecret.DeepCopy()

	if err := toRequiredSecret(sc.dnsSuffix, sc.ca, service, secretCopy); err != nil {
		return err
	}

	_, err := sc.secretClient.Secrets(secretCopy.Namespace).Update(secretCopy)
	return err
}

func (sc *serviceServingCertUpdateController) requiresRegeneration(secret *v1.Secret) (bool, *v1.Service) {
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

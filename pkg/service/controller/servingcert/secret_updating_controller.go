package servingcert

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/crypto/extensions"
	ocontroller "github.com/openshift/origin/pkg/controller"
)

// ServiceServingCertUpdateController is responsible for synchronizing Service objects stored
// in the system with actual running replica sets and pods.
type ServiceServingCertUpdateController struct {
	secretClient kcoreclient.SecretsGetter

	// Services that need to be checked
	queue workqueue.RateLimitingInterface

	serviceLister    listers.ServiceLister
	serviceHasSynced cache.InformerSynced

	secretLister    listers.SecretLister
	secretHasSynced cache.InformerSynced

	ca        *crypto.CA
	dnsSuffix string
	// minTimeLeftForCert is how much time is remaining for the serving cert before regenerating it.
	minTimeLeftForCert time.Duration

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// NewServiceServingCertUpdateController creates a new ServiceServingCertUpdateController.
// TODO this should accept a shared informer
func NewServiceServingCertUpdateController(services informers.ServiceInformer, secrets informers.SecretInformer, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string, resyncInterval time.Duration) *ServiceServingCertUpdateController {
	sc := &ServiceServingCertUpdateController{
		secretClient: secretClient,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		ca:        ca,
		dnsSuffix: dnsSuffix,
		// TODO base the expiry time on a percentage of the time for the lifespan of the cert
		minTimeLeftForCert: 1 * time.Hour,
	}

	sc.serviceLister = services.Lister()
	services.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{},
		resyncInterval,
	)
	sc.serviceHasSynced = services.Informer().GetController().HasSynced

	sc.secretLister = secrets.Lister()
	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.addSecret,
			UpdateFunc: sc.updateSecret,
		},
		resyncInterval,
	)
	sc.secretHasSynced = secrets.Informer().GetController().HasSynced

	sc.syncHandler = sc.syncSecret

	return sc
}

// Run begins watching and syncing.
func (sc *ServiceServingCertUpdateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer sc.queue.ShutDown()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, sc.serviceHasSynced, sc.secretHasSynced) {
		return
	}

	glog.V(5).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(sc.runWorker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
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
	secret := obj.(*v1.Secret)
	if len(secret.Annotations[ServiceNameAnnotation]) == 0 {
		return
	}

	glog.V(4).Infof("adding %s", secret.Name)
	sc.enqueueSecret(secret)
}

func (sc *ServiceServingCertUpdateController) updateSecret(old, cur interface{}) {
	secret := cur.(*v1.Secret)
	if len(secret.Annotations[ServiceNameAnnotation]) == 0 {
		// if the current doesn't have a service name, check the old
		secret = old.(*v1.Secret)
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

	dnsName := service.Name + "." + secretCopy.Namespace + ".svc"
	fqDNSName := dnsName + "." + sc.dnsSuffix
	certificateLifetime := 365 * 2 // 2 years
	servingCert, err := sc.ca.MakeServerCert(
		sets.NewString(dnsName, fqDNSName),
		certificateLifetime,
		extensions.ServiceServerCertificateExtensionV1(service),
	)
	if err != nil {
		return err
	}
	secretCopy.Annotations[ServingCertExpiryAnnotation] = servingCert.Certs[0].NotAfter.Format(time.RFC3339)
	secretCopy.Data[v1.TLSCertKey], secretCopy.Data[v1.TLSPrivateKeyKey], err = servingCert.GetPEMBytes()
	if err != nil {
		return err
	}
	ocontroller.EnsureOwnerRef(secretCopy, ownerRef(service))

	_, err = sc.secretClient.Secrets(secretCopy.Namespace).Update(secretCopy)
	return err
}

func (sc *ServiceServingCertUpdateController) requiresRegeneration(secret *v1.Secret) (bool, *v1.Service) {
	serviceName := secret.Annotations[ServiceNameAnnotation]
	if len(serviceName) == 0 {
		return false, nil
	}

	sharedService, err := sc.serviceLister.Services(secret.Namespace).Get(serviceName)
	if kapierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get service %s/%s: %v", secret.Namespace, secret.Annotations[ServiceNameAnnotation], err))
		return false, nil
	}

	if sharedService.Annotations[ServingCertSecretAnnotation] != secret.Name {
		return false, nil
	}
	if secret.Annotations[ServiceUIDAnnotation] != string(sharedService.UID) {
		return false, nil
	}

	// if we don't have an ownerref, just go ahead and regenerate.  It's easier than writing a
	// secondary logic flow.
	if !ocontroller.HasOwnerRef(secret, ownerRef(sharedService)) {
		return true, sharedService
	}

	// if we don't have the annotation for expiry, just go ahead and regenerate.  It's easier than writing a
	// secondary logic flow that creates the expiry dates
	expiryString, ok := secret.Annotations[ServingCertExpiryAnnotation]
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

package controllers

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// DockercfgTokenDeletedControllerOptions contains options for the DockercfgTokenDeletedController
type DockercfgTokenDeletedControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list secrets.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

// NewDockercfgTokenDeletedController returns a new *DockercfgTokenDeletedController.
func NewDockercfgTokenDeletedController(secrets informers.SecretInformer, cl kclientset.Interface, options DockercfgTokenDeletedControllerOptions) *DockercfgTokenDeletedController {
	e := &DockercfgTokenDeletedController{
		client: cl,
	}

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
				DeleteFunc: e.secretDeleted,
			},
		},
		options.Resync,
	)

	return e
}

// The DockercfgTokenDeletedController watches for service account tokens to be deleted.
// On delete, it removes the associated dockercfg secret if it exists.
type DockercfgTokenDeletedController struct {
	client           kclientset.Interface
	secretController cache.Controller
}

// Runs controller loops and returns on shutdown
func (e *DockercfgTokenDeletedController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, e.secretController.HasSynced) {
		return
	}

	glog.V(5).Infof("Worker started")
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

// secretDeleted reacts to a token secret being deleted by looking for a corresponding dockercfg secret and deleting it if it exists
func (e *DockercfgTokenDeletedController) secretDeleted(obj interface{}) {
	tokenSecret, ok := obj.(*v1.Secret)
	if !ok {
		return
	}

	dockercfgSecrets, err := e.findDockercfgSecrets(tokenSecret)
	if err != nil {
		glog.Error(err)
		return
	}
	if len(dockercfgSecrets) == 0 {
		return
	}

	// remove the reference token secrets
	for _, dockercfgSecret := range dockercfgSecrets {
		if err := e.client.Core().Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name, nil); (err != nil) && !apierrors.IsNotFound(err) {
			utilruntime.HandleError(err)
		}
	}
}

// findDockercfgSecret checks all the secrets in the namespace to see if the token secret has any existing dockercfg secrets that reference it
func (e *DockercfgTokenDeletedController) findDockercfgSecrets(tokenSecret *v1.Secret) ([]*v1.Secret, error) {
	dockercfgSecrets := []*v1.Secret{}

	options := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector(api.SecretTypeField, string(v1.SecretTypeDockercfg)).String()}
	potentialSecrets, err := e.client.Core().Secrets(tokenSecret.Namespace).List(options)
	if err != nil {
		return nil, err
	}

	for i, currSecret := range potentialSecrets.Items {
		if currSecret.Annotations[ServiceAccountTokenSecretNameKey] == tokenSecret.Name {
			dockercfgSecrets = append(dockercfgSecrets, &potentialSecrets.Items[i])
		}
	}

	return dockercfgSecrets, nil
}

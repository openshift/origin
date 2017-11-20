package controllers

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// NumServiceAccountUpdateRetries controls the number of times we will retry on conflict errors.
// This happens when multiple service account controllers update at the same time.
const NumServiceAccountUpdateRetries = 10

// DockercfgDeletedControllerOptions contains options for the DockercfgDeletedController
type DockercfgDeletedControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list secrets.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

// NewDockercfgDeletedController returns a new *DockercfgDeletedController.
func NewDockercfgDeletedController(secrets informers.SecretInformer, cl kclientset.Interface, options DockercfgDeletedControllerOptions) *DockercfgDeletedController {
	e := &DockercfgDeletedController{
		client: cl,
	}

	e.secretController = secrets.Informer().GetController()
	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Secret:
					return t.Type == v1.SecretTypeDockercfg
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

// The DockercfgDeletedController watches for service account dockercfg secrets to be deleted
// It removes the corresponding token secret and service account references.
type DockercfgDeletedController struct {
	client           kclientset.Interface
	secretController cache.Controller
}

// Run processes the queue.
func (e *DockercfgDeletedController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, e.secretController.HasSynced) {
		return
	}

	glog.V(5).Infof("Worker started")
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

// secretDeleted reacts to a Secret being deleted by looking to see if it's a dockercfg secret for a service account, in which case it
// it removes the references from the service account and removes the token created to back the dockercfgSecret
func (e *DockercfgDeletedController) secretDeleted(obj interface{}) {
	dockercfgSecret, ok := obj.(*v1.Secret)
	if !ok {
		return
	}
	if _, exists := dockercfgSecret.Annotations[ServiceAccountTokenSecretNameKey]; !exists {
		return
	}

	for i := 1; i <= NumServiceAccountUpdateRetries; i++ {
		if err := e.removeDockercfgSecretReference(dockercfgSecret); err != nil {
			if kapierrors.IsConflict(err) && i < NumServiceAccountUpdateRetries {
				time.Sleep(wait.Jitter(100*time.Millisecond, 0.0))
				continue
			}

			glog.Error(err)
			break
		}

		break
	}

	// remove the reference token secret
	if err := e.client.Core().Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Annotations[ServiceAccountTokenSecretNameKey], nil); (err != nil) && !kapierrors.IsNotFound(err) {
		utilruntime.HandleError(err)
	}
}

// removeDockercfgSecretReference updates the given ServiceAccount to remove ImagePullSecret and Secret references
func (e *DockercfgDeletedController) removeDockercfgSecretReference(dockercfgSecret *v1.Secret) error {
	serviceAccount, err := e.getServiceAccount(dockercfgSecret)
	if kapierrors.IsNotFound(err) {
		// if the service account is gone, no work to do
		return nil
	}
	if err != nil {
		return err
	}

	changed := false

	secrets := []v1.ObjectReference{}
	for _, s := range serviceAccount.Secrets {
		if s.Name == dockercfgSecret.Name {
			changed = true
			continue
		}

		secrets = append(secrets, s)
	}
	serviceAccount.Secrets = secrets

	imagePullSecrets := []v1.LocalObjectReference{}
	for _, s := range serviceAccount.ImagePullSecrets {
		if s.Name == dockercfgSecret.Name {
			changed = true
			continue
		}

		imagePullSecrets = append(imagePullSecrets, s)
	}
	serviceAccount.ImagePullSecrets = imagePullSecrets

	if changed {
		_, err = e.client.Core().ServiceAccounts(dockercfgSecret.Namespace).Update(serviceAccount)
		if err != nil {
			return err
		}
	}

	return nil
}

// getServiceAccount returns the ServiceAccount referenced by the given secret.  return nil, but no error if the secret doesn't reference a service account
func (e *DockercfgDeletedController) getServiceAccount(secret *v1.Secret) (*v1.ServiceAccount, error) {
	saName, saUID := secret.Annotations[v1.ServiceAccountNameKey], secret.Annotations[v1.ServiceAccountUIDKey]
	if len(saName) == 0 || len(saUID) == 0 {
		return nil, nil
	}

	serviceAccount, err := e.client.Core().ServiceAccounts(secret.Namespace).Get(saName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if saUID != string(serviceAccount.UID) {
		return nil, fmt.Errorf("secret (%v) service account UID (%v) does not match service account (%v) UID (%v)", secret.Name, saUID, serviceAccount.Name, serviceAccount.UID)
	}
	return serviceAccount, nil
}

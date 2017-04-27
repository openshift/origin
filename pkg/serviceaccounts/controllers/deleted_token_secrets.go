package controllers

import (
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// DockercfgTokenDeletedControllerOptions contains options for the DockercfgTokenDeletedController
type DockercfgTokenDeletedControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list secrets.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

// NewDockercfgTokenDeletedController returns a new *DockercfgTokenDeletedController.
// See pkg/cmd/server/bootstrappolicy/controller_policy.go to see what cluster roles apply to this controller.
func NewDockercfgTokenDeletedController(cl kclientset.Interface, options DockercfgTokenDeletedControllerOptions) *DockercfgTokenDeletedController {
	e := &DockercfgTokenDeletedController{
		client: cl,
	}

	dockercfgSelector := fields.OneTermEqualSelector(api.SecretTypeField, string(api.SecretTypeServiceAccountToken))
	_, e.secretController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				opts := api.ListOptions{FieldSelector: dockercfgSelector}
				return e.client.Core().Secrets(api.NamespaceAll).List(opts)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				opts := api.ListOptions{FieldSelector: dockercfgSelector, ResourceVersion: options.ResourceVersion}
				return e.client.Core().Secrets(api.NamespaceAll).Watch(opts)
			},
		},
		&api.Secret{},
		options.Resync,
		cache.ResourceEventHandlerFuncs{
			DeleteFunc: e.secretDeleted,
		},
	)

	return e
}

// The DockercfgTokenDeletedController watches for service account tokens to be deleted.
// On delete, it removes the associated dockercfg secret if it exists.
type DockercfgTokenDeletedController struct {
	client kclientset.Interface

	secretController *cache.Controller
}

// Run should always block forever.  The caller should execute it in a different go routine.
func (e *DockercfgTokenDeletedController) Run(stopCh <-chan struct{}) {
	e.secretController.Run(stopCh)
}

// secretDeleted reacts to a token secret being deleted by looking for a corresponding dockercfg secret and deleting it if it exists
func (e *DockercfgTokenDeletedController) secretDeleted(obj interface{}) {
	tokenSecret, ok := obj.(*api.Secret)
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
func (e *DockercfgTokenDeletedController) findDockercfgSecrets(tokenSecret *api.Secret) ([]*api.Secret, error) {
	dockercfgSecrets := []*api.Secret{}

	options := api.ListOptions{FieldSelector: fields.OneTermEqualSelector(api.SecretTypeField, string(api.SecretTypeDockercfg))}
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

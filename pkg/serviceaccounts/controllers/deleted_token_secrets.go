package controllers

import (
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"
)

// DockercfgTokenDeletedControllerOptions contains options for the DockercfgTokenDeletedController
type DockercfgTokenDeletedControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list secrets.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

// NewDockercfgTokenDeletedController returns a new *DockercfgTokenDeletedController.
func NewDockercfgTokenDeletedController(cl client.Interface, options DockercfgTokenDeletedControllerOptions) *DockercfgTokenDeletedController {
	e := &DockercfgTokenDeletedController{
		client: cl,
	}

	dockercfgSelector := fields.OneTermEqualSelector(client.SecretType, string(api.SecretTypeServiceAccountToken))
	_, e.secretController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return e.client.Secrets(api.NamespaceAll).List(labels.Everything(), dockercfgSelector)
			},
			WatchFunc: func(rv string) (watch.Interface, error) {
				return e.client.Secrets(api.NamespaceAll).Watch(labels.Everything(), dockercfgSelector, rv)
			},
		},
		&api.Secret{},
		options.Resync,
		framework.ResourceEventHandlerFuncs{
			DeleteFunc: e.secretDeleted,
		},
	)

	return e
}

// The DockercfgTokenDeletedController watches for service account tokens to be deleted.
// On delete, it removes the associated dockercfg secret if it exists.
type DockercfgTokenDeletedController struct {
	stopChan chan struct{}

	client client.Interface

	secretController *framework.Controller
}

// Runs controller loops and returns immediately
func (e *DockercfgTokenDeletedController) Run() {
	if e.stopChan == nil {
		e.stopChan = make(chan struct{})
		go e.secretController.Run(e.stopChan)
	}
}

// Stop gracefully shuts down this controller
func (e *DockercfgTokenDeletedController) Stop() {
	if e.stopChan != nil {
		close(e.stopChan)
		e.stopChan = nil
	}
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
		if err := e.client.Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name); (err != nil) && !apierrors.IsNotFound(err) {
			util.HandleError(err)
		}
	}
}

// findDockercfgSecret checks all the secrets in the namespace to see if the token secret has any existing dockercfg secrets that reference it
func (e *DockercfgTokenDeletedController) findDockercfgSecrets(tokenSecret *api.Secret) ([]*api.Secret, error) {
	dockercfgSecrets := []*api.Secret{}

	dockercfgSelector := fields.OneTermEqualSelector(client.SecretType, string(api.SecretTypeDockercfg))
	potentialSecrets, err := e.client.Secrets(tokenSecret.Namespace).List(labels.Everything(), dockercfgSelector)
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

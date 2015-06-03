package controllers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller/framework"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/credentialprovider"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/secret"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/util/namer"
)

const ServiceAccountTokenSecretNameKey = "openshift.io/token-secret.name"

// DockercfgControllerOptions contains options for the DockercfgController
type DockercfgControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list service accounts.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	DefaultDockerURL string
}

// NewDockercfgController returns a new *DockercfgController.
func NewDockercfgController(cl client.Interface, options DockercfgControllerOptions) *DockercfgController {
	e := &DockercfgController{
		client: cl,
	}

	_, e.serviceAccountController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return e.client.ServiceAccounts(api.NamespaceAll).List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(rv string) (watch.Interface, error) {
				return e.client.ServiceAccounts(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), rv)
			},
		},
		&api.ServiceAccount{},
		options.Resync,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    e.serviceAccountAdded,
			UpdateFunc: e.serviceAccountUpdated,
		},
	)

	e.dockerURL = options.DefaultDockerURL

	return e
}

// DockercfgController manages dockercfg secrets for ServiceAccount objects
type DockercfgController struct {
	stopChan chan struct{}

	client client.Interface

	dockerURL     string
	dockerURLLock sync.Mutex

	serviceAccountController *framework.Controller
}

// Runs controller loops and returns immediately
func (e *DockercfgController) Run() {
	if e.stopChan == nil {
		e.stopChan = make(chan struct{})
		go e.serviceAccountController.Run(e.stopChan)
	}
}

// Stop gracefully shuts down this controller
func (e *DockercfgController) Stop() {
	if e.stopChan != nil {
		close(e.stopChan)
		e.stopChan = nil
	}
}

func (e *DockercfgController) SetDockerURL(newDockerURL string) {
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	e.dockerURL = newDockerURL
}

// serviceAccountAdded reacts to a ServiceAccount creation by creating a corresponding ServiceAccountToken Secret
func (e *DockercfgController) serviceAccountAdded(obj interface{}) {
	serviceAccount := obj.(*api.ServiceAccount)

	if err := e.createDockercfgSecretIfNeeded(serviceAccount); err != nil {
		glog.Error(err)
	}
}

// serviceAccountUpdated reacts to a ServiceAccount update (or re-list) by ensuring a corresponding ServiceAccountToken Secret exists
func (e *DockercfgController) serviceAccountUpdated(oldObj interface{}, newObj interface{}) {
	newServiceAccount := newObj.(*api.ServiceAccount)

	if err := e.createDockercfgSecretIfNeeded(newServiceAccount); err != nil {
		glog.Error(err)
	}
}

const (
	// These constants are here to create a name that is short enough to survive chopping by generate name
	maxNameLength             = 63
	randomLength              = 5
	maxSecretPrefixNameLength = maxNameLength - randomLength
)

func getDockercfgSecretNamePrefix(serviceAccount *api.ServiceAccount) string {
	return namer.GetName(serviceAccount.Name, "dockercfg-", maxSecretPrefixNameLength)
}
func getTokenSecretNamePrefix(serviceAccount *api.ServiceAccount) string {
	return namer.GetName(serviceAccount.Name, "token-", maxSecretPrefixNameLength)
}

// createDockercfgSecretIfNeeded makes sure at least one ServiceAccountToken secret exists, and is included in the serviceAccount's Secrets list
func (e *DockercfgController) createDockercfgSecretIfNeeded(serviceAccount *api.ServiceAccount) error {

	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(serviceAccount)

	// look for an ImagePullSecret in the form
	foundPullSecret := len(imageDockercfgPullSecrets) > 0
	foundMountableSecret := len(mountableDockercfgSecrets) > 0

	switch {
	// if we already have a docker pull secret, simply return
	case foundPullSecret && foundMountableSecret:
		return nil

	case foundPullSecret && !foundMountableSecret, !foundPullSecret && foundMountableSecret:
		dockercfgSecretName := ""
		switch {
		case foundPullSecret:
			dockercfgSecretName = imageDockercfgPullSecrets.List()[0]
		case foundMountableSecret:
			dockercfgSecretName = mountableDockercfgSecrets.List()[0]
		}

		err := e.createDockerPullSecretReference(serviceAccount, dockercfgSecretName)
		if kapierrors.IsConflict(err) {
			// nothing to do.  Our choice was stale or we got a conflict.  Either way that means that the service account was updated.  We simply need to return because we'll get an update notification later
			return nil
		}

		return err

	}

	// if we get here, then we need to create a new dockercfg secret
	// before we do expensive things, make sure our view of the service account is up to date
	if liveServiceAccount, err := e.client.ServiceAccounts(serviceAccount.Namespace).Get(serviceAccount.Name); err != nil {
		return err
	} else if liveServiceAccount.ResourceVersion != serviceAccount.ResourceVersion {
		// our view of the service account is not up to date
		// we'll get notified of an update event later and get to try again
		// this only prevent interactions between successive runs of this controller's event handlers, but that is useful
		glog.V(2).Infof("View of ServiceAccount %s/%s is not up to date, skipping dockercfg creation", serviceAccount.Namespace, serviceAccount.Name)
		return nil
	}

	dockercfgSecret, err := e.createDockerPullSecret(serviceAccount)
	if err != nil {
		return err
	}

	err = e.createDockerPullSecretReference(serviceAccount, dockercfgSecret.Name)
	if kapierrors.IsConflict(err) {
		// nothing to do.  Our choice was stale or we got a conflict.  Either way that means that the service account was updated.  We simply need to return because we'll get an update notification later
		// we do need to clean up our dockercfgSecret.  token secrets are cleaned up by the controller handling service account dockercfg secret deletes
		glog.V(2).Infof("Deleting secret %s/%s (err=%v)", dockercfgSecret.Namespace, dockercfgSecret.Name, err)
		if err := e.client.Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name); err != nil {
			glog.Error(err)
		}
		return nil
	}

	return err
}

// createDockerPullSecretReference updates a service account to reference the dockercfgSecret as a Secret and an ImagePullSecret
func (e *DockercfgController) createDockerPullSecretReference(staleServiceAccount *api.ServiceAccount, dockercfgSecretName string) error {
	liveServiceAccount, err := e.client.ServiceAccounts(staleServiceAccount.Namespace).Get(staleServiceAccount.Name)
	if err != nil {
		return err
	}

	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(liveServiceAccount)
	staleDockercfgMountableSecrets, staleImageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(staleServiceAccount)

	// if we're trying to create a reference based on stale lists of dockercfg secrets, let the caller know
	if !reflect.DeepEqual(staleDockercfgMountableSecrets.List(), mountableDockercfgSecrets.List()) || !reflect.DeepEqual(staleImageDockercfgPullSecrets.List(), imageDockercfgPullSecrets.List()) {
		return kapierrors.NewConflict("ServiceAccount", staleServiceAccount.Name, fmt.Errorf("cannot add reference to %s based on stale data.  decision made for %v,%v, but live version is %v,%v", dockercfgSecretName, staleDockercfgMountableSecrets.List(), staleImageDockercfgPullSecrets.List(), mountableDockercfgSecrets.List(), imageDockercfgPullSecrets.List()))
	}

	changed := false
	if !mountableDockercfgSecrets.Has(dockercfgSecretName) {
		liveServiceAccount.Secrets = append(liveServiceAccount.Secrets, api.ObjectReference{Name: dockercfgSecretName})
		changed = true
	}

	if !imageDockercfgPullSecrets.Has(dockercfgSecretName) {
		liveServiceAccount.ImagePullSecrets = append(liveServiceAccount.ImagePullSecrets, api.LocalObjectReference{Name: dockercfgSecretName})
		changed = true
	}

	if changed {
		if _, err = e.client.ServiceAccounts(liveServiceAccount.Namespace).Update(liveServiceAccount); err != nil {
			// TODO: retry on API conflicts in case the conflict was unrelated to our generated dockercfg secrets?
			return err
		}
	}
	return nil
}

const (
	tokenSecretWaitInterval = 20 * time.Millisecond
	tokenSecretWaitTimes    = 100
)

// createTokenSecret creates a token secret for a given service account.  Returns the name of the token
func (e *DockercfgController) createTokenSecret(serviceAccount *api.ServiceAccount) (*api.Secret, error) {
	tokenSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      secret.Strategy.GenerateName(getTokenSecretNamePrefix(serviceAccount)),
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				api.ServiceAccountNameKey: serviceAccount.Name,
				api.ServiceAccountUIDKey:  string(serviceAccount.UID),
			},
		},
		Type: api.SecretTypeServiceAccountToken,
		Data: map[string][]byte{},
	}

	_, err := e.client.Secrets(tokenSecret.Namespace).Create(tokenSecret)
	if err != nil {
		return nil, err
	}

	// now we have to wait for the service account token controller to make this valid
	// TODO remove this once we have a create-token endpoint
	for i := 0; i <= tokenSecretWaitTimes; i++ {
		liveTokenSecret, err := e.client.Secrets(tokenSecret.Namespace).Get(tokenSecret.Name)
		if err != nil {
			return nil, err
		}

		if len(liveTokenSecret.Data[api.ServiceAccountTokenKey]) > 0 {
			return liveTokenSecret, nil
		}

		time.Sleep(wait.Jitter(tokenSecretWaitInterval, 0.0))

	}

	// the token wasn't ever created, attempt deletion
	glog.Warningf("Deleting unfilled token secret %s/%s", tokenSecret.Namespace, tokenSecret.Name)
	if err := e.client.Secrets(tokenSecret.Namespace).Delete(tokenSecret.Name); err != nil {
		glog.Error(err)
	}
	return nil, fmt.Errorf("token never generated for %s", tokenSecret.Name)
}

// createDockerPullSecret creates a dockercfg secret based on the token secret
func (e *DockercfgController) createDockerPullSecret(serviceAccount *api.ServiceAccount) (*api.Secret, error) {
	tokenSecret, err := e.createTokenSecret(serviceAccount)
	if err != nil {
		return nil, err
	}

	dockercfgSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      secret.Strategy.GenerateName(getDockercfgSecretNamePrefix(serviceAccount)),
			Namespace: tokenSecret.Namespace,
			Annotations: map[string]string{
				api.ServiceAccountNameKey:        serviceAccount.Name,
				api.ServiceAccountUIDKey:         string(serviceAccount.UID),
				ServiceAccountTokenSecretNameKey: string(tokenSecret.Name),
			},
		},
		Type: api.SecretTypeDockercfg,
		Data: map[string][]byte{},
	}

	// prevent updating the DockerURL until we've created the secret
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	dockercfg := &credentialprovider.DockerConfig{
		e.dockerURL: credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: string(tokenSecret.Data[api.ServiceAccountTokenKey]),
			Email:    "serviceaccount@example.org",
		},
	}
	dockercfgContent, err := json.Marshal(dockercfg)
	if err != nil {
		return nil, err
	}
	dockercfgSecret.Data[api.DockerConfigKey] = dockercfgContent

	// Save the secret
	createdSecret, err := e.client.Secrets(tokenSecret.Namespace).Create(dockercfgSecret)

	return createdSecret, err
}

func getSecretReferences(serviceAccount *api.ServiceAccount) util.StringSet {
	references := util.NewStringSet()
	for _, secret := range serviceAccount.Secrets {
		references.Insert(secret.Name)
	}
	return references
}

func getGeneratedDockercfgSecretNames(serviceAccount *api.ServiceAccount) (util.StringSet, util.StringSet) {
	mountableDockercfgSecrets := util.StringSet{}
	imageDockercfgPullSecrets := util.StringSet{}

	secretNamePrefix := getDockercfgSecretNamePrefix(serviceAccount)

	for _, s := range serviceAccount.Secrets {
		if strings.HasPrefix(s.Name, secretNamePrefix) {
			mountableDockercfgSecrets.Insert(s.Name)
		}
	}
	for _, s := range serviceAccount.ImagePullSecrets {
		if strings.HasPrefix(s.Name, secretNamePrefix) {
			imageDockercfgPullSecrets.Insert(s.Name)
		}
	}
	return mountableDockercfgSecrets, imageDockercfgPullSecrets
}

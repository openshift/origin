package serviceaccounts

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/client"
)

// TokenRetriever defined an interface for getting an API token for a service account
type TokenRetriever interface {
	GetToken(serviceAccountNamespace, serviceAccountName string) (token string, err error)
}

// ClientLookupTokenRetriever uses its client to look up a service account token
type ClientLookupTokenRetriever struct {
	Client kclientsetinternal.Interface
}

// GetToken returns a token for the named service account or an error if none existed after a timeout
func (s *ClientLookupTokenRetriever) GetToken(namespace, name string) (string, error) {
	for i := 0; i < 30; i++ {
		// Wait on subsequent retries
		if i != 0 {
			time.Sleep(time.Second)
		}

		// Get the service account
		serviceAccount, err := s.Client.Core().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// Get the secrets
		// TODO: JTL: create one directly once we have that ability
		for _, secretRef := range serviceAccount.Secrets {
			secret, err2 := s.Client.Core().Secrets(namespace).Get(secretRef.Name, metav1.GetOptions{})
			if err2 != nil {
				// Tolerate fetch errors on a particular secret
				continue
			}
			if IsValidServiceAccountToken(serviceAccount, secret) {
				// Return a valid token
				return string(secret.Data[kapi.ServiceAccountTokenKey]), nil
			}
		}
	}

	return "", fmt.Errorf("Could not get token for %s/%s", namespace, name)
}

// Clients returns an OpenShift and Kubernetes client with the credentials of the named service account
// TODO: change return types to client.Interface/kclientset.Interface to allow auto-reloading credentials
func Clients(
	config restclient.Config,
	tokenRetriever TokenRetriever,
	namespace,
	name string,
) (*restclient.Config, *client.Client, kclientsetinternal.Interface, kclientsetexternal.Interface, error) {
	// Clear existing auth info
	config.Username = ""
	config.Password = ""
	config.CertFile = ""
	config.CertData = []byte{}
	config.KeyFile = ""
	config.KeyData = []byte{}
	config.BearerToken = ""

	kubeUserAgent := ""
	openshiftUserAgent := ""

	// they specified, don't mess with it
	if len(config.UserAgent) > 0 {
		kubeUserAgent = config.UserAgent
		openshiftUserAgent = config.UserAgent

	} else {
		kubeUserAgent = fmt.Sprintf("%s system:serviceaccount:%s:%s", restclient.DefaultKubernetesUserAgent(), namespace, name)
		openshiftUserAgent = fmt.Sprintf("%s system:serviceaccount:%s:%s", client.DefaultOpenShiftUserAgent(), namespace, name)
	}

	// For now, just initialize the token once
	// TODO: refetch the token if the client encounters 401 errors
	token, err := tokenRetriever.GetToken(namespace, name)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	config.BearerToken = token

	config.UserAgent = openshiftUserAgent
	c, err := client.New(&config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	config.UserAgent = kubeUserAgent
	internalKubeClientset, err := kclientsetinternal.NewForConfig(&config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	externalKubeClientset, err := kclientsetexternal.NewForConfig(&config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return &config, c, internalKubeClientset, externalKubeClientset, nil
}

// IsValidServiceAccountToken returns true if the given secret contains a service account token valid for the given service account
func IsValidServiceAccountToken(serviceAccount *kapi.ServiceAccount, secret *kapi.Secret) bool {
	if secret.Type != kapi.SecretTypeServiceAccountToken {
		return false
	}
	if secret.Namespace != serviceAccount.Namespace {
		return false
	}
	if secret.Annotations[kapi.ServiceAccountNameKey] != serviceAccount.Name {
		return false
	}
	if secret.Annotations[kapi.ServiceAccountUIDKey] != string(serviceAccount.UID) {
		return false
	}
	if len(secret.Data[kapi.ServiceAccountTokenKey]) == 0 {
		return false
	}
	return true
}

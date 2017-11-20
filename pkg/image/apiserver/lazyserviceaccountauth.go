package apiserver

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kapi "k8s.io/kubernetes/pkg/api"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
)

// lazyServiceAccountSecretBasicAuthStore reads the service account token from a service account
// and uses it as basic auth credentials.
type lazyServiceAccountSecretBasicAuthStore struct {
	lock      sync.Mutex
	client    coreclient.CoreInterface
	namespace string
	name      string
	username  string
	token     string
	expires   time.Time
}

func newLazyServiceAccountSecretBasicAuthStore(client coreclient.CoreInterface, namespace, name string) *lazyServiceAccountSecretBasicAuthStore {
	return &lazyServiceAccountSecretBasicAuthStore{
		client:    client,
		namespace: namespace,
		name:      name,
		username:  strings.Replace(serviceaccount.MakeUsername(namespace, name), ":", "_", -1),
	}
}

// Basic lazily creates an access token based on template and returns it.
// If a token has already been created and has not yet expired, it is
// returned. template.UserName is the user identity. It answers the same
// value for any realm.
func (c *lazyServiceAccountSecretBasicAuthStore) Basic(*url.URL) (string, string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	now := time.Now()
	if len(c.token) != 0 {
		if !now.After(c.expires) {
			return c.username, c.token
		}
		c.token = ""
	}

	sa, err := c.client.ServiceAccounts(c.namespace).Get(c.name, metav1.GetOptions{})
	if err != nil {
		glog.V(4).Infof("unable to find service account %s in namespace %s: %v", c.name, c.namespace, err)
		return c.username, ""
	}
	secrets, err := c.client.Secrets(c.namespace).List(metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("type", string(kapi.SecretTypeServiceAccountToken)).String(),
	})
	for _, ref := range sa.Secrets {
		for _, secret := range secrets.Items {
			if ref.Name == secret.Name {
				glog.V(4).Infof("matched secret %s to service account %s in namespace %s", secret.Name, c.name, c.namespace)
				c.token = string(secret.Data[kapi.ServiceAccountTokenKey])
				c.expires = now.Add(1 * time.Hour)
				return c.username, c.token
			}
		}
	}
	glog.V(4).Infof("no matching secret for service account %s in namespace %s", c.name, c.namespace)
	return c.username, ""
}

// Forget clears any stored token.
func (c *lazyServiceAccountSecretBasicAuthStore) Forget(*url.URL) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(c.token) == 0 {
		return
	}
	c.token = ""
}

func (c *lazyServiceAccountSecretBasicAuthStore) RefreshToken(*url.URL, string) string { return "" }
func (c *lazyServiceAccountSecretBasicAuthStore) SetRefreshToken(realm *url.URL, service, token string) {
}

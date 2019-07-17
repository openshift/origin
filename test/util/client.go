package util

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pborman/uuid"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"

	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned"
	userv1client "github.com/openshift/client-go/user/clientset/versioned"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/deprecatedclient"
)

// GetBaseDir returns the base directory used for test.
func GetBaseDir() string {
	baseDir := os.Getenv("BASETMPDIR")
	if len(baseDir) == 0 {
		return path.Join(os.TempDir(), "openshift-"+Namespace())
	}
	return baseDir
}

func KubeConfigPath() string {
	return filepath.Join(GetBaseDir(), "openshift.local.config", "master", "admin.kubeconfig")
}

func GetClusterAdminKubeClient(adminKubeConfigFile string) (kubernetes.Interface, error) {
	clientConfig, err := GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(clientConfig)
}

func GetClusterAdminClientConfig(adminKubeConfigFile string) (*restclient.Config, error) {
	conf, err := deprecatedclient.GetClientConfig(adminKubeConfigFile, nil)
	if err != nil {
		return nil, err
	}
	return turnOffRateLimiting(conf), nil
}

// GetClusterAdminClientConfigOrDie returns a REST config for the cluster admin
// user or panic.
func GetClusterAdminClientConfigOrDie(adminKubeConfigFile string) *restclient.Config {
	conf, err := GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		panic(err)
	}
	return conf
}

func GetClientForUser(clusterAdminConfig *restclient.Config, username string) (kubernetes.Interface, *restclient.Config, error) {
	userClient, err := userv1client.NewForConfig(clusterAdminConfig)
	if err != nil {
		return nil, nil, err
	}

	user, err := userClient.UserV1().Users().Get(username, metav1.GetOptions{})
	if err != nil && !kerrs.IsNotFound(err) {
		return nil, nil, err
	}
	if err != nil {
		user = &userv1.User{
			ObjectMeta: metav1.ObjectMeta{Name: username},
		}
		user, err = userClient.UserV1().Users().Create(user)
		if err != nil {
			return nil, nil, err
		}
	}

	oauthClient, err := oauthv1client.NewForConfig(clusterAdminConfig)
	if err != nil {
		return nil, nil, err
	}

	oauthClientObj := &oauthv1.OAuthClient{
		ObjectMeta:  metav1.ObjectMeta{Name: "test-integration-client"},
		GrantMethod: oauthv1.GrantHandlerAuto,
	}
	if _, err := oauthClient.OauthV1().OAuthClients().Create(oauthClientObj); err != nil && !kerrs.IsAlreadyExists(err) {
		return nil, nil, err
	}

	randomToken := uuid.NewRandom()
	accesstoken := base64.RawURLEncoding.EncodeToString([]byte(randomToken))
	// make sure the token is long enough to pass validation
	for i := len(accesstoken); i < 32; i++ {
		accesstoken += "A"
	}
	token := &oauthv1.OAuthAccessToken{
		ObjectMeta:  metav1.ObjectMeta{Name: accesstoken},
		ClientName:  oauthClientObj.Name,
		UserName:    username,
		UserUID:     string(user.UID),
		Scopes:      []string{"user:full"},
		RedirectURI: "https://localhost:8443/oauth/token/implicit",
	}
	if _, err := oauthClient.OauthV1().OAuthAccessTokens().Create(token); err != nil {
		return nil, nil, err
	}

	userClientConfig := restclient.AnonymousClientConfig(turnOffRateLimiting(clusterAdminConfig))
	userClientConfig.BearerToken = token.Name

	kubeClientset, err := kubernetes.NewForConfig(userClientConfig)
	if err != nil {
		return nil, nil, err
	}

	return kubeClientset, userClientConfig, nil
}

func WaitForClusterResourceQuotaCRDAvailable(clusterAdminClientConfig *rest.Config) error {
	return WaitForCRDAvailable(clusterAdminClientConfig, schema.GroupVersionResource{
		Version:  "v1",
		Group:    "quota.openshift.io",
		Resource: "clusterresourcequotas",
	})
}

func WaitForSecurityContextConstraintsCRDAvailable(clusterAdminClientConfig *rest.Config) error {
	return WaitForCRDAvailable(clusterAdminClientConfig, schema.GroupVersionResource{
		Version:  "v1",
		Group:    "security.openshift.io",
		Resource: "securitycontextconstraints",
	})
}

func WaitForRoleBindingRestrictionCRDAvailable(clusterAdminClientConfig *rest.Config) error {
	return WaitForCRDAvailable(clusterAdminClientConfig, schema.GroupVersionResource{
		Version:  "v1",
		Group:    "authorization.openshift.io",
		Resource: "rolebindingrestrictions",
	})
}

func WaitForCRDAvailable(clusterAdminClientConfig *rest.Config, gvr schema.GroupVersionResource) error {
	dynamicClient := dynamic.NewForConfigOrDie(clusterAdminClientConfig)
	stopCh := make(chan struct{})
	defer close(stopCh)
	err := wait.PollImmediateUntil(1*time.Minute, func() (done bool, err error) {
		_, listErr := dynamicClient.Resource(gvr).List(metav1.ListOptions{})
		return listErr == nil, nil
	}, stopCh)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster resource quota CRD: %v", err)
	}
	return nil
}

// turnOffRateLimiting reduces the chance that a flaky test can be written while using this package
func turnOffRateLimiting(config *restclient.Config) *restclient.Config {
	configCopy := *config
	configCopy.QPS = 10000
	configCopy.Burst = 10000
	configCopy.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
	// We do not set a timeout because that will cause watches to fail
	// Integration tests are already limited to 5 minutes
	// configCopy.Timeout = time.Minute
	return &configCopy
}

package util

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pborman/uuid"

	corev1 "k8s.io/api/core/v1"
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
	quota "k8s.io/kubernetes/pkg/quota/v1"
	sautil "k8s.io/kubernetes/pkg/serviceaccount"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

// GetBaseDir returns the base directory used for test.
func GetBaseDir() string {
	return cmdutil.Env("BASETMPDIR", path.Join(os.TempDir(), "openshift-"+Namespace()))
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
	conf, err := configapi.GetClientConfig(adminKubeConfigFile, nil)
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
	userClient, err := userclient.NewForConfig(clusterAdminConfig)
	if err != nil {
		return nil, nil, err
	}

	user, err := userClient.User().Users().Get(username, metav1.GetOptions{})
	if err != nil && !kerrs.IsNotFound(err) {
		return nil, nil, err
	}
	if err != nil {
		user = &userapi.User{
			ObjectMeta: metav1.ObjectMeta{Name: username},
		}
		user, err = userClient.User().Users().Create(user)
		if err != nil {
			return nil, nil, err
		}
	}

	oauthClient, err := oauthclient.NewForConfig(clusterAdminConfig)
	if err != nil {
		return nil, nil, err
	}

	oauthClientObj := &oauthapi.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: "test-integration-client"},
	}
	if _, err := oauthClient.Oauth().OAuthClients().Create(oauthClientObj); err != nil && !kerrs.IsAlreadyExists(err) {
		return nil, nil, err
	}

	randomToken := uuid.NewRandom()
	accesstoken := base64.RawURLEncoding.EncodeToString([]byte(randomToken))
	// make sure the token is long enough to pass validation
	for i := len(accesstoken); i < 32; i++ {
		accesstoken += "A"
	}
	token := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: accesstoken},
		ClientName: oauthClientObj.Name,
		UserName:   username,
		UserUID:    string(user.UID),
	}
	if _, err := oauthClient.Oauth().OAuthAccessTokens().Create(token); err != nil {
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

func GetScopedClientForUser(clusterAdminClientConfig *restclient.Config, username string, scopes []string) (kubernetes.Interface, *restclient.Config, error) {
	// make sure the user exists
	if _, _, err := GetClientForUser(clusterAdminClientConfig, username); err != nil {
		return nil, nil, err
	}
	user, err := userclient.NewForConfigOrDie(clusterAdminClientConfig).User().Users().Get(username, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	token := &oauthapi.OAuthAccessToken{
		ObjectMeta:  metav1.ObjectMeta{Name: fmt.Sprintf("%s-token-plus-some-padding-here-to-make-the-limit-%d", username, rand.Int())},
		ClientName:  "openshift-challenging-client",
		ExpiresIn:   86400,
		Scopes:      scopes,
		RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
		UserName:    user.Name,
		UserUID:     string(user.UID),
	}
	if _, err := oauthclient.NewForConfigOrDie(clusterAdminClientConfig).Oauth().OAuthAccessTokens().Create(token); err != nil {
		return nil, nil, err
	}

	scopedConfig := restclient.AnonymousClientConfig(turnOffRateLimiting(clusterAdminClientConfig))
	scopedConfig.BearerToken = token.Name
	kubeClient, err := kubernetes.NewForConfig(scopedConfig)
	if err != nil {
		return nil, nil, err
	}
	return kubeClient, scopedConfig, nil
}

func GetClientForServiceAccount(adminClient kubernetes.Interface, clientConfig restclient.Config, namespace, name string) (*kubernetes.Clientset, *restclient.Config, error) {
	_, err := adminClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil && !kerrs.IsAlreadyExists(err) {
		return nil, nil, err
	}

	sa, err := adminClient.CoreV1().ServiceAccounts(namespace).Create(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name}})
	if kerrs.IsAlreadyExists(err) {
		sa, err = adminClient.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, nil, err
	}

	token := ""
	err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
		selector := fields.OneTermEqualSelector(coreapi.SecretTypeField, string(corev1.SecretTypeServiceAccountToken))
		secrets, err := adminClient.CoreV1().Secrets(namespace).List(metav1.ListOptions{FieldSelector: selector.String()})
		if err != nil {
			return false, err
		}
		for _, secret := range secrets.Items {
			if sautil.IsServiceAccountToken(&secret, sa) {
				token = string(secret.Data[corev1.ServiceAccountTokenKey])
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, nil, err
	}

	saClientConfig := restclient.AnonymousClientConfig(turnOffRateLimiting(&clientConfig))
	saClientConfig.BearerToken = token

	kubeClientset, err := kubernetes.NewForConfig(saClientConfig)
	if err != nil {
		return nil, nil, err
	}

	return kubeClientset, saClientConfig, nil
}

func WaitForClusterResourceQuotaCRDAvailable(clusterAdminClientConfig *rest.Config) error {
	dynamicClient := dynamic.NewForConfigOrDie(clusterAdminClientConfig)
	stopCh := make(chan struct{})
	defer close(stopCh)
	err := wait.PollImmediateUntil(1*time.Minute, func() (done bool, err error) {
		_, listErr := dynamicClient.Resource(schema.GroupVersionResource{
			Version:  "v1",
			Group:    "quota.openshift.io",
			Resource: "clusterresourcequotas",
		}).List(metav1.ListOptions{})
		return listErr == nil, nil
	}, stopCh)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster resource quota CRD: %v", err)
	}
	return nil
}

// WaitForResourceQuotaLimitSync watches given resource quota until its hard limit is updated to match the desired
// spec or timeout occurs.
func WaitForResourceQuotaLimitSync(
	client corev1client.ResourceQuotaInterface,
	name string,
	hardLimit corev1.ResourceList,
	timeout time.Duration,
) error {
	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quota.ResourceNames(hardLimit)

	list, err := client.List(metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
	if err != nil {
		return err
	}

	for i := range list.Items {
		used := quota.Mask(list.Items[i].Status.Hard, expectedResourceNames)
		if isLimitSynced(used, hardLimit) {
			return nil
		}
	}

	rv := list.ResourceVersion
	w, err := client.Watch(metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String(), ResourceVersion: rv})
	if err != nil {
		return err
	}
	defer w.Stop()

	for time.Now().Before(endTime) {
		select {
		case val, ok := <-w.ResultChan():
			if !ok {
				// reget and re-watch
				continue
			}
			if rq, ok := val.Object.(*corev1.ResourceQuota); ok {
				used := quota.Mask(rq.Status.Hard, expectedResourceNames)
				if isLimitSynced(used, hardLimit) {
					return nil
				}
			}
		case <-time.After(endTime.Sub(time.Now())):
			return wait.ErrWaitTimeout
		}
	}
	return wait.ErrWaitTimeout
}

func isLimitSynced(received, expected corev1.ResourceList) bool {
	resourceNames := quota.ResourceNames(expected)
	masked := quota.Mask(received, resourceNames)
	if len(masked) != len(expected) {
		return false
	}
	if le, _ := quota.LessThanOrEqual(masked, expected); !le {
		return false
	}
	if le, _ := quota.LessThanOrEqual(expected, masked); !le {
		return false
	}
	return true
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

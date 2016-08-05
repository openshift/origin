package util

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/serviceaccounts"
)

// GetBaseDir returns the base directory used for test.
func GetBaseDir() string {
	return cmdutil.Env("BASETMPDIR", path.Join(os.TempDir(), "openshift-"+Namespace()))
}

func KubeConfigPath() string {
	return filepath.Join(GetBaseDir(), "openshift.local.config", "master", "admin.kubeconfig")
}

func GetClusterAdminKubeClient(adminKubeConfigFile string) (*kclient.Client, error) {
	c, _, err := configapi.GetKubeClient(adminKubeConfigFile, nil)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func GetClusterAdminClient(adminKubeConfigFile string) (*client.Client, error) {
	clientConfig, err := GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		return nil, err
	}
	osClient, err := client.New(clientConfig)
	if err != nil {
		return nil, err
	}
	return osClient, nil
}

func GetClusterAdminClientConfig(adminKubeConfigFile string) (*restclient.Config, error) {
	_, conf, err := configapi.GetKubeClient(adminKubeConfigFile, nil)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func GetClientForUser(clientConfig restclient.Config, username string) (*client.Client, *kclient.Client, *restclient.Config, error) {
	token, err := tokencmd.RequestToken(&clientConfig, nil, username, "password")
	if err != nil {
		return nil, nil, nil, err
	}

	userClientConfig := clientcmd.AnonymousClientConfig(&clientConfig)
	userClientConfig.BearerToken = token

	kubeClient, err := kclient.New(&userClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	osClient, err := client.New(&userClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	return osClient, kubeClient, &userClientConfig, nil
}

func GetScopedClientForUser(adminClient *client.Client, clientConfig restclient.Config, username string, scopes []string) (*client.Client, *kclient.Client, *restclient.Config, error) {
	// make sure the user exists
	if _, _, _, err := GetClientForUser(clientConfig, username); err != nil {
		return nil, nil, nil, err
	}
	user, err := adminClient.Users().Get(username)
	if err != nil {
		return nil, nil, nil, err
	}

	token := &oauthapi.OAuthAccessToken{
		ObjectMeta:  kapi.ObjectMeta{Name: fmt.Sprintf("%s-token-plus-some-padding-here-to-make-the-limit-%d", username, rand.Int())},
		ClientName:  origin.OpenShiftCLIClientID,
		ExpiresIn:   86400,
		Scopes:      scopes,
		RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
		UserName:    user.Name,
		UserUID:     string(user.UID),
	}
	if _, err := adminClient.OAuthAccessTokens().Create(token); err != nil {
		return nil, nil, nil, err
	}

	scopedConfig := clientcmd.AnonymousClientConfig(&clientConfig)
	scopedConfig.BearerToken = token.Name
	kubeClient, err := kclient.New(&scopedConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	osClient, err := client.New(&scopedConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	return osClient, kubeClient, &scopedConfig, nil
}

func GetClientForServiceAccount(adminClient *kclient.Client, clientConfig restclient.Config, namespace, name string) (*client.Client, *kclient.Client, *restclient.Config, error) {
	_, err := adminClient.Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: namespace}})
	if err != nil && !kerrs.IsAlreadyExists(err) {
		return nil, nil, nil, err
	}

	sa, err := adminClient.ServiceAccounts(namespace).Create(&kapi.ServiceAccount{ObjectMeta: kapi.ObjectMeta{Name: name}})
	if kerrs.IsAlreadyExists(err) {
		sa, err = adminClient.ServiceAccounts(namespace).Get(name)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	token := ""
	err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
		selector := fields.OneTermEqualSelector(kapi.SecretTypeField, string(kapi.SecretTypeServiceAccountToken))
		secrets, err := adminClient.Secrets(namespace).List(kapi.ListOptions{FieldSelector: selector})
		if err != nil {
			return false, err
		}
		for _, secret := range secrets.Items {
			if serviceaccounts.IsValidServiceAccountToken(sa, &secret) {
				token = string(secret.Data[kapi.ServiceAccountTokenKey])
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, nil, nil, err
	}

	saClientConfig := clientcmd.AnonymousClientConfig(&clientConfig)
	saClientConfig.BearerToken = token

	kubeClient, err := kclient.New(&saClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	osClient, err := client.New(&saClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	return osClient, kubeClient, &saClientConfig, nil
}

// WaitForResourceQuotaSync watches given resource quota until its hard limit is updated to match the desired
// spec or timeout occurs.
func WaitForResourceQuotaLimitSync(
	client kclient.ResourceQuotaInterface,
	name string,
	hardLimit kapi.ResourceList,
	timeout time.Duration,
) error {

	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quota.ResourceNames(hardLimit)

	list, err := client.List(kapi.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector()})
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
	w, err := client.Watch(kapi.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector(), ResourceVersion: rv})
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
			if rq, ok := val.Object.(*kapi.ResourceQuota); ok {
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

func isLimitSynced(received, expected kapi.ResourceList) bool {
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

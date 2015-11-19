package util

import (
	"os"
	"path"
	"path/filepath"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
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
	if c, _, err := configapi.GetKubeClient(adminKubeConfigFile); err != nil {
		return nil, err
	} else {
		return c, nil
	}
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

func GetClusterAdminClientConfig(adminKubeConfigFile string) (*kclient.Config, error) {
	_, conf, err := configapi.GetKubeClient(adminKubeConfigFile)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func GetClientForUser(clientConfig kclient.Config, username string) (*client.Client, *kclient.Client, *kclient.Config, error) {
	token, err := tokencmd.RequestToken(&clientConfig, nil, username, "password")
	if err != nil {
		return nil, nil, nil, err
	}

	userClientConfig := clientcmd.AnonymousClientConfig(clientConfig)
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

func GetClientForServiceAccount(adminClient *kclient.Client, clientConfig kclient.Config, namespace, name string) (*client.Client, *kclient.Client, *kclient.Config, error) {
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
		selector := fields.OneTermEqualSelector(kclient.SecretType, string(kapi.SecretTypeServiceAccountToken))
		secrets, err := adminClient.Secrets(namespace).List(labels.Everything(), selector)
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

	saClientConfig := clientcmd.AnonymousClientConfig(clientConfig)
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

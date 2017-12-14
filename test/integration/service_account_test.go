package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	api "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	"github.com/openshift/origin/pkg/serviceaccounts/controllers"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestServiceAccountAuthorization(t *testing.T) {
	saNamespace := api.NamespaceDefault
	saName := serviceaccountadmission.DefaultServiceAccountName
	saUsername := apiserverserviceaccount.MakeUsername(saNamespace, saName)

	// Start one OpenShift master as "cluster1" to play the external kube server
	masterConfig, cluster1AdminConfigFile, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	cluster1AdminConfig, err := testutil.GetClusterAdminClientConfig(cluster1AdminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cluster1AdminKubeClientset, err := testutil.GetClusterAdminKubeClient(cluster1AdminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get a service account token and build a client
	saToken, err := waitForServiceAccountToken(cluster1AdminKubeClientset, saNamespace, saName, 20, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saToken) == 0 {
		t.Fatalf("token was not created")
	}
	cluster1SAClientConfig := restclient.Config{
		Host:        cluster1AdminConfig.Host,
		BearerToken: saToken,
		TLSClientConfig: restclient.TLSClientConfig{
			CAFile: cluster1AdminConfig.CAFile,
			CAData: cluster1AdminConfig.CAData,
		},
	}
	cluster1SAKubeClient, err := kclientset.NewForConfig(&cluster1SAClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Make sure the service account doesn't have access
	failNS := &api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-fail"}}
	if _, err := cluster1SAKubeClient.Core().Namespaces().Create(failNS); !errors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got %v", err)
	}

	// Make the service account a cluster admin on cluster1
	addRoleOptions := &policy.RoleModificationOptions{
		RoleName:            bootstrappolicy.ClusterAdminRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(authorizationclient.NewForConfigOrDie(cluster1AdminConfig).Authorization()),
		Users:               []string{saUsername},
	}
	if err := addRoleOptions.AddRole(); err != nil {
		t.Fatal(err)
	}

	// Give the policy cache a second to catch its breath
	time.Sleep(time.Second)

	// Make sure the service account now has access
	// This tests authentication using the etcd-based token getter
	passNS := &api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-pass"}}
	if _, err := cluster1SAKubeClient.Core().Namespaces().Create(passNS); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a kubeconfig from the serviceaccount config
	cluster1SAKubeConfigFile, err := ioutil.TempFile(testutil.GetBaseDir(), "cluster1-service-account.kubeconfig")
	if err != nil {
		t.Fatalf("error creating tmpfile: %v", err)
	}
	defer os.Remove(cluster1SAKubeConfigFile.Name())
	if err := writeClientConfigToKubeConfig(cluster1SAClientConfig, cluster1SAKubeConfigFile.Name()); err != nil {
		t.Fatalf("error creating kubeconfig: %v", err)
	}
}

func writeClientConfigToKubeConfig(config restclient.Config, path string) error {
	kubeConfig := &clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"myserver": {Server: config.Host, CertificateAuthority: config.CAFile, CertificateAuthorityData: config.CAData}},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{"myuser": {Token: config.BearerToken}},
		Contexts:       map[string]*clientcmdapi.Context{"mycontext": {Cluster: "myserver", AuthInfo: "myuser"}},
		CurrentContext: "mycontext",
	}
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}
	if err := clientcmd.WriteToFile(*kubeConfig, path); err != nil {
		return err
	}
	return nil
}

func waitForServiceAccountToken(client kclientset.Interface, ns, name string, attempts int, interval time.Duration) (string, error) {
	for i := 0; i <= attempts; i++ {
		time.Sleep(interval)
		token, err := getServiceAccountToken(client, ns, name)
		if err != nil {
			return "", err
		}
		if len(token) > 0 {
			return token, nil
		}
	}
	return "", nil
}

func getServiceAccountToken(client kclientset.Interface, ns, name string) (string, error) {
	secrets, err := client.Core().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, secret := range secrets.Items {
		if secret.Type == api.SecretTypeServiceAccountToken && secret.Annotations[api.ServiceAccountNameKey] == name {
			sa, err := client.Core().ServiceAccounts(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				return "", err
			}

			for _, ref := range sa.Secrets {
				if ref.Name == secret.Name {
					return string(secret.Data[api.ServiceAccountTokenKey]), nil
				}
			}

		}
	}

	return "", nil
}

func TestAutomaticCreationOfPullSecrets(t *testing.T) {
	saNamespace := api.NamespaceDefault
	saName := serviceaccountadmission.DefaultServiceAccountName

	masterConfig, clusterAdminConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get a service account token
	saToken, err := waitForServiceAccountToken(clusterAdminKubeClient, saNamespace, saName, 20, time.Second)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(saToken) == 0 {
		t.Errorf("token was not created")
	}

	// Get the matching dockercfg secret
	_, saPullSecret, err := waitForServiceAccountPullSecret(clusterAdminKubeClient, saNamespace, saName, 20, time.Second)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(saPullSecret) == 0 {
		t.Errorf("pull secret was not created")
	}
}

func waitForServiceAccountPullSecret(client kclientset.Interface, ns, name string, attempts int, interval time.Duration) (string, string, error) {
	for i := 0; i <= attempts; i++ {
		time.Sleep(interval)
		secretName, token, err := getServiceAccountPullSecret(client, ns, name)
		if err != nil {
			return "", "", err
		}
		if len(token) > 0 {
			return secretName, token, nil
		}
	}
	return "", "", nil
}

func getServiceAccountPullSecret(client kclientset.Interface, ns, name string) (string, string, error) {
	secrets, err := client.Core().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	for _, secret := range secrets.Items {
		if secret.Type == api.SecretTypeDockercfg && secret.Annotations[api.ServiceAccountNameKey] == name {
			return secret.Name, string(secret.Data[api.DockerConfigKey]), nil
		}
	}
	return "", "", nil
}

func TestEnforcingServiceAccount(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	masterConfig.ServiceAccountConfig.LimitSecretReferences = false
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get a service account token
	saToken, err := waitForServiceAccountToken(clusterAdminKubeClient, api.NamespaceDefault, serviceaccountadmission.DefaultServiceAccountName, 20, time.Second)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(saToken) == 0 {
		t.Errorf("token was not created")
	}

	pod := &api.Pod{}
	pod.Name = "foo"
	pod.Namespace = api.NamespaceDefault
	pod.Spec.ServiceAccountName = serviceaccountadmission.DefaultServiceAccountName

	container := api.Container{}
	container.Name = "foo"
	container.Image = "openshift/hello-openshift"
	pod.Spec.Containers = []api.Container{container}

	secretVolume := api.Volume{}
	secretVolume.Name = "bar-vol"
	secretVolume.Secret = &api.SecretVolumeSource{}
	secretVolume.Secret.SecretName = "bar"
	pod.Spec.Volumes = []api.Volume{secretVolume}

	err = wait.Poll(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := clusterAdminKubeClient.Core().Pods(api.NamespaceDefault).Create(pod); err != nil {
			// The SA admission controller cache seems to take forever to update.  This check comes after the limit check, so until we get it sorted out
			// check if we're getting this particular error
			if strings.Contains(err.Error(), "no API token found for service account") {
				return true, nil
			}

			t.Log(err)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminKubeClient.Core().Pods(api.NamespaceDefault).Delete(pod.Name, nil)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		sa, err := clusterAdminKubeClient.Core().ServiceAccounts(api.NamespaceDefault).Get(bootstrappolicy.DeployerServiceAccountName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}
		sa.Annotations[serviceaccountadmission.EnforceMountableSecretsAnnotation] = "true"
		_, err = clusterAdminKubeClient.Core().ServiceAccounts(api.NamespaceDefault).Update(sa)
		return err
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedMessage := "is not allowed because service account deployer does not reference that secret"
	pod.Spec.ServiceAccountName = bootstrappolicy.DeployerServiceAccountName

	err = wait.Poll(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := clusterAdminKubeClient.Core().Pods(api.NamespaceDefault).Create(pod); err == nil || !strings.Contains(err.Error(), expectedMessage) {
			clusterAdminKubeClient.Core().Pods(api.NamespaceDefault).Delete(pod.Name, nil)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

}

func TestDockercfgTokenDeletedController(t *testing.T) {
	masterConfig, clusterAdminConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sa := &api.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: "ns1"},
	}

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, sa.Namespace, "ignored"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secretsWatch, err := clusterAdminKubeClient.Core().Secrets(sa.Namespace).Watch(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer secretsWatch.Stop()

	if _, err := clusterAdminKubeClient.Core().ServiceAccounts(sa.Namespace).Create(sa); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, sa.Namespace, []string{sa.Name}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the service account dockercfg secret's name
	dockercfgSecretName, _, err := waitForServiceAccountPullSecret(clusterAdminKubeClient, sa.Namespace, sa.Name, 20, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dockercfgSecretName) == 0 {
		t.Fatal("pull secret was not created")
	}

	// Get the matching secret's name
	dockercfgSecret, err := clusterAdminKubeClient.Core().Secrets(sa.Namespace).Get(dockercfgSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secretName := dockercfgSecret.Annotations[controllers.ServiceAccountTokenSecretNameKey]
	if len(secretName) == 0 {
		t.Fatal("secret was not created")
	}

	// Delete the service account's secret
	if err := clusterAdminKubeClient.Core().Secrets(sa.Namespace).Delete(secretName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect the matching dockercfg secret to also be deleted
	waitForSecretDelete(dockercfgSecretName, secretsWatch, t)
}

func waitForSecretDelete(secretName string, w watch.Interface, t *testing.T) {
	for {
		select {
		case event := <-w.ResultChan():
			secret := event.Object.(*api.Secret)
			secret.Data = nil // reduce noise in log
			t.Logf("got %#v %#v", event, secret)
			if event.Type == watch.Deleted && secret.Name == secretName {
				return
			}

		case <-time.After(3 * time.Minute):
			t.Fatalf("timeout: %v", secretName)
		}
	}
}

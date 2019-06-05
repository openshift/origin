package integration

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	"github.com/openshift/openshift-controller-manager/pkg/serviceaccounts/controllers"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func waitForServiceAccountToken(client kubernetes.Interface, ns, name string, attempts int, interval time.Duration) (string, error) {
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

func getServiceAccountToken(client kubernetes.Interface, ns, name string) (string, error) {
	secrets, err := client.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, secret := range secrets.Items {
		if secret.Type == corev1.SecretTypeServiceAccountToken && secret.Annotations[corev1.ServiceAccountNameKey] == name {
			sa, err := client.CoreV1().ServiceAccounts(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				return "", err
			}

			for _, ref := range sa.Secrets {
				if ref.Name == secret.Name {
					return string(secret.Data[corev1.ServiceAccountTokenKey]), nil
				}
			}

		}
	}

	return "", nil
}

func TestAutomaticCreationOfPullSecrets(t *testing.T) {
	saNamespace := corev1.NamespaceDefault
	saName := serviceaccountadmission.DefaultServiceAccountName

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.ImagePolicyConfig.InternalRegistryHostname = "internal.registry.com:8080"
	masterConfig.ImagePolicyConfig.ExternalRegistryHostnames = []string{"external.registry.com"}
	clusterAdminConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
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
	if !strings.Contains(saPullSecret, masterConfig.ImagePolicyConfig.InternalRegistryHostname) {
		t.Errorf("missing %q in %v", masterConfig.ImagePolicyConfig.InternalRegistryHostname, saPullSecret)
	}
	if !strings.Contains(saPullSecret, masterConfig.ImagePolicyConfig.ExternalRegistryHostnames[0]) {
		t.Errorf("missing %q in %v", masterConfig.ImagePolicyConfig.ExternalRegistryHostnames[0], saPullSecret)
	}
}

func waitForServiceAccountPullSecret(client kubernetes.Interface, ns, name string, attempts int, interval time.Duration) (string, string, error) {
	for i := 0; i <= attempts; i++ {
		time.Sleep(interval)
		secretName, dockerCfg, err := getServiceAccountPullSecret(client, ns, name)
		if err != nil {
			return "", "", err
		}
		if len(dockerCfg) > 2 {
			return secretName, dockerCfg, nil
		}
	}
	return "", "", nil
}

func getServiceAccountPullSecret(client kubernetes.Interface, ns, name string) (string, string, error) {
	secrets, err := client.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	for _, secret := range secrets.Items {
		if secret.Type == corev1.SecretTypeDockercfg && secret.Annotations[corev1.ServiceAccountNameKey] == name {
			return secret.Name, string(secret.Data[corev1.DockerConfigKey]), nil
		}
	}
	return "", "", nil
}

func TestDockercfgTokenDeletedController(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.ImagePolicyConfig.InternalRegistryHostname = "internal.registry.com:8080"
	masterConfig.ImagePolicyConfig.ExternalRegistryHostnames = []string{"external.registry.com"}
	clusterAdminConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
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

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: "ns1"},
	}

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, sa.Namespace, "ignored"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secretsWatch, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Watch(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer secretsWatch.Stop()

	if _, err := clusterAdminKubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(sa); err != nil {
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
	dockercfgSecret, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Get(dockercfgSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secretName := dockercfgSecret.Annotations[controllers.ServiceAccountTokenSecretNameKey]
	if len(secretName) == 0 {
		t.Fatal("secret was not created")
	}

	// Delete the service account's secret
	if err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Delete(secretName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect the matching dockercfg secret to also be deleted
	waitForSecretDelete(dockercfgSecretName, secretsWatch, t)
}

func waitForSecretDelete(secretName string, w watch.Interface, t *testing.T) {
	for {
		select {
		case event := <-w.ResultChan():
			secret := event.Object.(*corev1.Secret)
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

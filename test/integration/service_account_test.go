package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/printers"
	"k8s.io/client-go/kubernetes"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/oc/cli/admin/policy"
	"github.com/openshift/origin/pkg/serviceaccounts/controllers"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestServiceAccountAuthorization(t *testing.T) {
	saNamespace := corev1.NamespaceDefault
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
	cluster1SAClientConfig := rest.Config{
		Host:        cluster1AdminConfig.Host,
		BearerToken: saToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: cluster1AdminConfig.CAFile,
			CAData: cluster1AdminConfig.CAData,
		},
	}
	cluster1SAKubeClient, err := kubernetes.NewForConfig(&cluster1SAClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Make sure the service account doesn't have access
	failNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-fail"}}
	if _, err := cluster1SAKubeClient.CoreV1().Namespaces().Create(failNS); !errors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got %v", err)
	}

	// Make the service account a cluster admin on cluster1
	addRoleOptions := &policy.RoleModificationOptions{
		RoleName:   bootstrappolicy.ClusterAdminRoleName,
		RoleKind:   "ClusterRole",
		RbacClient: rbacv1client.NewForConfigOrDie(cluster1AdminConfig),
		Users:      []string{saUsername},
		PrintFlags: genericclioptions.NewPrintFlags(""),
		ToPrinter:  func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}
	if err := addRoleOptions.AddRole(); err != nil {
		t.Fatal(err)
	}

	// Give the policy cache a second to catch its breath
	time.Sleep(time.Second)

	// Make sure the service account now has access
	// This tests authentication using the etcd-based token getter
	passNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-pass"}}
	if _, err := cluster1SAKubeClient.CoreV1().Namespaces().Create(passNS); err != nil {
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

func writeClientConfigToKubeConfig(config rest.Config, path string) error {
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

func TestEnforcingServiceAccount(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
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
	saToken, err := waitForServiceAccountToken(clusterAdminKubeClient, corev1.NamespaceDefault, serviceaccountadmission.DefaultServiceAccountName, 20, time.Second)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(saToken) == 0 {
		t.Errorf("token was not created")
	}

	pod := &corev1.Pod{}
	pod.Name = "foo"
	pod.Namespace = corev1.NamespaceDefault
	pod.Spec.ServiceAccountName = serviceaccountadmission.DefaultServiceAccountName

	container := corev1.Container{}
	container.Name = "foo"
	container.Image = "openshift/hello-openshift"
	pod.Spec.Containers = []corev1.Container{container}

	secretVolume := corev1.Volume{}
	secretVolume.Name = "bar-vol"
	secretVolume.Secret = &corev1.SecretVolumeSource{}
	secretVolume.Secret.SecretName = "bar"
	pod.Spec.Volumes = []corev1.Volume{secretVolume}

	err = wait.Poll(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := clusterAdminKubeClient.CoreV1().Pods(corev1.NamespaceDefault).Create(pod); err != nil {
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

	clusterAdminKubeClient.CoreV1().Pods(corev1.NamespaceDefault).Delete(pod.Name, nil)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		sa, err := clusterAdminKubeClient.CoreV1().ServiceAccounts(corev1.NamespaceDefault).Get(bootstrappolicy.DeployerServiceAccountName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}
		sa.Annotations[serviceaccountadmission.EnforceMountableSecretsAnnotation] = "true"
		_, err = clusterAdminKubeClient.CoreV1().ServiceAccounts(corev1.NamespaceDefault).Update(sa)
		return err
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedMessage := "is not allowed because service account deployer does not reference that secret"
	pod.Spec.ServiceAccountName = bootstrappolicy.DeployerServiceAccountName

	err = wait.Poll(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := clusterAdminKubeClient.CoreV1().Pods(corev1.NamespaceDefault).Create(pod); err == nil || !strings.Contains(err.Error(), expectedMessage) {
			clusterAdminKubeClient.CoreV1().Pods(corev1.NamespaceDefault).Delete(pod.Name, nil)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

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

package controller_manager

import (
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	exutil "github.com/openshift/origin/test/extended/util"
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

var _ = g.Describe("[Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("pull-secrets", exutil.KubeConfigPath())

	g.It("TestAutomaticCreationOfPullSecrets", func() {
		t := g.GinkgoT()

		clusterAdminKubeClient := oc.AdminKubeClient()
		saNamespace := oc.Namespace()
		saName := "default"

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

		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get("cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(saPullSecret, imageConfig.Status.InternalRegistryHostname) {
			t.Errorf("missing %q in %v", imageConfig.Status.InternalRegistryHostname, saPullSecret)
		}

		if len(imageConfig.Spec.ExternalRegistryHostnames) > 0 {
			if !strings.Contains(saPullSecret, imageConfig.Spec.ExternalRegistryHostnames[0]) {
				t.Errorf("missing %q in %v", imageConfig.Spec.ExternalRegistryHostnames[0], saPullSecret)
			}
		}

	})
})

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

var _ = g.Describe("[Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("pull-secrets", exutil.KubeConfigPath())

	g.It("TestDockercfgTokenDeletedController", func() {
		t := g.GinkgoT()

		clusterAdminKubeClient := oc.AdminKubeClient()
		saNamespace := oc.Namespace()

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: saNamespace},
		}

		sa, err := clusterAdminKubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(sa)
		if err != nil {
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
		secretName := dockercfgSecret.Annotations["openshift.io/token-secret.name"]
		if len(secretName) == 0 {
			t.Fatal("secret was not created")
		}

		// Delete the service account's secret
		if err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Delete(secretName, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expect the matching dockercfg secret to also be deleted
		if err := wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
			_, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Get(
				dockercfgSecretName,
				metav1.GetOptions{},
			)
			if err != nil {
				t.Logf("getting docker secret returned: %v", err)
			}
			return errors.IsNotFound(err), nil
		}); err != nil {
			t.Fatalf("waiting for secret deletion: %v", err)
		}
	})
})

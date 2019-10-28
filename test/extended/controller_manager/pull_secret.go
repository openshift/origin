package controller_manager

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

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

		clusterAdminKubeClient := oc.AdminKubeClient()
		saNamespace := oc.Namespace()
		saName := "default"

		// Get a service account token
		g.By("waiting for service account token")
		saToken, err := waitForServiceAccountToken(clusterAdminKubeClient, saNamespace, saName, 20, time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(saToken).NotTo(o.BeEmpty())

		// Get the matching dockercfg secret
		g.By("waiting for service account pull secret")
		_, saPullSecret, err := waitForServiceAccountPullSecret(clusterAdminKubeClient, saNamespace, saName, 20, time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(saPullSecret).NotTo(o.BeEmpty())

		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get("cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(saPullSecret).To(o.ContainSubstring(imageConfig.Status.InternalRegistryHostname))

		if len(imageConfig.Spec.ExternalRegistryHostnames) > 0 {
			o.Expect(saPullSecret).To(o.ContainSubstring(imageConfig.Spec.ExternalRegistryHostnames[0]))
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
		clusterAdminKubeClient := oc.AdminKubeClient()
		saNamespace := oc.Namespace()

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: saNamespace},
		}
		g.By("creating service account")
		secretsWatch, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Watch(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer secretsWatch.Stop()

		_, err = clusterAdminKubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(sa)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Get the service account dockercfg secret's name
		dockercfgSecretName, _, err := waitForServiceAccountPullSecret(clusterAdminKubeClient, sa.Namespace, sa.Name, 20, time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dockercfgSecretName).NotTo(o.BeEmpty())

		// Get the matching secret's name
		dockercfgSecret, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Get(dockercfgSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		secretName := dockercfgSecret.Annotations["openshift.io/token-secret.name"]
		o.Expect(secretName).NotTo(o.BeEmpty())

		// Delete the service account's secret
		g.By("deleting service account's secret")
		err = clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Delete(secretName, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Expect the matching dockercfg secret to also be deleted
		waitForSecretDelete(dockercfgSecretName, secretsWatch)
	})
})

func waitForSecretDelete(secretName string, w watch.Interface) {
	for {
		select {
		case event := <-w.ResultChan():
			secret := event.Object.(*corev1.Secret)
			secret.Data = nil // reduce noise in log
			e2e.Logf("got %#v %#v", event, secret)
			if event.Type == watch.Deleted && secret.Name == secretName {
				return
			}

		case <-time.After(3 * time.Minute):
			g.Fail(fmt.Sprintf("timed out waiting for secret %s to be deleted", secretName))
		}
	}
}

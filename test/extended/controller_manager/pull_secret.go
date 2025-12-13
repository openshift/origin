package controller_manager

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("pull-secrets")

	g.It("TestAutomaticCreationOfPullSecrets [apigroup:config.openshift.io][apigroup:image.openshift.io]", g.Label("Size:S"), func() {
		t := g.GinkgoT()

		clusterAdminKubeClient := oc.AdminKubeClient()
		saNamespace := oc.Namespace()
		saName := "default"

		// Get the matching dockercfg secret
		_, saPullSecret, err := waitForServiceAccountPullSecret(clusterAdminKubeClient, saNamespace, saName, 20, time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(saPullSecret) == 0 {
			t.Errorf("pull secret was not created")
		}

		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(context.Background(), "cluster", metav1.GetOptions{})
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
	secrets, err := client.CoreV1().Secrets(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	for _, secret := range secrets.Items {
		if secret.Type != corev1.SecretTypeDockercfg {
			continue
		}
		if secret.Annotations["openshift.io/internal-registry-auth-token.service-account"] == name {
			return secret.Name, string(secret.Data[corev1.DockerConfigKey]), nil
		}
		// TODO remove after 4.16
		if secret.Annotations[corev1.ServiceAccountNameKey] == name {
			return secret.Name, string(secret.Data[corev1.DockerConfigKey]), nil
		}
	}
	return "", "", nil
}

var _ = g.Describe("[sig-devex][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("pull-secrets")

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			g.By("dumping openshift-controller-manager logs")
			exutil.DumpPodLogsStartingWithInNamespace("controller-manager", "openshift-controller-manager", oc.AsAdmin())
		}
	})

	g.It("TestDockercfgTokenDeletedController [apigroup:image.openshift.io]", g.Label("Size:S"), func() {
		t := g.GinkgoT()

		clusterAdminKubeClient := oc.AdminKubeClient()
		saNamespace := oc.Namespace()

		g.By("creating test service account")
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: saNamespace},
		}

		sa, err := clusterAdminKubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(context.Background(), sa, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		g.By("waiting for service account's pull secret to be created")
		dockercfgSecretName, _, err := waitForServiceAccountPullSecret(clusterAdminKubeClient, sa.Namespace, sa.Name, 20, time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dockercfgSecretName) == 0 {
			t.Fatal("pull secret was not created")
		}

		// Get the matching secret's name
		dockercfgSecret, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Get(context.Background(), dockercfgSecretName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// TODO the rest of this test can be removed after 4.16

		if dockercfgSecret.Annotations["openshift.io/internal-registry-auth-token.binding"] == "bound" {
			// image pull secret auth contains bound service account API tokens so no service account API token secret to be found
			return
		}
		secretName := dockercfgSecret.Annotations["openshift.io/token-secret.name"]
		if len(secretName) == 0 {
			t.Fatal("secret was not created")
		}

		g.By("deleting the token used to generate the pull secret")
		if err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Delete(context.Background(), secretName, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expect the matching dockercfg secret to also be deleted
		if err := wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
			_, err := clusterAdminKubeClient.CoreV1().Secrets(sa.Namespace).Get(
				context.Background(),
				dockercfgSecretName,
				metav1.GetOptions{},
			)
			if err == nil {
				e2e.Logf("secret %s/%s exists", sa.Namespace, dockercfgSecretName)
				return false, nil
			}
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}); err != nil {
			t.Fatalf("error waiting for pull secret deletion: %v", err)
		}
	})
})

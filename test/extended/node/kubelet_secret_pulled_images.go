package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/credentialprovider"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// internalRegistryPrefix is the OpenShift internal registry service address
	internalRegistryPrefix = "image-registry.openshift-image-registry.svc:5000"
	// credVerifyPublicImage is a cluster-hosted image accessible to all authenticated SAs
	credVerifyPublicImage = internalRegistryPrefix + "/openshift/tools:latest"
)

var _ = g.Describe("[sig-node][Suite:openshift/disruptive-longrunning][Disruptive][OCPFeatureGate:KubeletEnsureSecretPulledImages][Serial]", g.Ordered, func() {
	defer g.GinkgoRecover()

	var (
		oc         = exutil.NewCLIWithoutNamespace("kubelet-cred-verify")
		ctx        = context.Background()
		sourceNS   = "cred-verify-source"
		workerNode string

		privateImage string
		pullSecret   []byte
	)

	// Setup: import a private image into the internal registry so all tests
	// can use it without hardcoded credentials or external accounts.
	g.BeforeAll(func() {
		// Skip on MicroShift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}

		if !exutil.IsNoUpgradeFeatureSet(oc) {
			g.Skip("requires TechPreviewNoUpgrade or CustomNoUpgrade feature set")
		}

		nodes, err := getWorkerNodes(oc)
		if err != nil || len(nodes) == 0 {
			g.Skip("no worker nodes available")
		}
		workerNode = nodes[0].Name
		e2e.Logf("Worker node: %s", workerNode)

		// Tag the cluster-hosted openshift/tools image into a namespace-scoped imagestream
		// so it becomes a "private" image requiring namespace-level pull credentials.
		credVerifyEnsureNamespace(ctx, oc, sourceNS)
		privateImage = fmt.Sprintf("%s/%s/test-image:latest", internalRegistryPrefix, sourceNS)

		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(
			"openshift/tools:latest",
			fmt.Sprintf("%s/test-image:latest", sourceNS),
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() error {
			out, e := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"istag", "test-image:latest", "-n", sourceNS,
				"-o", "jsonpath={.image.dockerImageReference}",
			).Output()
			if e != nil {
				return e
			}
			if out == "" {
				return fmt.Errorf("imagestream tag not ready")
			}
			e2e.Logf("Image ready: %s", out)
			return nil
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())

		pullSecret = credVerifyExtractSAPullSecret(ctx, oc, sourceNS, "default")
		e2e.Logf("Private image: %s", privateImage)
	})

	g.AfterAll(func() {
		credVerifyDeleteNamespace(ctx, oc, sourceNS)
	})

	// This test validates that:
	// - A tenant with valid credentials can pull a private image
	// - A different tenant without credentials cannot access the same private image
	// - Both tenants can pull a public image without any secrets
	g.It("Case 1: Multi-tenancy isolation for private and public images", func() {
		tenantA := "cred-verify-tenant-a"
		tenantB := "cred-verify-tenant-b"
		credVerifyEnsureNamespace(ctx, oc, tenantA)
		credVerifyEnsureNamespace(ctx, oc, tenantB)
		g.DeferCleanup(credVerifyDeleteNamespace, ctx, oc, tenantA)
		g.DeferCleanup(credVerifyDeleteNamespace, ctx, oc, tenantB)

		// Only tenant-a gets pull permission and a pull secret
		credVerifyGrantImagePuller(oc, sourceNS, tenantA)
		credVerifyCreateSecret(ctx, oc, tenantA, "pull-secret", pullSecret)

		g.By("Verifying tenant-a can pull private image with valid secret")
		credVerifyRunPod(ctx, oc, credVerifyPod(tenantA, "pod-1a-with-secret", privateImage, workerNode, corev1.PullIfNotPresent, "pull-secret"))

		g.By("Verifying tenant-b cannot pull private image without secret")
		credVerifyExpectImagePullError(ctx, oc, credVerifyPod(tenantB, "pod-1a-no-secret", privateImage, workerNode, corev1.PullIfNotPresent))

		g.By("Verifying tenant-a can pull public image without secret")
		credVerifyRunPod(ctx, oc, credVerifyPod(tenantA, "pod-1b-public-a", credVerifyPublicImage, workerNode, corev1.PullIfNotPresent))

		g.By("Verifying tenant-b can pull same public image without secret")
		credVerifyRunPod(ctx, oc, credVerifyPod(tenantB, "pod-1b-public-b", credVerifyPublicImage, workerNode, corev1.PullIfNotPresent))
	})

	// This test validates kubelet pull record behavior during credential rotation:
	// - Pod succeeds when secret name changes but credential content (hash) stays the same
	// - Pod succeeds when secret name stays the same but credential content (hash) changes
	g.It("Case 2: Credential rotation", func() {
		ns := "cred-verify-rotation"
		credVerifyEnsureNamespace(ctx, oc, ns)
		g.DeferCleanup(credVerifyDeleteNamespace, ctx, oc, ns)

		credVerifyGrantImagePuller(oc, sourceNS, ns)
		credVerifyCreateSecret(ctx, oc, ns, "secret-v1", pullSecret)

		g.By("Pulling private image with secret-v1 to establish pull record on the node")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-initial-pull", privateImage, workerNode, corev1.PullIfNotPresent, "secret-v1"))

		// Delete secret-v1 and recreate as secret-v2 with the SAME credentials.
		// The secret name (coordinates) changed, but the credential content (hash) is identical.
		g.By("Verifying pod succeeds when secret hash matches but secret coordinates differ")
		err := oc.AdminKubeClient().CoreV1().Secrets(ns).Delete(ctx, "secret-v1", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		credVerifyCreateSecret(ctx, oc, ns, "secret-v2", pullSecret)
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-hash-match", privateImage, workerNode, corev1.PullIfNotPresent, "secret-v2"))

		// Create a second SA to get different credentials, then recreate secret-v2
		// with those new credentials. The secret name stays the same but the content changes.
		g.By("Verifying pod succeeds when secret coordinates match but secret hash differs")
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "rotated-sa"},
		}
		_, err = oc.AdminKubeClient().CoreV1().ServiceAccounts(sourceNS).Create(ctx, sa, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		rotatedSecret := credVerifyExtractSAPullSecret(ctx, oc, sourceNS, "rotated-sa")
		err = oc.AdminKubeClient().CoreV1().Secrets(ns).Delete(ctx, "secret-v2", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		credVerifyCreateSecret(ctx, oc, ns, "secret-v2", rotatedSecret)
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-coord-match", privateImage, workerNode, corev1.PullIfNotPresent, "secret-v2"))
	})

	// This test validates credential verification across all ImagePullPolicy modes:
	// - Never: uses cached image without pulling, kubelet still verifies credentials
	// - Always: forces a fresh pull even when image is cached
	// - IfNotPresent: uses cached image with credential check
	g.It("Case 3: ImagePullPolicy scenarios", func() {
		ns := "cred-verify-pullpolicy"
		credVerifyEnsureNamespace(ctx, oc, ns)
		g.DeferCleanup(credVerifyDeleteNamespace, ctx, oc, ns)

		credVerifyGrantImagePuller(oc, sourceNS, ns)
		credVerifyCreateSecret(ctx, oc, ns, "pull-secret", pullSecret)

		// IfNotPresent first: this also caches the image on the node for the Never test
		g.By("Verifying IfNotPresent ImagePullPolicy with valid secret pulls and caches the image")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-ifnotpresent", privateImage, workerNode, corev1.PullIfNotPresent, "pull-secret"))

		g.By("Verifying Never ImagePullPolicy with valid secret uses cached image")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-never", privateImage, workerNode, corev1.PullNever, "pull-secret"))

		g.By("Verifying Always ImagePullPolicy with valid secret re-pulls the image")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-always", privateImage, workerNode, corev1.PullAlways, "pull-secret"))
	})

	// This test validates imagePullCredentialsVerificationPolicy via KubeletConfig:
	// - NeverVerify: disables credential verification, pod without secret can use cached image
	// - AlwaysVerify: requires valid credentials for all images, pod without secret is rejected
	// Switching from NeverVerify to AlwaysVerify also verifies that the policy update takes
	// effect after kubelet restart triggered by the MCO rollout.
	g.It("Case 4: Credential verification policy [Slow]", func() {
		kcName := "cred-verify-policy"
		ns := "cred-verify-policy"
		credVerifyEnsureNamespace(ctx, oc, ns)
		g.DeferCleanup(credVerifyDeleteNamespace, ctx, oc, ns)

		mcClient, err := mcclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		credVerifyGrantImagePuller(oc, sourceNS, ns)
		credVerifyCreateSecret(ctx, oc, ns, "pull-secret", pullSecret)

		g.DeferCleanup(func() {
			_ = deleteKC(oc, kcName)
			_ = waitForMCP(ctx, mcClient, "worker", 30*time.Minute)
		})

		g.By("Pre-caching private image on the node with a valid secret")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-seed", privateImage, workerNode, corev1.PullIfNotPresent, "pull-secret"))

		g.By("Applying NeverVerify policy and waiting for MCO rollout")
		credVerifyApplyPolicy(ctx, mcClient, kcName, `{"imagePullCredentialsVerificationPolicy":"NeverVerify"}`)
		credVerifyWaitForMCPUpdating(ctx, mcClient, "worker")
		err = waitForMCP(ctx, mcClient, "worker", 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying NeverVerify policy allows pod without secret to use cached image")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-neververify", privateImage, workerNode, corev1.PullNever))

		g.By("Switching to AlwaysVerify policy and waiting for MCO rollout")
		credVerifyApplyPolicy(ctx, mcClient, kcName, `{"imagePullCredentialsVerificationPolicy":"AlwaysVerify"}`)
		credVerifyWaitForMCPUpdating(ctx, mcClient, "worker")
		err = waitForMCP(ctx, mcClient, "worker", 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		// This pod also re-caches the image after MCO rollout since pull records are cleared
		g.By("Verifying AlwaysVerify policy allows pod with valid secret")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-alwaysverify-secret", privateImage, workerNode, corev1.PullIfNotPresent, "pull-secret"))

		// Use a nonexistent-secret to override the default SA auto-injection,
		// ensuring the pod truly has no valid credentials for AlwaysVerify to reject.
		g.By("Verifying AlwaysVerify policy blocks pod without valid secret")
		credVerifyExpectImagePullError(ctx, oc, credVerifyPod(ns, "pod-alwaysverify-nosecret", privateImage, workerNode, corev1.PullIfNotPresent, "nonexistent-secret"))
	})

	// Validates that cached pull-records work offline but new credentials need the registry for verification
	g.It("Case 5: Registry availability", func() {
		ns := "cred-verify-registry"
		credVerifyEnsureNamespace(ctx, oc, ns)
		g.DeferCleanup(credVerifyDeleteNamespace, ctx, oc, ns)

		credVerifyGrantImagePuller(oc, sourceNS, ns)
		credVerifyCreateSecret(ctx, oc, ns, "pull-secret", pullSecret)

		g.By("Caching private image then making the registry unavailable")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-seed", privateImage, workerNode, corev1.PullIfNotPresent, "pull-secret"))

		deploy, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-image-registry").Get(ctx, "image-registry", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalReplicas := ptr.Deref(deploy.Spec.Replicas, int32(1))

		err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment/image-registry", "-n", "openshift-image-registry", "--replicas=0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() {
			_ = oc.AsAdmin().WithoutNamespace().Run("scale").Args(
				"deployment/image-registry", "-n", "openshift-image-registry",
				fmt.Sprintf("--replicas=%d", originalReplicas),
			).Execute()
			_ = oc.AsAdmin().WithoutNamespace().Run("rollout").Args(
				"status", "deployment/image-registry", "-n", "openshift-image-registry", "--timeout=2m",
			).Execute()
		})
		err = oc.AsAdmin().WithoutNamespace().Run("rollout").Args("status", "deployment/image-registry", "-n", "openshift-image-registry", "--timeout=2m").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying cached pull-record works when registry is down")
		credVerifyRunPod(ctx, oc, credVerifyPod(ns, "pod-cached", privateImage, workerNode, corev1.PullIfNotPresent, "pull-secret"))

		g.By("Verifying new credentials fail when registry is down")
		credVerifyCreateSecret(ctx, oc, ns, "new-secret", credVerifyBuildDockerConfigJSON(internalRegistryPrefix, "dummy", "dummy"))
		credVerifyExpectImagePullError(ctx, oc, credVerifyPod(ns, "pod-new-creds", privateImage, workerNode, corev1.PullIfNotPresent, "new-secret"))
	})
})

// credVerifyExtractSAPullSecret reads the auto-generated dockercfg secret for the given
// service account and returns a dockerconfigjson blob suitable for use as an imagePullSecret.
func credVerifyExtractSAPullSecret(ctx context.Context, oc *exutil.CLI, namespace, saName string) []byte {
	var result []byte

	o.Eventually(func() error {
		secrets, err := oc.AdminKubeClient().CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, s := range secrets.Items {
			if s.Type != corev1.SecretTypeDockercfg {
				continue
			}
			if s.Annotations["openshift.io/internal-registry-auth-token.service-account"] != saName {
				continue
			}
			cfg := credentialprovider.DockerConfig{}
			if err := json.Unmarshal(s.Data[corev1.DockerConfigKey], &cfg); err != nil {
				return err
			}
			for _, auth := range cfg {
				result = credVerifyBuildDockerConfigJSON(internalRegistryPrefix, auth.Username, auth.Password)
				e2e.Logf("Extracted SA credentials, user=%s", auth.Username)
				return nil
			}
		}
		return fmt.Errorf("SA pull secret not found in %s", namespace)
	}, 60*time.Second, 5*time.Second).Should(o.Succeed())

	return result
}

func credVerifyBuildDockerConfigJSON(server, username, password string) []byte {
	data, err := json.Marshal(map[string]interface{}{
		"auths": map[string]interface{}{
			server: map[string]interface{}{
				"username": username,
				"password": password,
			},
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return data
}

func credVerifyGrantImagePuller(oc *exutil.CLI, sourceNS, targetNS string) {
	err := oc.AsAdmin().WithoutNamespace().Run("policy").Args(
		"add-role-to-group", "system:image-puller",
		fmt.Sprintf("system:serviceaccounts:%s", targetNS),
		"-n", sourceNS,
	).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func credVerifyEnsureNamespace(ctx context.Context, oc *exutil.CLI, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce": "baseline",
				"pod-security.kubernetes.io/audit":   "baseline",
				"pod-security.kubernetes.io/warn":    "baseline",
			},
		},
	}
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("namespace", name,
			"pod-security.kubernetes.io/enforce=baseline",
			"pod-security.kubernetes.io/audit=baseline",
			"pod-security.kubernetes.io/warn=baseline",
			"--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func credVerifyDeleteNamespace(ctx context.Context, oc *exutil.CLI, name string) {
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) || err == nil {
		return
	}
	e2e.Logf("Warning: failed to delete namespace %s: %v", name, err)
}

func credVerifyCreateSecret(ctx context.Context, oc *exutil.CLI, namespace, name string, dockerConfigJSON []byte) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": dockerConfigJSON},
	}
	_, err := oc.AdminKubeClient().CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func credVerifyRunPod(ctx context.Context, oc *exutil.CLI, pod *corev1.Pod) {
	_, err := oc.AdminKubeClient().CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), pod)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func credVerifyExpectImagePullError(ctx context.Context, oc *exutil.CLI, pod *corev1.Pod) {
	_, err := oc.AdminKubeClient().CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	err = e2epod.WaitForPodCondition(ctx, oc.AdminKubeClient(), pod.Namespace, pod.Name, "ErrImagePull", 3*time.Minute, func(p *corev1.Pod) (bool, error) {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil &&
				(cs.State.Waiting.Reason == "ErrImagePull" || cs.State.Waiting.Reason == "ImagePullBackOff") {
				return true, nil
			}
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// credVerifyApplyPolicy creates or updates a KubeletConfig targeting workers with the given raw JSON.
func credVerifyApplyPolicy(ctx context.Context, mcClient *mcclient.Clientset, name, kubeletConfigJSON string) {
	kc := &machineconfigv1.KubeletConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: machineconfigv1.KubeletConfigSpec{
			MachineConfigPoolSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pools.operator.machineconfiguration.openshift.io/worker": "",
				},
			},
			KubeletConfig: &runtime.RawExtension{
				Raw: []byte(kubeletConfigJSON),
			},
		},
	}

	existing, err := mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	existing.Spec = kc.Spec
	_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Update(ctx, existing, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// credVerifyWaitForMCPUpdating waits for the MCP to start updating, avoiding a race
// where waitForMCP returns immediately before the MCO picks up a KubeletConfig change.
func credVerifyWaitForMCPUpdating(ctx context.Context, mcClient *mcclient.Clientset, poolName string) {
	o.Eventually(func() bool {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting MCP %s: %v", poolName, err)
			return false
		}
		for _, condition := range mcp.Status.Conditions {
			if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), fmt.Sprintf("MCP %s should start updating", poolName))
}

func credVerifyPod(namespace, name, image, nodeName string, pullPolicy corev1.PullPolicy, secretName ...string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			NodeName:      nodeName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: ptr.To[int64](1000),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "test",
					Image:           image,
					ImagePullPolicy: pullPolicy,
					Command:         []string{"sh", "-c", "echo running && sleep 3600"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					},
				},
			},
		},
	}
	for _, s := range secretName {
		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: s})
	}
	return pod
}

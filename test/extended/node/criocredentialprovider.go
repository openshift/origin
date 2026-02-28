package node

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	apicfgv1 "github.com/openshift/api/config/v1"
	apicfgv1alpha1 "github.com/openshift/api/config/v1alpha1"
	"github.com/openshift/origin/test/extended/imagepolicy"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	workerPool                          = "worker"
	masterPool                          = "master"
	controllerConfigName                = "machine-config-controller"
	crioCredentialProviderName          = "crio-credential-provider"
	debugNamespaceDefault               = "default"
	genericCredentialProviderConfigPath = "/etc/kubernetes/credential-providers/generic-credential-provider.yaml"
	dummypodImage                       = "docker.io/library/nginx@sha256:7f2f2b29e70f2785a697e2364718c6dbbe198ee7e17ae736a9da80bdd85ce843"
)

var _ = g.Describe("[sig-node][Suite:openshift/disruptive-longrunning][Disruptive][OCPFeatureGate:CRIOCredentialProviderConfig][Serial]", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc                           = exutil.NewCLIWithoutNamespace("crio-credential-provider")
		tctx                         = context.Background()
		credentialProviderConfigPath string
		workerNodes                  []corev1.Node
		cli                          = exutil.NewCLIWithPodSecurityLevel("criocp-mynamespace", admissionapi.LevelBaseline)
		clif                         = cli.KubeFramework()
	)

	g.BeforeAll(func() {
		if !exutil.IsTechPreviewNoUpgrade(tctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
		credentialProviderConfigPath = getCredentialProviderConfigPath(oc)
		e2e.Logf("Using credential provider config path: %s", credentialProviderConfigPath)
		if credentialProviderConfigPath == "" {
			g.Skip("skipping, error determining expected credential provider config path")
		}
		var err error
		workerNodes, err = getWorkerNodes(oc)
		if err != nil || len(workerNodes) == 0 {
			g.Skip("skipping, no worker nodes found")
		}

	})

	g.DescribeTable("criocredentialproviderconfig tests",
		func(expectedMatchImages, updatedMatchImages, excludedMatchImages []string, expectProviderAfterUpdate bool) {
			updateCRIOCredentialProviderConfig(oc, expectedMatchImages, false)
			g.DeferCleanup(updateCRIOCredentialProviderConfig, oc, []string{}, false)

			verifyWorkerNodeCRIOCredentialProviderConfig(oc, expectedMatchImages, nil, workerNodes[0], credentialProviderConfigPath, true)

			if updatedMatchImages != nil && expectProviderAfterUpdate {
				updateCRIOCredentialProviderConfig(oc, updatedMatchImages, false)
				verifyWorkerNodeCRIOCredentialProviderConfig(oc, updatedMatchImages, excludedMatchImages, workerNodes[0], credentialProviderConfigPath, expectProviderAfterUpdate)
			}

			if !expectProviderAfterUpdate {
				updateCRIOCredentialProviderConfig(oc, updatedMatchImages, false)
				verifyWorkerNodeCRIOCredentialProviderConfig(oc, updatedMatchImages, excludedMatchImages, workerNodes[0], credentialProviderConfigPath, expectProviderAfterUpdate)
			}

		},
		// First entry tests initial setup only - updatedMatchImages is nil so update branches are intentionally skipped
		g.Entry("pass update criocredentialproviderconfig with valid image entry", []string{"docker.io",
			"123456789.dkr.ecr.us-east-1.amazonaws.com",
			"*.azurecr.io",
			"gcr.io",
			"*.*.registry.io",
			"registry.io:8080/path"}, nil, nil, true),
		g.Entry("update CRIOCredentialProviderConfig with removal one of image entries",
			[]string{"*.myhost.io", "registry.io:8080/path"},
			[]string{"registry.io:8080/path"}, []string{"*.myhost.io"}, true),
		g.Entry("remove CRIOCredentialProviderConfig entry on removal all matchImages entries",
			[]string{"*.myhost.io", "registry.io:8080/path"},
			nil, nil, false),
	)

	g.It("Should fail with empty value matchImages", func() {
		updateCRIOCredentialProviderConfig(oc, []string{""}, true)
		g.DeferCleanup(updateCRIOCredentialProviderConfig, oc, []string{}, false)
	})

	g.It("Should execute crio credential provider if private mirror configured", func() {

		matchImages := []string{"docker.io"}
		updateCRIOCredentialProviderConfig(oc, matchImages, false)
		g.DeferCleanup(updateCRIOCredentialProviderConfig, oc, []string{}, false)
		verifyWorkerNodeCRIOCredentialProviderConfig(oc, matchImages, nil, workerNodes[0], credentialProviderConfigPath, true)

		// namespace rbac
		createNamespaceRBAC(clif, clif.Namespace.Name)

		// secret
		createSecret(clif, clif.Namespace.Name, "dummy-secret", map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"docker.io":{"auth":"bXl1c2VyOm15cGFzcw=="}}}`),
		})

		// IDMS docker.io/library/nginx to docker.io/qiwanredhat/mirror-pull-secret-dummy, which requires pulling credentials from criocredentialprovider
		createIDMSResources(oc)
		g.DeferCleanup(cleanupIDMSResources, oc)

		logSince := time.Now().UTC().Format("2006-01-02 15:04:05")

		pod, err := launchTestPod(context.Background(), clif, "dummy-pod", dummypodImage, workerNodes[0].Name)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to launch test pod")
		g.DeferCleanup(func() {
			clif.ClientSet.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		})
		err = e2epod.WaitForPodCondition(context.Background(), clif.ClientSet, pod.Namespace, pod.Name, "image pull attempted", 60*time.Second, func(pod *corev1.Pod) (bool, error) {
			// Check if any container has started pulling or has a pull error
			for _, status := range pod.Status.ContainerStatuses {
				if status.State.Waiting != nil && (status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff") {
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "timeout waiting for pod to attempt image pull")
		// Verify provider log from this test window.
		var lastOut string
		var lastErr error
		o.Eventually(func() (string, error) {
			out, err := oc.AsAdmin().Run("debug").Args(
				"-n", debugNamespaceDefault, "node/"+workerNodes[0].Name, "--", "chroot", "/host", "sh", "-c",
				fmt.Sprintf("journalctl --since '%s' _COMM=crio-credential | grep 'Wrote auth file to /etc/crio/auth/'", logSince),
			).Output()
			lastOut = out
			lastErr = err
			return out, err
		}, 2*time.Minute, 5*time.Second).Should(o.ContainSubstring("Wrote auth file to /etc/crio/auth/"), "expected log message not found in criocredentialprovider logs. Last output (err=%v):\n%s", lastErr, lastOut)
	})
})

func updateCRIOCredentialProviderConfig(oc *exutil.CLI, matchImages []string, expectErr bool) {
	e2e.Logf("Updating CRIOCredentialProviderConfig 'cluster' with matchImages")
	initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, masterPool)
	var images []apicfgv1alpha1.MatchImage
	for _, img := range matchImages {
		images = append(images, apicfgv1alpha1.MatchImage(img))
	}

	skippedUpdate := false
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		crioCPConfig, err := oc.AdminConfigClient().ConfigV1alpha1().CRIOCredentialProviderConfigs().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		currentImages := make([]string, 0, len(crioCPConfig.Spec.MatchImages))
		for _, img := range crioCPConfig.Spec.MatchImages {
			currentImages = append(currentImages, string(img))
		}

		if !matchImagesDiffer(currentImages, matchImages) {
			skippedUpdate = true
			e2e.Logf("matchImages already up to date, skipping CRIOCredentialProviderConfig update")
			return nil
		}

		crioCPConfig.Spec.MatchImages = images
		_, err = oc.AdminConfigClient().ConfigV1alpha1().CRIOCredentialProviderConfigs().Update(
			context.Background(), crioCPConfig, metav1.UpdateOptions{},
		)
		return err
	})
	if expectErr {
		o.Expect(err).To(o.HaveOccurred(), "expected error updating CRIOCredentialProviderConfig 'cluster'")
		return
	}

	o.Expect(err).NotTo(o.HaveOccurred(), "error updating CRIOCredentialProviderConfig 'cluster'")

	if skippedUpdate {
		e2e.Logf("matchImages has no rendered config updates, skipping waiting for MCP update")
		return
	}

	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
}

func getWorkerNodes(oc *exutil.CLI) ([]corev1.Node, error) {
	workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: `node-role.kubernetes.io/worker`,
	})
	if err != nil {
		return nil, err
	}
	e2e.Logf("Discovered %d worker nodes.", len(workerNodes.Items))
	return workerNodes.Items, nil
}

func verifyWorkerNodeCRIOCredentialProviderConfig(oc *exutil.CLI, expectedMatchImages, excludedMatchImages []string, node corev1.Node, path string, expectCRIOProviderEntry bool) {
	nodeName := node.Name
	out, err := oc.AsAdmin().Run("debug").Args("-n", debugNamespaceDefault, "node/"+nodeName, "--", "chroot", "/host", "cat", path).Output()
	e2e.Logf("%s", out)

	if !expectCRIOProviderEntry {

		if path == genericCredentialProviderConfigPath {
			o.Expect(err).To(o.HaveOccurred(), "expected error reading generic credential provider config on node %s but got none", nodeName)
			return
		}

		o.Expect(err).NotTo(o.HaveOccurred(), "error reading CRIOCredentialProviderConfig on node %s", nodeName)
		o.Expect(out).NotTo(o.ContainSubstring(crioCredentialProviderName), "expected no CRIOCredentialProviderConfig on node %s but found one", nodeName)
		return
	}

	for _, img := range expectedMatchImages {
		o.Expect(out).To(o.ContainSubstring(string(apicfgv1alpha1.MatchImage(img))), "expected match image %s not found in CRIOCredentialProviderConfig on node %s", img, nodeName)
	}
	for _, img := range excludedMatchImages {
		o.Expect(out).NotTo(o.ContainSubstring(string(apicfgv1alpha1.MatchImage(img))), "excluded match image %s found in CRIOCredentialProviderConfig on node %s", img, nodeName)
	}
}

func getCredentialProviderConfigPath(oc *exutil.CLI) string {
	cc, err := oc.AsAdmin().MachineConfigurationClient().MachineconfigurationV1().ControllerConfigs().Get(context.Background(), controllerConfigName, metav1.GetOptions{})
	if err != nil {
		e2e.Logf("could not get controllerconfig, skipping test")
		return ""
	}

	var credProviderConfigPath string

	if cc.Spec.Infra.Status.PlatformStatus == nil {
		return genericCredentialProviderConfigPath
	}

	// Determine credential provider config path based on platform
	credProviderConfigPathFormat := filepath.FromSlash("/etc/kubernetes/credential-providers/%s-credential-provider.yaml")
	switch cc.Spec.Infra.Status.PlatformStatus.Type {
	case apicfgv1.AWSPlatformType:
		credProviderConfigPath = fmt.Sprintf(credProviderConfigPathFormat, "ecr")
	case apicfgv1.GCPPlatformType:
		credProviderConfigPath = fmt.Sprintf(credProviderConfigPathFormat, "gcr")
	case apicfgv1.AzurePlatformType:
		credProviderConfigPath = fmt.Sprintf(credProviderConfigPathFormat, "acr")
	default:
		credProviderConfigPath = genericCredentialProviderConfigPath
	}
	return credProviderConfigPath
}

func createIDMSResources(oc *exutil.CLI) {
	initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, masterPool)

	idms := &apicfgv1.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "digest-mirror",
		},
		Spec: apicfgv1.ImageDigestMirrorSetSpec{
			ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{
				{
					Mirrors: []apicfgv1.ImageMirror{
						apicfgv1.ImageMirror("docker.io/qiwanredhat/mirror-pull-secret-dummy"),
					},
					Source:             "docker.io/library/nginx",
					MirrorSourcePolicy: apicfgv1.NeverContactSource,
				},
			},
		},
	}

	_, err := oc.AdminConfigClient().ConfigV1().ImageDigestMirrorSets().Create(context.Background(), idms, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "error creating ImageDigestMirrorSet %q", idms.Name)

	e2e.Logf("Created ImageDigestMirrorSet %q", idms.Name)

	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
}

func cleanupIDMSResources(oc *exutil.CLI) {
	initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, masterPool)

	err := oc.AdminConfigClient().ConfigV1().ImageDigestMirrorSets().Delete(context.Background(), "digest-mirror", metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "error deleting ImageDigestMirrorSet %q", "digest-mirror")

	e2e.Logf("Deleted ImageDigestMirrorSet %q", "digest-mirror")

	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
}

func createNamespaceRBAC(f *e2e.Framework, namespace string) {
	_, err := f.ClientSet.RbacV1().Roles(namespace).Create(context.Background(), &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "credential-provider-secret-reader",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "error creating role in namespace %q", namespace)
	e2e.Logf("Created role in namespace %q", namespace)

	_, err = f.ClientSet.RbacV1().RoleBindings(namespace).Create(context.Background(), &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "credential-provider-secret-reader-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     "system:serviceaccount:" + namespace + ":default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "credential-provider-secret-reader",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "error creating rolebinding in namespace %q", namespace)
	e2e.Logf("Created rolebinding in namespace %q", namespace)
}

func createSecret(f *e2e.Framework, namespace, name string, data map[string][]byte) {
	_, err := f.ClientSet.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: data,
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "error creating secret %q in namespace %q", name, namespace)
	e2e.Logf("Created secret %q in namespace %q", name, namespace)
}

func launchTestPod(ctx context.Context, f *e2e.Framework, podName, image, nodeName string) (*corev1.Pod, error) {
	g.By(fmt.Sprintf("launching the pod: %s on node: %s", podName, nodeName))
	contName := fmt.Sprintf("%s-container", podName)
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            contName,
					Image:           image,
					ImagePullPolicy: corev1.PullAlways,
					Command:         []string{"/bin/sh", "-c", "exit 0"},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeName:      nodeName,
		},
	}
	pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
	return pod, err
}

func matchImagesDiffer(originalImages, matchImages []string) bool {
	orig := append([]string(nil), originalImages...)
	want := append([]string(nil), matchImages...)

	sort.Strings(orig)
	sort.Strings(want)

	if len(orig) != len(want) {
		return true
	}
	for i := range orig {
		if orig[i] != want[i] {
			return true
		}
	}
	return false
}

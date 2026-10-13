package node

import (
	"context"
	"fmt"
	"os"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
)

var _ = g.Describe("[Suite:openshift/usernamespace] [sig-node] [FeatureGate:ProcMountType] [FeatureGate:UserNamespacesSupport] nested container", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("nested-podman", admissionapi.LevelBaseline)
	g.It("should pass podman localsystem test in baseline mode",
		func(ctx context.Context) {
			runNestedPod(ctx, oc)
		},
	)
})

func runNestedPod(ctx context.Context, oc *exutil.CLI) {
	g.By("updating namespace to have a userns compatible uid range")

	namespace, err := oc.AsAdmin().KubeClient().CoreV1().Namespaces().Get(ctx, oc.Namespace(), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	namespace.Annotations[securityv1.SupplementalGroupsAnnotation] = "1000/65536"
	namespace.Annotations[securityv1.UIDRangeAnnotation] = "1000/65536"
	namespace, err = oc.AsAdmin().KubeClient().CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("creating custom builder image")
	name := "baseline-nested-container"
	customImage := exutil.FixturePath("testdata", "node", "nested_container")
	err = oc.Run("new-build").Args("--binary", "--strategy=docker", fmt.Sprintf("--name=%s", name)).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	br, _ := exutil.StartBuildAndWait(oc, name, fmt.Sprintf("--from-dir=%s", customImage))
	br.AssertSuccess()

	// Use the existing ClusterRole that grants access to the nested-container SCC
	nestedContainerClusterRole := "system:openshift:scc:nested-container"
	bindUserRoleToSCC(ctx, oc, nestedContainerClusterRole, "user-nested-container-")

	g.By("creating a pod with a nested container")
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"io.kubernetes.cri-o.Devices": "/dev/fuse,/dev/net/tun",
				"openshift.io/scc":            "nested-container",
			},
		},
		Spec: corev1.PodSpec{
			HostUsers: pointer.Bool(false),
			DNSPolicy: corev1.DNSNone,
			DNSConfig: &corev1.PodDNSConfig{
				Nameservers: []string{"1.1.1.1"},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "nested-podman",
					Image:           fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/%s", namespace.Name, name),
					ImagePullPolicy: corev1.PullAlways,
					Args: []string{
						"./run_tests.sh",
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: pointer.Int64(1000),
						ProcMount: ptr.To(corev1.UnmaskedProcMount),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SETUID",
								"SETGID",
							},
						},
					},
				},
			},
		},
	}
	pod, err = oc.KubeClient().CoreV1().Pods(namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("nested-container"), "Pod should be run in nested-container SCC")

	g.By("waiting for the pod to complete")
	o.Eventually(func() error {
		_, err := oc.AsAdmin().Run("exec").Args(pod.Name, "--", "[", "-f", "done", "]").Output()
		if err != nil {
			return err
		}
		return nil
	}, "30m", "10s").Should(o.Succeed())

	// To upload test results from podman system test, use ARTIFACT_DIR env var.
	// It's not a smart way, but there's no other way to pass the artifact directory
	// to each test case.
	g.By("uploading results from podman system test")
	artifact := os.Getenv("ARTIFACT_DIR")
	_, err = oc.AsAdmin().Run("cp").Args(
		fmt.Sprintf("%s:serial-junit/report.xml", pod.Name),
		fmt.Sprintf("%s/junit/podman-system-serial.xml", artifact),
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().Run("cp").Args(
		fmt.Sprintf("%s:parallel-junit/report.xml", pod.Name),
		fmt.Sprintf("%s/junit/podman-system-parallel.xml", artifact),
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	logs, err := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace.Name).GetLogs(pod.Name, &corev1.PodLogOptions{}).Do(ctx).Raw()
	o.Expect(err).NotTo(o.HaveOccurred())

	_, err = oc.AsAdmin().Run("exec").Args(pod.Name, "--", "[", "!", "-f", "fail", "]").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("more than one of the podman system tests failed:\n%s", logs))
}

func bindUserRoleToSCC(ctx context.Context, oc *exutil.CLI, scc, name string) {
	// Create a role binding for the regular user
	g.By("Creating a role binding for the regular user")
	_, err := oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(
		ctx,
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: name,
				Namespace:    oc.Namespace(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     scc,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:     "User",
					Name:     oc.Username(),
					APIGroup: "rbac.authorization.k8s.io",
				},
			},
		},
		metav1.CreateOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
}

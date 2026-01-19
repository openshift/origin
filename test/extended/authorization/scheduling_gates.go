package authorization

import (
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	securityv1 "github.com/openshift/api/security/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:SchedulingGates] Scheduling gates", func() {
	defer g.GinkgoRecover()

	// Use LevelPrivileged to allow admin to create privileged pods
	oc := exutil.NewCLIWithPodSecurityLevel("scheduling-gates", admissionapi.LevelPrivileged)

	g.It("should allow ServiceAccount without privileged SCC access to remove schedulingGates from a privileged pod", func() {
		ctx := context.Background()
		namespace := oc.Namespace()

		g.By("Creating a ServiceAccount with only pod RBAC permissions (no privileged SCC access)")
		sa := setupServiceAccountWithPodPermissions(ctx, oc, namespace)

		g.By("Admin creating a privileged pod with scheduling gates")
		pod := createPrivilegedPodWithSchedulingGates(ctx, oc, namespace, "test-privileged-pod-with-gates")

		g.By("Verifying the pod was admitted with privileged SCC")
		o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("privileged"))

		g.By("Creating a client authenticated as the ServiceAccount")
		saClient := createClientFromServiceAccount(ctx, oc, sa)

		g.By("ServiceAccount attempting to remove scheduling gates from the privileged pod")
		updatedPod, err := saClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		updatedPod.Spec.SchedulingGates = nil

		// This should succeed because of the apiserver-library-go fix that bypasses
		// SCC admission for schedulingGates-only changes.
		// Without the fix, this would fail with: "Forbidden: not usable by user or serviceaccount"
		_, err = saClient.CoreV1().Pods(namespace).Update(ctx, updatedPod, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "ServiceAccount should be able to remove scheduling gates from privileged pod without having privileged SCC access")

		g.By("Verifying the scheduling gates were removed and pod still has privileged SCC")
		finalPod, err := saClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(finalPod.Spec.SchedulingGates).To(o.BeEmpty())
		o.Expect(finalPod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("privileged"))

		framework.Logf("Successfully demonstrated ServiceAccount without privileged SCC access can remove scheduling gates from privileged pod")
	})
})

// setupServiceAccountWithPodPermissions creates a ServiceAccount with permissions to create and update pods
func setupServiceAccountWithPodPermissions(ctx context.Context, oc *exutil.CLI, namespace string) *corev1.ServiceAccount {
	framework.Logf("Creating ServiceAccount")
	sa, err := oc.AdminKubeClient().CoreV1().
		ServiceAccounts(namespace).
		Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-sa-"},
		}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Waiting for ServiceAccount %q to be provisioned...", sa.Name)
	err = exutil.WaitForServiceAccount(
		oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace),
		sa.Name,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Creating role for pod management")
	rule := rbacv1helpers.
		NewRule("create", "update", "get").
		Groups("").
		Resources("pods").RuleOrDie()
	role, err := oc.AdminKubeClient().RbacV1().
		Roles(namespace).
		Create(ctx, &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-manager"},
			Rules:      []rbacv1.PolicyRule{rule},
		}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Creating rolebinding")
	_, err = oc.AdminKubeClient().RbacV1().
		RoleBindings(namespace).
		Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pod-manager-",
				Namespace:    namespace,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: role.Name,
			},
		}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Waiting for RBAC to propagate")
	err = wait.PollImmediate(
		framework.Poll,
		framework.PodStartTimeout, func() (bool, error) {
			review, err := oc.AdminKubeClient().
				AuthorizationV1().
				SubjectAccessReviews().
				Create(ctx, &authorizationv1.SubjectAccessReview{
					Spec: authorizationv1.SubjectAccessReviewSpec{
						User: fmt.Sprintf("system:serviceaccount:%s:%s", namespace, sa.Name),
						ResourceAttributes: &authorizationv1.ResourceAttributes{
							Verb:      "update",
							Resource:  "pods",
							Namespace: namespace,
						},
					},
				}, metav1.CreateOptions{})
			if err != nil {
				return false, nil
			}

			output, err := json.Marshal(review)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Review: %s", output)

			return review.Status.Allowed, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sa
}

// createClientFromServiceAccount creates a clientset for the ServiceAccount
func createClientFromServiceAccount(ctx context.Context, oc *exutil.CLI, sa *corev1.ServiceAccount) *kubernetes.Clientset {
	framework.Logf("Creating service account token")
	bootstrapperToken, err := oc.AdminKubeClient().CoreV1().
		ServiceAccounts(sa.Namespace).
		CreateToken(
			ctx,
			sa.Name,
			&authenticationv1.TokenRequest{},
			metav1.CreateOptions{},
		)
	o.Expect(err).NotTo(o.HaveOccurred())

	saClientConfig := restclient.AnonymousClientConfig(oc.AdminConfig())
	saClientConfig.BearerToken = bootstrapperToken.Status.Token

	return kubernetes.NewForConfigOrDie(saClientConfig)
}

// createPrivilegedPodWithSchedulingGates creates a privileged pod with scheduling gates.
// The pod requires privileged SCC and is created by admin who has access to it.
func createPrivilegedPodWithSchedulingGates(ctx context.Context, oc *exutil.CLI, namespace, name string) *corev1.Pod {
	podTemplate := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				securityv1.RequiredSCCAnnotation: "privileged",
			},
		},
		Spec: corev1.PodSpec{
			SchedulingGates: []corev1.PodSchedulingGate{
				{Name: "example.com/test-gate"},
			},
			Containers: []corev1.Container{
				{
					Name:    "pause",
					Image:   "registry.k8s.io/pause:3.9",
					Command: []string{"/pause"},
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
				},
			},
		},
	}

	// Admin creates the pod - admin has access to privileged SCC
	pod, err := oc.AdminKubeClient().CoreV1().
		Pods(namespace).
		Create(ctx, podTemplate, metav1.CreateOptions{})

	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("privileged"))
	o.Expect(pod.Spec.SchedulingGates).To(o.HaveLen(1))
	o.Expect(pod.Spec.SchedulingGates[0].Name).To(o.Equal("example.com/test-gate"))

	err = wait.PollImmediate(framework.Poll, framework.PodStartTimeout, func() (bool, error) {
		pod, err := oc.AdminKubeClient().CoreV1().
			Pods(pod.Namespace).
			Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodScheduled &&
				condition.Status == corev1.ConditionFalse &&
				condition.Reason == corev1.PodReasonSchedulingGated {
				return true, nil
			}
		}

		return false, nil
	})

	o.Expect(err).NotTo(o.HaveOccurred(), "Pod should be in SchedulingGated state")
	return pod
}

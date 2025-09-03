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

	securityv1 "github.com/openshift/api/security/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:SchedulingGates] Scheduling gates", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("scheduling-gates", admissionapi.LevelRestricted)

	g.It("should allow ServiceAccount with restricted-v2 SCC and proper RBAC to create and update pods with scheduling gates", func() {
		ctx := context.Background()
		namespace := oc.Namespace()

		sa := setupServiceAccountWithPodPermissions(ctx, oc, namespace)
		pod := createPodWithSchedulingGates(ctx, oc, namespace, "test-pod-with-gates")

		saClient := createClientFromServiceAccount(ctx, oc, sa)
		updatedPod, err := saClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Starting the actual test: removing scheduling gates from pod")
		updatedPod.Spec.SchedulingGates = nil

		_, err = saClient.CoreV1().Pods(namespace).Update(ctx, updatedPod, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "ServiceAccount should be able to update pod to remove scheduling gates")

		finalPod, err := saClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(finalPod.Spec.SchedulingGates).To(o.BeEmpty())

		o.Expect(finalPod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("restricted-v2"))

		framework.Logf("Successfully demonstrated ServiceAccount with restricted-v2 SCC can manage scheduling gates")
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

// createPodWithSchedulingGates creates a pod spec with scheduling gates and restricted-v2 SCC annotation
func createPodWithSchedulingGates(ctx context.Context, oc *exutil.CLI, namespace, name string) *corev1.Pod {
	podTemplate := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				securityv1.RequiredSCCAnnotation: "restricted-v2",
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
				},
			},
		},
	}

	pod, err := oc.AdminKubeClient().CoreV1().
		Pods(namespace).
		Create(ctx, podTemplate, metav1.CreateOptions{})

	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pod.Annotations[securityv1.ValidatedSCCAnnotation]).To(o.Equal("restricted-v2"))
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

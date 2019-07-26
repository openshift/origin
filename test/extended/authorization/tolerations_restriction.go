package authorization

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

var _ = g.Describe("[Feature: Toleration restrictions] Tolerations restrictions should be functional", func() {
	defer g.GinkgoRecover()
	f := e2e.NewDefaultFramework("tolerations-restriction")
	cs, err := e2e.LoadClientset()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.Describe("Pod with service account", func() {
		g.It(fmt.Sprintf("with enough privileges should pass"), func() {
			err := createClusterRole(cs, f.Namespace.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = createSAWithTolerationsAccess(cs, f.Namespace.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = createRoleBinding(cs, f.Namespace.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = createPodWithSA(cs, f.Namespace.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
	g.Describe("Pod without service account should fail", func() {
		g.It(fmt.Sprintf("should fail"), func() {
			err = createPodWithOutSA(cs, f.Namespace.Name)
			o.Expect(err).Should(o.HaveOccurred())
		})
	})

})

func createClusterRole(cs clientset.Interface, nsName string) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tolerations-role",
		},
		Rules: []rbacv1.PolicyRule{

			{
				APIGroups: []string{"toleration.scheduling.openshift.io"},
				Resources: []string{"node-role.kubernetes.io/master"},
				Verbs:     []string{"exists"},
			},
		},
	}
	if _, err := cs.RbacV1().Roles(nsName).Create(role); err != nil {
		return err
	}
	return nil
}

func createRoleBinding(cs clientset.Interface, nsName string) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tolerations-tolebinding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "tolerations-privilege",
			Name:     "tolerations-role",
		},
		Subjects: []rbacv1.Subject{{
			Name:      "test-sa",
			Kind:      "serviceaccount",
			Namespace: nsName,
		}},
	}
	if _, err := cs.RbacV1().RoleBindings(nsName).Create(roleBinding); err != nil {
		return err
	}
	return nil
}

func createSAWithTolerationsAccess(cs clientset.Interface, nsName string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sa",
		},
	}

	if _, err := cs.CoreV1().ServiceAccounts(nsName).Create(sa); err != nil {
		return err
	}
	return nil
}

func createPodWithSA(cs clientset.Interface, nsName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "pause",
					Image: imageutils.GetPauseImageName(),
				},
			},
			ServiceAccountName: "test-sa",
		},
	}
	if _, err := cs.CoreV1().Pods(nsName).Create(pod); err != nil {
		return err
	}
	return nil
}

func createPodWithOutSA(cs clientset.Interface, nsName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "pause",
					Image: imageutils.GetPauseImageName(),
				},
			},
		},
	}
	if _, err := cs.CoreV1().Pods(nsName).Create(pod); err != nil {
		return err
	}
	return nil
}

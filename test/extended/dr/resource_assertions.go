package dr

import (
	"context"
	"fmt"

	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/extended/util/image"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/pointer"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const postBackupNamespaceName = "etcd-backup-ns"

var (
	postBackupNamespace = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: postBackupNamespaceName},
	}

	postBackSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "post-backup-secret", Namespace: postBackupNamespaceName},
		StringData: map[string]string{"post": "backup"},
		Type:       corev1.SecretTypeOpaque,
	}

	postBackupService = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "post-backup-service", Namespace: postBackupNamespaceName},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "etcd-metrics",
					Protocol:   "TCP",
					Port:       9979,
					TargetPort: intstr.IntOrString{IntVal: 9979},
				},
			},
			Selector: map[string]string{"etcd": "true"},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	postBackupDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "post-backup-deployment", Namespace: postBackupNamespaceName},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(int32(2)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "backup-deployment"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "backup-deployment"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "post-backup-sleep-container",
							Image:   image.ShellImage(),
							Command: []string{"sleep", "infinity"},
						},
					},
				},
			},
		},
	}
)

func createPostBackupResources(oc *exutil.CLI) error {
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), &postBackupNamespace, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create post backup namespace: %w", err)
	}

	_, err = oc.AdminKubeClient().CoreV1().Secrets(postBackupNamespaceName).Create(context.Background(), &postBackSecret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create post backup secret: %w", err)
	}

	_, err = oc.AdminKubeClient().CoreV1().Services(postBackupNamespaceName).Create(context.Background(), &postBackupService, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create post backup service: %w", err)
	}

	_, err = oc.AdminKubeClient().AppsV1().Deployments(postBackupNamespaceName).Create(context.Background(), &postBackupDeployment, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create post backup deployment: %w", err)
	}

	return nil
}

func assertPostBackupResourcesAreNotFound(oc *exutil.CLI) {
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), postBackupNamespaceName, metav1.GetOptions{})
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

	_, err = oc.AdminKubeClient().CoreV1().Secrets(postBackupNamespaceName).Get(context.Background(), postBackSecret.Name, metav1.GetOptions{})
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

	_, err = oc.AdminKubeClient().CoreV1().Services(postBackupNamespaceName).Get(context.Background(), postBackupService.Name, metav1.GetOptions{})
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

	_, err = oc.AdminKubeClient().AppsV1().Deployments(postBackupNamespaceName).Get(context.Background(), postBackupDeployment.Name, metav1.GetOptions{})
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

	// TODO(thomas): ideally we also would want to find left-over containers by id in cri-o, which was too complex so we trust the API instead
	pods, err := e2epod.GetPods(context.Background(), oc.AdminKubeClient(), postBackupNamespaceName, postBackupDeployment.Spec.Selector.MatchLabels)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(len(pods)).To(o.BeZero())
}

func assertPostBackupResourcesAreStillFound(oc *exutil.CLI) {
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), postBackupNamespaceName, metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	_, err = oc.AdminKubeClient().CoreV1().Secrets(postBackupNamespaceName).Get(context.Background(), postBackSecret.Name, metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	_, err = oc.AdminKubeClient().CoreV1().Services(postBackupNamespaceName).Get(context.Background(), postBackupService.Name, metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	_, err = oc.AdminKubeClient().AppsV1().Deployments(postBackupNamespaceName).Get(context.Background(), postBackupDeployment.Name, metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
}

func assertPostBackupResourcesAreStillFunctional(oc *exutil.CLI) {
	d, err := oc.AdminKubeClient().AppsV1().Deployments(postBackupNamespaceName).Get(context.Background(), postBackupDeployment.Name, metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(*d.Spec.Replicas).To(o.Equal(*postBackupDeployment.Spec.Replicas))

	pods, err := e2epod.GetPods(context.Background(), oc.AdminKubeClient(), postBackupNamespaceName, postBackupDeployment.Spec.Selector.MatchLabels)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(len(pods)).To(o.Equal(int(*postBackupDeployment.Spec.Replicas)))
}

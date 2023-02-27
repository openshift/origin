package cpu_partitioning

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-node] CPU Partitioned cluster workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc  = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx = context.Background()
	)

	g.BeforeEach(func() {
		skipNonCPUPartitionedCluster(oc)
	})

	g.Context("in annotated namespaces", func() {

		var (
			deploymentLabels = map[string]string{"app": "workload-pinned"}
			namespace        = "workload-pinning"
			name             = "workload"
		)

		g.AfterEach(func() {
			o.Expect(cleanup(oc, namespace)).NotTo(o.HaveOccurred())
		})

		g.It("should be modified", func() {

			e := createNamespace(oc, namespace, namespaceAnnotation)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating namespace %s", namespace)

			e = createDeployment(oc, name, namespace, deploymentLabels, deploymentPodAnnotation)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating pinned deployment")

			pods, err := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=workload-pinned",
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, pod := range pods.Items {
				o.Expect(pod.Annotations).To(
					o.And(
						o.HaveKey(workloadAnnotations),
						o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex)),
					), "pod (%s/%s) does not contain correct annotations", pod.Namespace, pod.Name)

				for _, container := range pod.Spec.Containers {
					_, ok := container.Resources.Limits[resourceLabel]
					o.Expect(ok).To(o.BeTrue())
					_, ok = container.Resources.Requests[resourceLabel]
					o.Expect(ok).To(o.BeTrue())
				}
			}
		})
	})

	g.Context("in non-annotated namespaces", func() {

		var (
			deploymentLabels = map[string]string{"app": "workload"}
			namespace        = "workload-non-pinning"
			name             = "workload"
		)

		g.AfterEach(func() {
			o.Expect(cleanup(oc, namespace)).NotTo(o.HaveOccurred())
		})

		g.It("should not be modified", func() {

			e := createNamespace(oc, namespace, nil)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating namespace %s", namespace)

			e = createDeployment(oc, name, namespace, deploymentLabels, nil)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating pinned deployment")

			pods, err := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=workload",
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, pod := range pods.Items {
				o.Expect(pod.Annotations).NotTo(
					o.And(
						o.HaveKey(workloadAnnotations),
						o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex)),
					), "pod (%s/%s) does not contain correct annotations", pod.Namespace, pod.Name)

				for _, container := range pod.Spec.Containers {
					_, ok := container.Resources.Limits[resourceLabel]
					o.Expect(ok).To(o.BeFalse())
					_, ok = container.Resources.Requests[resourceLabel]
					o.Expect(ok).To(o.BeFalse())
				}
			}
		})
	})
})

func cleanup(oc *exutil.CLI, namespace string) error {
	ctx := context.Background()
	delOpts := metav1.DeleteOptions{}
	return oc.KubeClient().CoreV1().Namespaces().Delete(ctx, namespace, delOpts)
}

func createNamespace(oc *exutil.CLI, name string, annotations map[string]string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
	_, err := oc.KubeClient().CoreV1().
		Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	return err
}

func createDeployment(oc *exutil.CLI, name, namespace string, depLabels map[string]string, podAnnotations map[string]string) error {
	deployment := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    depLabels,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: depLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      depLabels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "builder",
					Containers: []corev1.Container{
						{
							Name:  "busy-work",
							Image: image.ShellImage(),
							Command: []string{
								"/bin/bash",
								"-c",
								`while true; do echo "Busy working, cycling through the ones and zeros"; sleep 5; done`,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := oc.KubeClient().AppsV1().
		Deployments(namespace).
		Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = exutil.WaitForPods(
		oc.KubeClient().CoreV1().Pods(namespace),
		labels.SelectorFromSet(depLabels),
		exutil.CheckPodIsRunning, 1, time.Minute*2,
	)
	return err
}

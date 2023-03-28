package cpu_partitioning

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	psapi "k8s.io/pod-security-admission/api"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	ocpv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc                      = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx                     = context.Background()
		isClusterCPUPartitioned = false
	)

	g.BeforeEach(func() {
		isClusterCPUPartitioned = getCpuPartitionedStatus(oc) == ocpv1.CPUPartitioningAllNodes
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

		g.It("should be modified if CPUPartitioningMode = AllNodes", func() {

			e := createNamespace(oc, namespace, namespaceAnnotation)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating namespace %s", namespace)

			e = createDeployment(oc, name, namespace, deploymentLabels, deploymentPodAnnotation)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating pinned deployment")

			_, e = exutil.WaitForPods(
				oc.KubeClient().CoreV1().Pods(namespace),
				labels.SelectorFromSet(deploymentLabels),
				exutil.CheckPodIsRunning, 1, time.Minute*3,
			)
			o.Expect(e).ToNot(o.HaveOccurred(), "error waiting for pod")

			pods, err := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=workload-pinned",
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			matcher := o.And(
				o.HaveKey(workloadAnnotations),
				o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex)),
			)

			matcher, messageFormat := adjustMatcherAndMessageForCluster(isClusterCPUPartitioned, matcher)

			for _, pod := range pods.Items {

				o.Expect(pod.Annotations).To(
					matcher, "pod (%s/%s) %s annotations", pod.Namespace, pod.Name, messageFormat)

				for _, container := range pod.Spec.Containers {
					_, ok := container.Resources.Limits[resourceLabel]
					o.Expect(ok).To(o.Equal(isClusterCPUPartitioned),
						"limits resources %s be present for container %s in pod %s/%s", messageFormat, container.Name, pod.Name, pod.Namespace)
					_, ok = container.Resources.Requests[resourceLabel]
					o.Expect(ok).To(o.Equal(isClusterCPUPartitioned),
						"requests resources %s be present for container %s in pod %s/%s", messageFormat, container.Name, pod.Name, pod.Namespace)
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

		g.It("should not be allowed if CPUPartitioningMode = AllNodes", func() {

			e := createNamespace(oc, namespace, nil)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating namespace %s", namespace)

			e = createDeployment(oc, name, namespace, deploymentLabels, deploymentPodAnnotation)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating pinned deployment")

			switch {
			case !isClusterCPUPartitioned:
				_, e = exutil.WaitForPods(
					oc.KubeClient().CoreV1().Pods(namespace),
					labels.SelectorFromSet(deploymentLabels),
					exutil.CheckPodIsRunning, 1, time.Minute*3,
				)
				o.Expect(e).ToNot(o.HaveOccurred(), "error waiting for pod")
			default:
				failureMessage := ""
				// Sometimes querying a deployment can be faster than it takes to update the status of that deployment,
				// we poll here to avoid that scenario.
				err := wait.Poll(3*time.Second, time.Second*30, func() (bool, error) {
					d, e := oc.KubeClient().AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
					if e != nil {
						return false, e
					}
					for _, condition := range d.Status.Conditions {
						if condition.Reason == "FailedCreate" {
							failureMessage = condition.Message
							return true, nil
						}
					}
					return false, nil
				})
				o.Expect(err).ToNot(o.HaveOccurred(), "error getting deployment")
				o.Expect(failureMessage).To(
					o.ContainSubstring(
						"is forbidden: autoscaling.openshift.io/ManagementCPUsOverride the pod namespace \"%s\" does not allow the workload type",
						namespace,
					))
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
			Labels: map[string]string{
				psapi.AuditLevelLabel:   string(psapi.LevelRestricted),
				psapi.EnforceLevelLabel: string(psapi.LevelRestricted),
				psapi.WarnLevelLabel:    string(psapi.LevelRestricted),
			},
		},
	}
	_, err := oc.KubeClient().CoreV1().
		Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return exutil.WaitForServiceAccountWithSecret(oc.AdminKubeClient().CoreV1().ServiceAccounts(name), "builder")
}

func createDeployment(oc *exutil.CLI, name, namespace string, depLabels map[string]string, podAnnotations map[string]string) error {
	zero := int64(0)
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
					TerminationGracePeriodSeconds: &zero,
					ServiceAccountName:            "builder",
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
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
	return err
}

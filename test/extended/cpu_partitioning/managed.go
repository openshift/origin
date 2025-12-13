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

		g.It("should be modified if CPUPartitioningMode = AllNodes", g.Label("Size:M"), func() {

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

		g.It("should be allowed if CPUPartitioningMode = AllNodes with a warning annotation", g.Label("Size:M"), func() {

			e := createNamespace(oc, namespace, nil)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating namespace %s", namespace)

			e = createDeployment(oc, name, namespace, deploymentLabels, deploymentPodAnnotation)
			o.Expect(e).ToNot(o.HaveOccurred(), "error creating pinned deployment")

			_, e = exutil.WaitForPods(
				oc.KubeClient().CoreV1().Pods(namespace),
				labels.SelectorFromSet(deploymentLabels),
				exutil.CheckPodIsRunning, 1, time.Minute*3,
			)
			o.Expect(e).ToNot(o.HaveOccurred(), "error waiting for pod")

			pods, e := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=workload",
			})
			o.Expect(e).ToNot(o.HaveOccurred(), "error getting pods")
			for _, pod := range pods.Items {
				val, ok := pod.GetAnnotations()["workload.openshift.io/warning"]
				if isClusterCPUPartitioned {
					o.Expect(ok).To(o.BeTrue(), "expected warning annotation to be present")
					o.Expect(val).To(o.ContainSubstring("namespace is not annotated with workload.openshift.io/allowed"))
				} else {
					o.Expect(ok).To(o.BeFalse(), "expected warning annotation to not be present")
				}
			}

		})
	})

	g.Context("with limits", func() {

		var (
			managedOC = exutil.NewCLI("cpu-partitioning").SetManagedNamespace().AsAdmin()
		)

		g.AfterEach(func() {
			o.Expect(cleanup(managedOC, managedOC.Namespace())).To(o.Succeed())
		})

		g.It("should have resources modified if CPUPartitioningMode = AllNodes", g.Label("Size:M"), func() {

			requests := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			}
			limits := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("30m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			}
			deployment, err := createManagedDeployment(managedOC, requests, limits)
			o.Expect(err).ToNot(o.HaveOccurred(), "error creating deployment with cpu limits")

			_, err = exutil.WaitForPods(
				managedOC.KubeClient().CoreV1().Pods(managedOC.AsAdmin().Namespace()),
				labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
				exutil.CheckPodIsRunning, 1, time.Minute*3,
			)
			o.Expect(err).ToNot(o.HaveOccurred(), "error waiting for pod")

			pods, err := managedOC.KubeClient().CoreV1().Pods(managedOC.Namespace()).List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			matcher := o.And(
				o.HaveKey(workloadAnnotations),
				o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex)),
			)

			matcher, messageFormat := adjustMatcherAndMessageForCluster(isClusterCPUPartitioned, matcher)

			for _, pod := range pods.Items {

				o.Expect(pod.Annotations).To(
					matcher, "pod (%s/%s) %s annotations", pod.Namespace, pod.Name, messageFormat)

				containerAnnotationResources, err := getWorkloadAnnotationResource(pod.Annotations)
				o.Expect(err).ToNot(o.HaveOccurred())

				for _, container := range pod.Spec.Containers {

					resource, found := containerAnnotationResources[container.Name]
					o.Expect(found).To(o.Equal(isClusterCPUPartitioned))
					if isClusterCPUPartitioned {
						o.Expect(resource.CPULimit).To(o.Equal(limits.Cpu().MilliValue()), "container %s is missing a cpulimit", container.Name)
					}

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
	return exutil.WaitForServiceAccountWithSecret(oc.AdminConfigClient().ConfigV1().ClusterVersions(), oc.AdminKubeClient().CoreV1().ServiceAccounts(name), "builder")
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

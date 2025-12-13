package pods

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	authv1 "github.com/openshift/api/authorization/v1"
	projv1 "github.com/openshift/api/project/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	successPodName          = "success-grace-period-pod"
	errorPodName            = "error-grace-period-pod"
	namespace               = "graceful-shutdown-testbed"
	serviceAccountName      = "graceful-shutdown"
	hostPath                = "/var/graceful-shutdown"
	nodeReadyTimeout        = 15 * time.Minute
	nodeLabelSelectorWorker = "node-role.kubernetes.io/worker,!node-role.kubernetes.io/edge"
)

var (
	testPodLabel = map[string]string{"graceful-restart-helper": ""}
)

const (
	busyScript = `
#!/bin/bash
while true
do
	echo "Busy working, cycling through the ones and zeros"
	sleep 5
done
	`
	preStopScript = `
#!/bin/bash

sleepTime=%0.f
filePath=%s

sleep $sleepTime

if [ ! -f $filePath ]
then
	echo "writing $filePath"
	echo "finished" >> $filePath
else
	echo "file exists, skipping"
fi
echo "done"
exit 0
`
	debugScript = `
#!/bin/bash

hostPath=%s
touch /host/$hostPath/completed-debug-pod
ls /host/$hostPath
`
)

var _ = Describe("[sig-node][Disruptive][Feature:KubeletGracefulShutdown]", func() {

	var (
		oc           = exutil.NewCLIWithoutNamespace("pod").AsAdmin()
		node         *corev1.Node
		successPod   *corev1.Pod
		errorPod     *corev1.Pod
		outputString string
	)

	It("Kubelet with graceful shutdown configuration should respect pods termination grace period", Label("Size:L"), func() {
		ctx := context.Background()

		createTestBed(ctx, oc)

		By("getting first worker node", func() {
			nodes, err := oc.KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: nodeLabelSelectorWorker})
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.Items).NotTo(HaveLen(0))
			node = &nodes.Items[0]
		})

		By("creating two pods with in range and out of range of grace periods", func() {
			// Currently hard coded values
			// TODO: Add dynamic support for current existing kubelet config.
			// TODO: Maybe read rendered config ex: renderedConfigName := node.Labels["machineconfiguration.openshift.io/currentConfig"]
			successPod = gracefulPodSpec(successPodName, node.Name, time.Minute*2)
			errorPod = gracefulPodSpec(errorPodName, node.Name, time.Minute*10)
			// Create two test pods
			_, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, successPod, metav1.CreateOptions{})
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			_, err = oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, errorPod, metav1.CreateOptions{})
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
			// Wait for pods to be running
			_, err = exutil.WaitForPods(
				oc.KubeClient().CoreV1().Pods(namespace),
				labels.SelectorFromSet(testPodLabel),
				exutil.CheckPodIsRunning, 2, time.Second*60)
			Expect(err).NotTo(HaveOccurred(), "unable to wait for pods")
		})

		By("triggering node reboot", func() {
			// Reboot node
			err := triggerReboot(oc, node.Name)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
			// Wait for node to be rebooted before examining with debug pod
			isReadyBeforeTimeout := e2enode.WaitForNodeToBeReady(ctx, oc.KubeFramework().ClientSet, node.Name, nodeReadyTimeout)
			Expect(isReadyBeforeTimeout).To(BeTrue(), "node was not ready before timeout %s", nodeReadyTimeout)
		})

		By("creating debug pod to gather files on the node", func() {
			// We only need to query the node once and `ls` the desired directory.
			// We can then determine the correct behavior by which file was written.
			err := wait.Poll(time.Second, time.Second*60, func() (done bool, err error) {
				outputString, err = debugPod(oc, node.Name)
				// If output contains the below errors, retry
				if err != nil && strings.Contains(outputString, "unable to create the debug pod") {
					return false, nil
				}
				return true, err
			})
			Expect(err).NotTo(HaveOccurred(), "was not able to ls directory %s", hostPath)
		})

		Expect(outputString).To(ContainSubstring("%s", successPodName))

		Expect(outputString).NotTo(ContainSubstring("%s", errorPodName))
	})

	AfterEach(func() {
		deleteTestBed(context.Background(), oc)
	})
})

func filePathForPod(podName string) string {
	return fmt.Sprintf("%s/completed-%s", hostPath, podName)
}

func debugPod(oc *exutil.CLI, nodeName string) (output string, err error) {
	name := "debug"
	script := fmt.Sprintf(debugScript, hostPath)
	pod := adminPod(name, nodeName, script)
	ctx := context.Background()
	_, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return
	}

	_, err = exutil.WaitForPods(
		oc.AdminKubeClient().CoreV1().Pods(namespace),
		labels.SelectorFromSet(map[string]string{"app": name}),
		func(p corev1.Pod) bool {
			return p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded
		}, 1, time.Minute*5)
	if err != nil {
		return
	}

	byteOutput, logError := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{}).DoRaw(ctx)
	output = string(byteOutput)
	err = logError
	return
}

// Force a node reboot
func triggerReboot(oc *exutil.CLI, nodeName string) error {
	name := "reboot"
	command := "exec chroot /host systemctl reboot"
	pod := adminPod("reboot", nodeName, command)
	_, err := oc.AsAdmin().KubeClient().CoreV1().
		Pods(namespace).
		Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	_, err = exutil.WaitForPods(
		oc.AdminKubeClient().CoreV1().Pods(namespace),
		labels.SelectorFromSet(map[string]string{"app": name}),
		func(p corev1.Pod) bool {
			return p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded
		}, 1, time.Minute*2)
	return err
}

func adminPod(podName, nodeName, script string) *corev1.Pod {
	isTrue := true
	zero := int64(0)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"app": podName},
		},
		Spec: corev1.PodSpec{
			HostPID:            true,
			RestartPolicy:      corev1.RestartPolicyNever,
			NodeName:           nodeName,
			ServiceAccountName: serviceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: podName,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &zero,
						Privileged: &isTrue,
					},
					Image: image.ShellImage(),
					Command: []string{
						"/bin/bash",
						"-c",
						script,
					},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/host",
							Name:      "host",
						},
					},
				},
			},
		},
	}
}

// Generate a pod spec with the termination grace period specified, and busy work lasting a little less
// then the specified grace period
// The pod will write to the node file system if allowed to fully complete
func gracefulPodSpec(name, nodeName string, gracePeriod time.Duration) *corev1.Pod {
	gracePeriodSecond := int64(gracePeriod.Seconds())
	busyWorkTime := gracePeriod - (time.Second * 20)
	hostDirType := corev1.HostPathDirectoryOrCreate
	isTrue := true
	isZero := int64(0)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: testPodLabel,
		},
		Spec: corev1.PodSpec{
			HostPID:            true,
			RestartPolicy:      corev1.RestartPolicyNever,
			PriorityClassName:  "system-cluster-critical",
			ServiceAccountName: serviceAccountName,
			NodeName:           nodeName,
			Containers: []corev1.Container{
				{
					Image: image.ShellImage(),
					Name:  name,
					Command: []string{
						"/bin/bash",
						"-c",
						busyScript,
					},
					Lifecycle: &corev1.Lifecycle{
						PreStop: &corev1.LifecycleHandler{
							Exec: &corev1.ExecAction{
								Command: []string{
									"/bin/bash",
									"-c",
									fmt.Sprintf(preStopScript, busyWorkTime.Seconds(), filePathForPod(name)),
								},
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						RunAsUser:  &isZero,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host",
							MountPath: hostPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &hostDirType,
						},
					},
				},
			},
			TerminationGracePeriodSeconds: &gracePeriodSecond,
		},
	}
}

// Helper methods to create test bed

func createTestBed(ctx context.Context, oc *exutil.CLI) {
	err := callProject(ctx, oc, true)
	Expect(err).NotTo(HaveOccurred())
	err = callServiceAccount(ctx, oc, true)
	Expect(err).NotTo(HaveOccurred())
	err = callRBAC(ctx, oc, true)
	Expect(err).NotTo(HaveOccurred())
	err = exutil.WaitForServiceAccountWithSecret(
		oc.AdminConfigClient().ConfigV1().ClusterVersions(),
		oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace),
		serviceAccountName)
	Expect(err).NotTo(HaveOccurred())
}

func deleteTestBed(ctx context.Context, oc *exutil.CLI) {
	err := callRBAC(ctx, oc, false)
	Expect(err).NotTo(HaveOccurred())
	err = callServiceAccount(ctx, oc, false)
	Expect(err).NotTo(HaveOccurred())
	err = callProject(ctx, oc, false)
	Expect(err).NotTo(HaveOccurred())
}

func callRBAC(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &authv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "system:openshift:scc:privileged",
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}

	client := oc.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callServiceAccount(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	client := oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace)
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callProject(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &projv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/audit":   "privileged",
				"pod-security.kubernetes.io/enforce": "privileged",
				"pod-security.kubernetes.io/warn":    "privileged",
			},
		},
	}

	client := oc.AsAdmin().ProjectClient().ProjectV1().Projects()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

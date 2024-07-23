package debug

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"

	"github.com/openshift-kni/commatrix/client"
)

type DebugPod struct {
	Name      string
	Namespace string
	NodeName  string
}

const (
	interval = 1 * time.Second
	timeout  = 2 * time.Minute
)

// New creates debug pod on the given node, puts it in infinite sleep,
// and returns the DebugPod object. Use the Clean() method to delete it.
func New(cs *client.ClientSet, node string, namespace string, image string) (*DebugPod, error) {
	if namespace == "" {
		return nil, errors.New("failed creating new debug pod: got empty namespace")
	}

	pod, err := createPodAndWait(cs, interval, timeout, node, namespace, image)
	if err != nil {
		return nil, err
	}

	return &DebugPod{
		Name:      pod.Name,
		Namespace: namespace,
		NodeName:  node}, nil
}

func (dp *DebugPod) Exec(cmd string) ([]byte, error) {
	cmdOnDebugPod := append([]string{"exec", "-n", dp.Namespace, dp.Name, "--", "chroot", "/host"}, strings.Split(cmd, " ")...)
	out, err := exec.Command("oc", cmdOnDebugPod...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to exec command \"%s\" on node %s: %v\n%s", cmd, dp.NodeName, err, string(out))
	}

	return out, nil
}

func (dp *DebugPod) ExecWithRetry(cmd string, interval time.Duration, duration time.Duration) ([]byte, error) {
	out := []byte{}
	execErr := errors.New("")

	if err := wait.PollUntilContextTimeout(context.TODO(), interval, duration, true, func(ctx context.Context) (bool, error) {
		out, execErr = dp.Exec(cmd)
		if execErr != nil {
			return false, execErr
		}

		return true, execErr
	}); err != nil {
		return nil, errors.Join(err, execErr)
	}

	return out, nil
}

// Clean deletes the debug pod and his namespace.
func (dp *DebugPod) Clean() error {
	output, err := exec.Command("oc", "delete", "pod", "-n", dp.Namespace, dp.Name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed deleting debug pod %s/%s: %v\n%s", dp.Namespace, dp.Name, err, string(output))
	}

	return nil
}

func (dp *DebugPod) String() string {
	return fmt.Sprintf(dp.Name)
}

func createPodAndWait(cs *client.ClientSet, interval time.Duration, timeout time.Duration, node string, namespace string, image string) (*corev1.Pod, error) {
	pod, err := createPod(cs, node, namespace, image)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug pod: %w", err)
	}

	err = waitPodPhase(cs, interval, timeout, pod, corev1.PodRunning)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for debug pod to be ready: %w", err)
	}

	return pod, nil
}

func waitPodPhase(cs *client.ClientSet, interval time.Duration, timeout time.Duration, pod *corev1.Pod, phase corev1.PodPhase) error {
	getErr := errors.New("")
	err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(ctx context.Context) (bool, error) {
		pod, getErr := cs.Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})

		if k8serrors.IsNotFound(getErr) {
			return false, getErr
		}

		if getErr != nil {
			return true, getErr
		}

		if pod.Status.Phase != phase {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return errors.Join(err, getErr)
	}

	return nil
}

func createPod(cs *client.ClientSet, node string, namespace string, image string) (*corev1.Pod, error) {
	podDef := getPodDefinition(node, namespace, image)
	pod, err := cs.Pods(namespace).Create(context.TODO(), podDef, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func getPodDefinition(node string, namespace string, image string) *corev1.Pod {
	podName := fmt.Sprintf("%s-debug-", strings.ReplaceAll(node, ".", "-"))
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"debug.openshift.io/source-container": "container-00",
				"debug.openshift.io/source-resource":  fmt.Sprintf("/v1, Resource=nodes/%s", node),
				"openshift.io/scc":                    "privileged",
			},
			Namespace:    namespace,
			GenerateName: podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "container-00",
					Command: []string{"/bin/sh"},
					Env: []corev1.EnvVar{
						{
							Name:  "TMOUT",
							Value: "900",
						},
					},
					Image: image,
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To[bool](true),
						RunAsUser:  ptr.To[int64](0),
					},
					Stdin:                    true,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessagePolicy("File"),
					TTY:                      true,
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/host",
							Name:      "host",
						},
						{
							MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
							Name:      "kube-api-access",
							ReadOnly:  true,
						},
					},
				},
			},
			DNSPolicy:                     corev1.DNSClusterFirst,
			EnableServiceLinks:            ptr.To[bool](true),
			HostIPC:                       true,
			HostNetwork:                   true,
			HostPID:                       true,
			NodeName:                      node,
			PreemptionPolicy:              ptr.To[corev1.PreemptionPolicy](corev1.PreemptLowerPriority),
			Priority:                      ptr.To[int32](1000000000),
			PriorityClassName:             "openshift-user-critical",
			RestartPolicy:                 corev1.RestartPolicyNever,
			SchedulerName:                 "default-scheduler",
			ServiceAccountName:            "default",
			TerminationGracePeriodSeconds: ptr.To[int64](30),
			Tolerations: []corev1.Toleration{
				{
					Effect:            corev1.TaintEffectNoExecute,
					Key:               "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					TolerationSeconds: ptr.To[int64](300),
				},
				{
					Effect:            corev1.TaintEffectNoExecute,
					Key:               "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					TolerationSeconds: ptr.To[int64](300),
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
							Type: ptr.To[corev1.HostPathType](corev1.HostPathDirectory),
						},
					},
				},
				{
					Name: "kube-api-access",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: ptr.To[int32](420),
							Sources: []corev1.VolumeProjection{
								{
									ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
										ExpirationSeconds: ptr.To[int64](3607),
										Path:              corev1.ServiceAccountTokenKey,
									},
								},
								{
									ConfigMap: &corev1.ConfigMapProjection{
										Items: []corev1.KeyToPath{
											{
												Key:  "ca.crt",
												Path: "ca.crt",
											},
										},
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-root-ca.crt",
										},
									},
								},
								{
									DownwardAPI: &corev1.DownwardAPIProjection{
										Items: []corev1.DownwardAPIVolumeFile{
											{
												FieldRef: &corev1.ObjectFieldSelector{
													APIVersion: "v1",
													FieldPath:  "metadata.namespace",
												},
												Path: "namespace",
											},
										},
									},
								},
								{
									ConfigMap: &corev1.ConfigMapProjection{
										Items: []corev1.KeyToPath{
											{
												Key:  "service-ca.crt",
												Path: "service-ca.crt",
											},
										},
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "openshift-service-ca.crt",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func CreateNamespace(cs *client.ClientSet, namespace string) error {
	ns := getNamespaceDefinition(namespace)
	_, err := cs.Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed creating namespace %s: %v", namespace, err)
	}

	return nil
}

func DeleteNamespace(cs *client.ClientSet, namespace string) error {
	err := cs.Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed deleting namespace %s: %v", namespace, err)
	}

	return nil
}

func getNamespaceDefinition(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/audit":   "privileged",
				"pod-security.kubernetes.io/enforce": "privileged",
				"pod-security.kubernetes.io/warn":    "privileged",
			},
		},
	}
}

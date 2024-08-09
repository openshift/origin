package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
	clientOptions "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/pkg/client"
)

//go:generate ../../bin/mockgen -destination mock/mock_utils.go -source utils.go
type UtilsInterface interface {
	CreateNamespace(namespace string) error
	DeleteNamespace(namespace string) error
	CreatePodOnNode(nodeName, namespace, image string) (pod *corev1.Pod, err error)
	DeletePod(pod *corev1.Pod) error
	RunCommandOnPod(pod *corev1.Pod, command []string) ([]byte, error)
	WriteFile(path string, data []byte) error
}

type utils struct {
	*client.ClientSet
}

const (
	interval = 1 * time.Second
	timeout  = 10 * time.Minute
)

func New(c *client.ClientSet) UtilsInterface {
	return &utils{c}
}

func (u *utils) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func (u *utils) CreateNamespace(namespace string) error {
	ns := getNamespaceDefinition(namespace)
	err := u.Create(context.TODO(), ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed creating namespace %s: %v", namespace, err)
	}

	return nil
}

func (u *utils) DeleteNamespace(namespace string) error {
	ns := getNamespaceDefinition(namespace)
	err := u.Delete(context.TODO(), ns)
	if err != nil {
		return fmt.Errorf("failed deleting namespace %s: %v", namespace, err)
	}

	return nil
}

func (u *utils) CreatePodOnNode(nodeName, namespace, image string) (*corev1.Pod, error) {
	pod := getPodDefinition(nodeName, namespace, image)
	err := u.Create(context.TODO(), pod)
	if err != nil {
		return nil, err
	}
	err = wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(ctx context.Context) (bool, error) {
		err := u.Get(context.TODO(), clientOptions.ObjectKey{Name: pod.Name, Namespace: namespace}, pod)

		if k8serrors.IsNotFound(err) {
			return false, err
		}

		if err != nil {
			return true, err
		}

		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (u *utils) DeletePod(pod *corev1.Pod) error {
	return u.Delete(context.TODO(), pod)
}

func (u *utils) RunCommandOnPod(pod *corev1.Pod, command []string) ([]byte, error) {
	containerName := pod.Spec.Containers[0].Name

	// ExecCommand runs command in the pod's first container and returns buffer output
	var buf bytes.Buffer
	req := u.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(u.Config, "POST", req.URL())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("cannot create SPDY executor for req %s: %w", req.URL().String(), err)
	}

	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: &buf,
		Stderr: os.Stderr,
		Tty:    true,
	})
	if err != nil {
		return buf.Bytes(), fmt.Errorf("remote command %v error [%w]. output [%s]", command, err, buf.String())
	}

	return buf.Bytes(), nil
}

func getPodDefinition(node string, namespace string, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"openshift.io/scc": "privileged",
			},
			Namespace:    namespace,
			GenerateName: "debug-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container",
					Command: []string{
						"/bin/sh",
						"-c",
						"sleep INF",
					},
					Image: image,
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To[bool](true),
						RunAsUser:  ptr.To[int64](0),
					},
				},
			},
			HostNetwork:                   true,
			HostPID:                       true,
			NodeName:                      node,
			PriorityClassName:             "openshift-user-critical",
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To[int64](1),
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
		},
	}
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

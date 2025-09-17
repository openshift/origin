package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
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
	CreatePodOnNode(nodeName, namespace, image string, command []string) (pod *corev1.Pod, err error)
	DeletePod(pod *corev1.Pod) error
	RunCommandOnPod(pod *corev1.Pod, command []string) ([]byte, error)
	GetPodLogs(namespace string, pod *corev1.Pod) (string, error)
	WriteFile(path string, data []byte) error
	GetPlatformType() (configv1.PlatformType, error)
	IsSNOCluster() (bool, error)
	WaitForPodStatus(namespace string, pod *corev1.Pod, PodPhase corev1.PodPhase) error
	IsIPv6Enabled() (bool, error)
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

func (u *utils) resolveImageStreamTagString(s string) (string, error) {
	namespace, name, tag := parseImageStreamTagString(s)
	if len(namespace) == 0 {
		return "", fmt.Errorf("expected namespace/name:tag")
	}
	return u.resolveImageStreamTag(namespace, name, tag)
}

func parseImageStreamTagString(s string) (string, string, string) {
	var namespace, nameAndTag string
	parts := strings.SplitN(s, "/", 2)
	switch len(parts) {
	case 2:
		namespace = parts[0]
		nameAndTag = parts[1]
	case 1:
		nameAndTag = parts[0]
	}
	name, tag, _ := imageutil.SplitImageStreamTag(nameAndTag)
	return namespace, name, tag
}

func (u *utils) resolveImageStreamTag(namespace, name, tag string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	imageStream, err := u.ImageClient.ImageStreams(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var image string
	if image, _, _, _, err = imageutil.ResolveRecentPullSpecForTag(imageStream, tag, false); err != nil {
		return "", fmt.Errorf("unable to resolve the imagestream tag %s/%s:%s: %v", namespace, name, tag, err)
	}
	return image, nil
}

func (u *utils) CreateNamespace(namespace string) error {
	ns := getNamespaceDefinition(namespace)
	err := u.Create(context.TODO(), ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed creating namespace %s: %v", namespace, err)
	}

	// wait for it to be ready with the openshift.io/sa.scc.uid-range Annotation
	err = u.waitForNamespaceSCCUAnnotation(ns)
	if err != nil {
		return fmt.Errorf("failed to wait for annotation 'openshift.io/sa.scc.uid-range' in namespace %s: %v", namespace, err)
	}

	return nil
}

func (u *utils) waitForNamespaceSCCUAnnotation(ns *corev1.Namespace) error {
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(ctx context.Context) (bool, error) {
		err := u.Get(context.TODO(), clientOptions.ObjectKey{Name: ns.Name}, ns)
		if err != nil {
			return false, err
		}
		_, found := ns.Annotations["openshift.io/sa.scc.uid-range"]
		return found, nil
	})
}

func (u *utils) DeleteNamespace(namespace string) error {
	ns := getNamespaceDefinition(namespace)
	err := u.Delete(context.TODO(), ns)
	if err != nil {
		return fmt.Errorf("failed deleting namespace %s: %v", namespace, err)
	}

	return nil
}

func (u *utils) WaitForPodStatus(namespace string, pod *corev1.Pod, PodPhase corev1.PodPhase) error {
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(ctx context.Context) (bool, error) {
		err := u.Get(context.TODO(), clientOptions.ObjectKey{Name: pod.Name, Namespace: namespace}, pod)

		if k8serrors.IsNotFound(err) {
			return false, err
		}

		if err != nil {
			return true, err
		}
		if pod.Status.Phase != PodPhase {
			return false, nil
		}

		return true, nil
	})
}

func (u *utils) CreatePodOnNode(nodeName, namespace, image string, command []string) (*corev1.Pod, error) {
	image, err := u.resolveImageStreamTagString(image)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve image: %w", err)
	}
	pod := getPodDefinition(nodeName, namespace, image, command)
	if err := u.Create(context.TODO(), pod); err != nil {
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
	req := u.CoreV1Interface.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(u.Config, "POST", req.URL())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("cannot create SPDY executor for req %s: %w", req.URL().String(), err)
	}

	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdout: &buf,
		Stderr: os.Stderr,
	})
	if err != nil {
		return buf.Bytes(), fmt.Errorf("remote command %v error [%w]. output [%s]", command, err, buf.String())
	}

	return buf.Bytes(), nil
}

func getPodDefinition(node string, namespace string, image string, command []string) *corev1.Pod {
	tolerationSeconds := int64(300)

	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", "sleep INF"}
	}

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
					Name:    "container",
					Command: command,
					Image:   image,
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
						RunAsUser:  ptr.To(int64(0)),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-root",
							MountPath: "/host",
						},
					},
				},
			},
			HostNetwork:                   true,
			HostPID:                       true,
			NodeName:                      node,
			PriorityClassName:             "openshift-user-critical",
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To(int64(1)),
			Tolerations: []corev1.Toleration{
				{
					Effect:            corev1.TaintEffectNoExecute,
					Key:               "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					TolerationSeconds: ptr.To(tolerationSeconds),
				},
				{
					Effect:            corev1.TaintEffectNoExecute,
					Key:               "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					TolerationSeconds: ptr.To(tolerationSeconds),
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host-root",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
							Type: ptr.To(corev1.HostPathType("Directory")), // Ensure the path is a directory
						},
					},
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

func (u *utils) IsSNOCluster() (bool, error) {
	infra := &configv1.Infrastructure{}
	err := u.Get(context.Background(), clientOptions.ObjectKey{Name: "cluster"}, infra)
	if err != nil {
		return false, err
	}

	return infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
}

// GetPlatformType returns the cluster's platform type.
// If it's not AWS, BareMetal, or None, it returns an unsupported platform error.
func (u *utils) GetPlatformType() (configv1.PlatformType, error) {
	infra := &configv1.Infrastructure{}
	err := u.Get(context.Background(), clientOptions.ObjectKey{Name: "cluster"}, infra)
	if err != nil {
		return "", err
	}

	return infra.Status.PlatformStatus.Type, nil
}

func (u *utils) GetPodLogs(namespace string, pod *corev1.Pod) (string, error) {
	log.Print("getting log of pod")
	podLogOptions := &corev1.PodLogOptions{}

	logsRequest := u.ClientSet.Pods(namespace).GetLogs(pod.Name, podLogOptions)
	logStream, err := logsRequest.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get log stream: %w", err)
	}
	defer logStream.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, logStream)

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// IsIPv6Enabled detects whether the cluster networking includes IPv6.
// It checks only the Spec.ClusterNetwork CIDRs for IPv6.
func (u *utils) IsIPv6Enabled() (bool, error) {
	network := &configv1.Network{}
	err := u.Get(context.Background(), clientOptions.ObjectKey{Name: "cluster"}, network)
	if err != nil {
		return false, err
	}

	// Check ClusterNetwork CIDRs
	for _, entry := range network.Spec.ClusterNetwork {
		if strings.Contains(entry.CIDR, ":") {
			return true, nil
		}
	}

	return false, nil
}

package utility

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

func NewHelper(namespace, deviceclass string) Helper {
	return Helper{namespace: namespace, deviceclass: deviceclass}
}

type Helper struct {
	namespace   string
	deviceclass string
}

func (h Helper) NewResourceClaimTemplate(name string) *resourceapi.ResourceClaimTemplate {
	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.namespace,
			Name:      name,
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{
						{
							Name:            "gpu",
							DeviceClassName: h.deviceclass,
						},
					},
				},
			},
		},
	}
}

func (h Helper) NewPod(name string) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   h.namespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To[int64](1),
			SecurityContext:               e2epodutil.GetRestrictedPodSecurityContext(),
		},
	}
	return pod
}

func NewContainer(name string) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           e2epodutil.GetDefaultTestImage(),
		Command:         e2epodutil.GenerateScriptCmd("env && sleep 100000"),
		SecurityContext: e2epodutil.GetRestrictedContainerSecurityContext(),
	}
}

func NewNvidiaSMIContainer(name string) corev1.Container {
	return corev1.Container{
		Name:    name,
		Image:   "ubuntu:22.04",
		Command: []string{"bash", "-c"},
		Args:    []string{"nvidia-smi -L; trap 'exit 0' TERM; sleep 9999 & wait"},
	}
}

func UsePrivilegedSCC(clientset *kubernetes.Clientset, sa, namespace string) error {
	name := fmt.Sprintf("%s-use-scc", sa)
	want := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:privileged",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa,
				Namespace: namespace,
			},
		},
	}
	_, err := clientset.RbacV1().ClusterRoleBindings().Create(context.Background(), &want, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func EnsureNodeLabel(clientset kubernetes.Interface, node string, key, want string) error {
	client := clientset.CoreV1().Nodes()
	current, err := client.Get(context.Background(), node, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if value, ok := current.Labels[key]; ok && value == want {
		return nil
	}

	if len(current.Labels) == 0 {
		current.Labels = map[string]string{}
	}
	current.Labels[key] = want
	_, err = client.Update(context.Background(), current, metav1.UpdateOptions{})
	return err
}

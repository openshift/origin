package helper

import (
	"context"
	"fmt"
	"io"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "k8s.io/kubernetes/test/utils"

	"k8s.io/utils/ptr"
)

func NewPod(namespace, name string) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
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

func UsePrivilegedSCC(ctx context.Context, clientset *kubernetes.Clientset, sa, namespace string) error {
	name := fmt.Sprintf("%s-use-scc-privileged", sa)
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
	_, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, &want, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func EnsureNodeLabel(ctx context.Context, clientset kubernetes.Interface, node string, key, want string) error {
	client := clientset.CoreV1().Nodes()
	current, err := client.Get(ctx, node, metav1.GetOptions{})
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
	_, err = client.Update(ctx, current, metav1.UpdateOptions{})
	return err
}

func GetLogs(ctx context.Context, clientset kubernetes.Interface, namespace, name, ctr string) (string, error) {
	client := clientset.CoreV1().Pods(namespace)
	options := corev1.PodLogOptions{Container: ctr}
	r, err := client.GetLogs(name, &options).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs %s: %w", name, err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs %s: %w", name, err)
	}
	return string(out), nil
}

func GetResourceClaimFor(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod) (*resourceapi.ResourceClaim, error) {
	result, err := clientset.ResourceV1beta1().ResourceClaims(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range result.Items {
		claim := &result.Items[i]
		for _, owner := range claim.OwnerReferences {
			if owner.Name == pod.Name && owner.UID == pod.UID {
				return claim, nil
			}
		}
	}
	return nil, nil
}

func PodRunningReady(ctx context.Context, t testing.TB, clientset kubernetes.Interface, component, namespace string, options metav1.ListOptions) error {
	client := clientset.CoreV1().Pods(namespace)
	result, err := client.List(ctx, options)
	if err != nil || len(result.Items) == 0 {
		return fmt.Errorf("[%s] still waiting for pod to show up - %w", component, err)
	}

	for _, pod := range result.Items {
		ready, err := testutils.PodRunningReady(&pod)
		if err != nil || !ready {
			err := fmt.Errorf("[%s] still waiting for pod: %s to be ready: %v", component, pod.Name, err)
			t.Log(err.Error())
			return err
		}
		t.Logf("[%s] pod: %s ready", component, pod.Name)
	}
	return nil
}

func GetAllocatedDeviceForRequest(request string, claim *resourceapi.ResourceClaim) (string, string, error) {
	// the request must exist in the spec
	found := false
	for _, dr := range claim.Spec.Devices.Requests {
		if dr.Name == request {
			found = true
		}
	}
	if !found {
		return "", "", fmt.Errorf("the request does not exist in the claim")
	}

	allocation := claim.Status.Allocation
	if allocation == nil {
		return "", "", fmt.Errorf("the given claim has not been allocated yet")
	}

	for _, r := range allocation.Devices.Results {
		if r.Request == request {
			return r.Device, r.Pool, nil
		}
	}
	return "", "", nil
}

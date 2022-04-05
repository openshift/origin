package networking

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network] external gateway address", func() {
	oc := exutil.NewCLI("ns-global")

	InOVNKubernetesContext(func() {
		f := oc.KubeFramework()

		g.It("Should validate failure if an external gateway address does not match the address family of the pod", func() {
			err := createPod(f.ClientSet, f.Namespace.Name, "test-valid-gateway-pod")
			expectNoError(err)
			// Set external gateway address into an IPv6 address that does not match the
			// address family of the pod.
			makeNamespaceWithExternalGatewaySet(f, "fd00:10:244:2::6")
			err = createPod(f.ClientSet, f.Namespace.Name, "test-invalid-gateway-pod")
			expectError(err, "pod creation failed due to invalid gateway address")
		})
	})

})

func createPod(client clientset.Interface, ns, generateName string) error {
	pod := frameworkpod.NewAgnhostPod(ns, "", nil, nil, nil)
	pod.ObjectMeta.GenerateName = generateName
	execPod, err := client.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	expectNoError(err, "failed to create new pod in namespace: %s", ns)
	err = wait.PollImmediate(poll, 2*time.Minute, func() (bool, error) {
		retrievedPod, err := client.CoreV1().Pods(execPod.Namespace).Get(context.TODO(), execPod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return retrievedPod.Status.Phase == v1.PodRunning, nil
	})
	return err
}

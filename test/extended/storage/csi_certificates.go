package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe(`[sig-storage][CSI][Jira:"Storage"] CSI driver operator secure certificates`, func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("csi-cert-check")

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Not supported on MicroShift")
		}
	})

	g.It("csi driver operators should use service CA signed certificates by default", func() {
		ctx := context.Background()

		g.By("Verifying the storage cluster operator is healthy")
		WaitForCSOHealthy(oc)

		g.By("Listing CSI driver operator pods")
		allPods, err := oc.AdminKubeClient().CoreV1().Pods(CSINamespace).List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list pods in namespace %s", CSINamespace)

		checked := 0
		var failures []string

		for _, pod := range allPods.Items {
			operatorContainer := getCsiDriverOperatorContainerName(pod)
			if operatorContainer == "" {
				continue
			}

			g.By(fmt.Sprintf("Checking logs of %s for secure certificate usage", pod.Name))
			logStream, err := oc.AdminKubeClient().CoreV1().Pods(CSINamespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: operatorContainer,
			}).Stream(ctx)
			if err != nil {
				e2e.Logf("Failed to get logs for pod %s, skipping: %v", pod.Name, err)
				continue
			}

			logBytes, err := io.ReadAll(logStream)
			logStream.Close()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to read logs for pod %s", pod.Name)

			logOutput := string(logBytes)
			if strings.Contains(logOutput, "Using insecure, self-signed certificates") {
				failures = append(failures, fmt.Sprintf("%s (pod %s): insecure self-signed certificates detected", operatorContainer, pod.Name))
			}
			if !strings.Contains(logOutput, "Using service-serving-cert provided certificates") {
				failures = append(failures, fmt.Sprintf("%s (pod %s): secure cert log not found", operatorContainer, pod.Name))
			}
			checked++
		}

		if checked == 0 {
			g.Skip(fmt.Sprintf("No CSI driver operator pods found on platform %q", e2e.TestContext.Provider))
		}
		o.Expect(failures).To(o.BeEmpty(),
			"CSI driver operators not using service CA signed certificates:\n%s", strings.Join(failures, "\n"))
	})
})

// Returns the name of the CSI driver operator
// container in the pod, or "" if the pod is not a CSI driver operator.
func getCsiDriverOperatorContainerName(pod corev1.Pod) string {
	for _, c := range pod.Spec.Containers {
		if strings.HasSuffix(c.Name, "-csi-driver-operator") {
			return c.Name
		}
	}
	return ""
}

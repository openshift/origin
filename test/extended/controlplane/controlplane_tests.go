package controlplane

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	kubeAPIServerNamespace = "openshift-kube-apiserver"
	insecureReadyzPort     = 6080
	portForwardTimeout     = 30 * time.Second
)

type kubeAPIServerTarget struct {
	podName   string
	namespace string
}

var _ = g.Describe("[sig-api-machinery][Suite:openshift/controlplane] Control Plane", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("controlplane")

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("HTTP readyz endpoint tests are not applicable to MicroShift clusters")
		}

		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("HTTP readyz endpoint tests are not applicable to HyperShift clusters.")
		}
	})

	g.It("[OTP] OCP-24698 should have accessible HTTP readyz endpoints for all kube-apiserver pods", func(ctx context.Context) {
		g.By("discovering kube-apiserver pods")
		pods, err := oc.AdminKubeClient().CoreV1().Pods(kubeAPIServerNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=openshift-kube-apiserver",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list kube-apiserver pods")
		o.Expect(pods.Items).NotTo(o.BeEmpty(), "no kube-apiserver pods found")

		// Filter for running pods with names starting with "kube-apiserver-ip"
		var targets []kubeAPIServerTarget
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning &&
				strings.HasPrefix(pod.Name, "kube-apiserver-ip") {
				targets = append(targets, kubeAPIServerTarget{
					podName:   pod.Name,
					namespace: kubeAPIServerNamespace,
				})
			}
		}
		o.Expect(targets).NotTo(o.BeEmpty(), "no running kube-apiserver-ip pods found")

		e2e.Logf("Found %d kube-apiserver pods to test", len(targets))

		for _, target := range targets {
			target := target // capture loop variable
			g.By(fmt.Sprintf("testing HTTP readyz endpoint for pod %s", target.podName))

			// Use a random local port to avoid conflicts
			localPort := rand.Intn(65534-1025) + 1025

			// Create context with timeout for port-forward
			pfCtx, pfCancel := context.WithCancel(ctx)
			defer pfCancel()

			// Start port-forward in background
			portForwardCmd := exec.CommandContext(pfCtx, "oc", "port-forward",
				"-n", target.namespace,
				target.podName,
				fmt.Sprintf("%d:%d", localPort, insecureReadyzPort),
			)

			e2e.Logf("Starting port-forward: %s", portForwardCmd.String())
			err := portForwardCmd.Start()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to start port-forward for pod %s", target.podName)

			// Ensure cleanup on test completion
			g.DeferCleanup(func() {
				e2e.Logf("Cleaning up port-forward for pod %s", target.podName)
				pfCancel()
				if portForwardCmd.Process != nil {
					_ = portForwardCmd.Process.Kill()
				}
			})

			// Wait for port-forward to be ready
			g.By(fmt.Sprintf("checking HTTP readyz endpoint for pod %s", target.podName))
			var responseBody string
			var lastErr error
			err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, portForwardTimeout, true, func(ctx context.Context) (bool, error) {
				resp, err := http.Get(fmt.Sprintf("http://localhost:%d/readyz?verbose", localPort))
				if err != nil {
					lastErr = err
					return false, nil
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				responseBody = string(body)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "port-forward readiness check failed for pod %s: %v", target.podName, lastErr)
			o.Expect(responseBody).To(o.ContainSubstring("ok"), "readyz endpoint response for pod %s", target.podName)
			e2e.Logf("Pod %s readyz response: %s", target.podName, responseBody)
			e2e.Logf("Successfully verified HTTP readyz endpoint for pod %s", target.podName)
		}
	})
})

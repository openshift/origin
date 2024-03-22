package kubevirt

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-kubevirt] node reboot", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("ns-global", admissionapi.LevelBaseline)

	InKubeVirtClusterContext(oc, func() {
		mgmtFramework := e2e.NewDefaultFramework("mgmt-framework")
		mgmtFramework.SkipNamespaceCreation = true

		hostedFramework := e2e.NewDefaultFramework("hosted-framework")
		hostedFramework.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

		When("an hosted control plane worker is rebooted [Early]", func() {
			var hostedClusterNodeToBeRebooted string

			simplifiedNodeReadyCondition := func(nodeName string) (corev1.NodeCondition, error) {
				node, err := hostedFramework.ClientSet.CoreV1().Nodes().Get(
					context.Background(),
					nodeName,
					metav1.GetOptions{},
				)
				if err != nil {
					return corev1.NodeCondition{}, err
				}

				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady {
						return corev1.NodeCondition{
							Type:   condition.Type,
							Status: condition.Status,
						}, nil
					}
				}
				return corev1.NodeCondition{}, nil
			}

			BeforeEach(func() {
				nodeList, err := hostedFramework.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(nodeList.Items).NotTo(BeEmpty())

				hostedClusterNodeToBeRebooted = nodeList.Items[0].Name

				Eventually(func() (corev1.NodeCondition, error) {
					return simplifiedNodeReadyCondition(hostedClusterNodeToBeRebooted)
				}).
					WithTimeout(2*time.Minute).
					WithPolling(5*time.Second).
					Should(
						Equal(corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue}),
						fmt.Sprintf(
							"node %q should have the `Ready` condition",
							hostedClusterNodeToBeRebooted,
						),
					)

				setMgmtFramework(mgmtFramework)
				expectNoError(
					rebootWorkerNode(
						mgmtFramework,
						hostedClusterNodeToBeRebooted,
					),
				)
				Eventually(func() (corev1.NodeCondition, error) {
					return simplifiedNodeReadyCondition(hostedClusterNodeToBeRebooted)
				}).
					WithTimeout(2*time.Minute).
					WithPolling(5*time.Second).
					Should(
						Equal(corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionFalse}),
						fmt.Sprintf(
							"node %q must reach the `NotReady` condition after reboot",
							hostedClusterNodeToBeRebooted,
						),
					)
			})

			It("should maintain node readiness", func() {
				Eventually(func() (corev1.NodeCondition, error) {
					return simplifiedNodeReadyCondition(hostedClusterNodeToBeRebooted)
				}).
					WithTimeout(5*time.Minute).
					WithPolling(5*time.Second).
					Should(
						Equal(corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue}),
						fmt.Sprintf("node %q should have the `Ready` condition", hostedClusterNodeToBeRebooted),
					)
				Consistently(func() (corev1.NodeCondition, error) {
					return simplifiedNodeReadyCondition(hostedClusterNodeToBeRebooted)
				}).
					WithTimeout(2*time.Minute).
					WithPolling(5*time.Second).
					Should(
						Equal(corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue}),
						fmt.Sprintf("node %q should have the `Ready` condition", hostedClusterNodeToBeRebooted),
					)
			})
		})
	})
})

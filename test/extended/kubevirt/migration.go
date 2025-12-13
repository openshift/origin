package kubevirt

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-kubevirt] migration", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("ns-global", admissionapi.LevelBaseline)
	InKubeVirtClusterContext(oc, func() {
		mgmtFramework := e2e.NewDefaultFramework("mgmt-framework")
		mgmtFramework.SkipNamespaceCreation = true

		hostedFramework := e2e.NewDefaultFramework("hosted-framework")
		hostedFramework.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
		var (
			numberOfReadyNodes = func() (int, error) {
				nodeList, err := hostedFramework.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return 0, err
				}
				numberOfReadyNodes := 0
				for _, node := range nodeList.Items {
					for _, condition := range node.Status.Conditions {
						if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
							numberOfReadyNodes += 1
						}
					}
				}
				return numberOfReadyNodes, nil
			}
		)
		Context("and live migrate hosted control plane workers [Early]", func() {
			var (
				numberOfNodes = 0
			)
			BeforeEach(func() {
				nodeList, err := hostedFramework.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				numberOfNodes = len(nodeList.Items)

				Eventually(numberOfReadyNodes).
					WithTimeout(2*time.Minute).
					WithPolling(5*time.Second).
					Should(Equal(numberOfNodes), "nodes should have ready state before migration")

				SetMgmtFramework(mgmtFramework)
				expectNoError(migrateWorkers(mgmtFramework))
			})
			It("should maintain node readiness", Label("Size:L"), func() {
				By("Check node readiness is as expected")
				isAWS, err := MgmtClusterIsType(mgmtFramework, configv1.AWSPlatformType)
				Expect(err).ToNot(HaveOccurred())

				if isAWS {
					// At aws live-migration tcp connections are broken so Node
					// readiness is broken too, we have wait for it to reach
					// not ready and then check if eventually and consistently it's
					// ready again
					Eventually(numberOfReadyNodes).
						WithTimeout(2*time.Minute).
						WithPolling(5*time.Second).
						ShouldNot(Equal(numberOfNodes), "nodes should reach not ready state")
				}
				Eventually(numberOfReadyNodes).
					WithTimeout(2*time.Minute).
					WithPolling(5*time.Second).
					Should(Equal(numberOfNodes), "nodes should reach ready state")
				Consistently(numberOfReadyNodes).
					WithTimeout(2*time.Minute).
					WithPolling(5*time.Second).
					Should(Equal(numberOfNodes), "nodes should maintain ready state")
			})
		})
	})
})

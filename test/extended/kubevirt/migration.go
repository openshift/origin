package kubevirt

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

		f1 := e2e.NewDefaultFramework("server-framework")
		f1.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
		var (
			numberOfReadyNodes = func() (int, error) {
				nodeList, err := f1.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
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
		AfterLiveMigrateWorkersContext(mgmtFramework, func() {
			It("should maintain node readiness", func() {
				nodeList, err := f1.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				numberOfNodes := len(nodeList.Items)

				By("Check node readiness is as expected")
				isAWS, err := mgmtClusterIsAWS(mgmtFramework)
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

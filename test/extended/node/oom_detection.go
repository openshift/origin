package node

import (
	"context"
	"fmt"
	o "github.com/onsi/gomega"
	"strings"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: expand this test to SSH into every node in the cluster and gather kubelet logs in case kubelet doesn't send
// 		 events to the api-server or `oc adm node-logs`
var _ = g.Describe("[sig-node][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("no-oom-kills")

	g.It("should not have pods that are OOM Killed", func() {
		kubeClient := oc.AdminKubeClient()

		events, err := kubeClient.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}
		oomEventNodeList := []string{}
		suiteStartTime := exutil.SuiteStartTime()
		errorMsg := "System OOM encountered"
		for _, event := range events.Items {
			if event.LastTimestamp.Time.Before(suiteStartTime) {
				continue
			}
			if !strings.Contains(event.Message, errorMsg) {
				continue
			}
			oomEventNodeList = append(oomEventNodeList, event.InvolvedObject.Name)
		}
		failMessage := fmt.Sprintf("%v OOM events occured in the following nodes\n", oomEventNodeList)
		if len(oomEventNodeList) > 0 {
			g.Fail(failMessage)
			return
		}
		// let us see if need to mark this as a flake
		// result.Flakef(strings.Join(oomEncounteredEvents, "\n"))
	})
})

var _ = g.Describe("[sig-node][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("no-oom-kills within worker node journal logs")

	g.It("OOM Killer shouldn't have been invoked in kubelet logs", func() {

		nodeList, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oomJournalNodeList := []string{}
		testDuration := exutil.DurationSinceStartInSeconds()
		for _, node := range nodeList.Items {
			out, err := oc.Run("adm", "node-logs").Args(node.Name, "--path=journal",
				"--case-sensitive=false", "--grep=invoked oom-killer", "--since=-"+string(testDuration)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(strings.ToLower(out), "invoked oom-killer") {
				continue
			}
			oomJournalNodeList = append(oomJournalNodeList, node.Name)
		}
		failMessage := fmt.Sprintf("%v OOM events occured in the following nodes from kubelet log\n",
			oomJournalNodeList)
		if len(oomJournalNodeList) > 0 {
			g.Fail(failMessage)
		}
		// TOOD: Figure out if we need to do a comparison when the events are not stuck on the node if there is a kernel
		//		 bug
	})
})

package security

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

func matchForbiddenNodeLine(nodeName, labelName, outPut string) bool {
	fmt.Printf("Debug: Entering matchForbiddenNodeLine with nodeName=%s, labelName=%s\n", nodeName, labelName)
	regexTemplate := fmt.Sprintf(`Error from server \(Forbidden\): nodes "%s" is forbidden: is not allowed to modify labels: %s`,
		regexp.QuoteMeta(nodeName), regexp.QuoteMeta(strings.Split(labelName, "=")[0]))
	fmt.Printf("Debug: regexTemplate=%s\n", regexTemplate)
	pattern := regexp.MustCompile(regexTemplate)
	fmt.Printf("Debug: Compiled regex pattern\n")
	result := pattern.MatchString(outPut)
	fmt.Printf("Debug: MatchString result=%v\n", result)
	return result
}

// testOpenshiftNodeLabeling attempts to apply the label to the given node by using node's
// kubeconfig present in /var/lib/kubelet/kubeconfig. It will capture the stdout/stderr of that execution
// and process it to verify that the forbidden labels aren't getting applied.
func testOpenshiftNodeLabeling(oc *exutil.CLI, node *corev1.Node, forbiddenLabel string) bool {
	fmt.Printf("Debug: Entering testOpenshiftNodeLabeling with node=%s, forbiddenLabel=%s\n", node.Name, forbiddenLabel)
	out, err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c",
		fmt.Sprintf(`KUBECONFIG=/var/lib/kubelet/kubeconfig kubectl label nodes/%s %s `, node.Name, forbiddenLabel)).Output()
	fmt.Printf("Debug: Executed oc debug command\n")

	if err == nil {
		// If for some reason the 'kubectl' command succeeded (instead of failing), printing the output
		// would help in debugging
		fmt.Printf("Debug: oc debug command succeeded unexpectedly\n")
		fmt.Printf("Output of the oc debug command, %s", out)
	} else {
		fmt.Printf("Debug: oc debug command failed as expected. Error: %v\n", err)
	}

	o.Expect(err).NotTo(o.BeNil())
	fmt.Printf("Debug: Assertion passed: error is not nil\n")
	result := matchForbiddenNodeLine(node.Name, forbiddenLabel, out)
	fmt.Printf("Debug: matchForbiddenNodeLine result=%v\n", result)
	return result
}

var _ = g.Describe("[sig-node] [Conformance] Prevent openshift node labeling on update by the node", func() {
	fmt.Println("Debug: Entering Describe block")
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("node-label-e2e", admissionapi.LevelPrivileged)
	fmt.Println("Debug: Created CLI with pod security level")

	g.It("TestOpenshiftNodeLabeling", func() {
		fmt.Println("Debug: Entering It block")
		clusterAdminKubeClientset := oc.AdminKubeClient()
		fmt.Println("Debug: Got admin kube client")

		workerNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		fmt.Printf("Debug: Listed worker nodes. Error: %v, Number of nodes: %d\n", err, len(workerNodes.Items))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes.Items)).NotTo(o.BeZero())
		fmt.Println("Debug: Assertions passed for worker nodes")

		if len(workerNodes.Items) > 0 {
			workerNode := &workerNodes.Items[0]
			fmt.Printf("Debug: Selected worker node: %s\n", workerNode.Name)
			forbiddenLabels := []string{`node-role.kubernetes.io/etcd1=""`, `node-role.kubernetes.io/etcd=""`}
			for _, forbiddenLabel := range forbiddenLabels {
				fmt.Printf("Debug: Testing forbidden label: %s\n", forbiddenLabel)
				result := testOpenshiftNodeLabeling(oc, workerNode, forbiddenLabel)
				fmt.Printf("Debug: testOpenshiftNodeLabeling result=%v\n", result)
				o.Expect(result).To(o.BeTrue())
				fmt.Println("Debug: Assertion passed: result is true")
			}
		} else {
			fmt.Println("Debug: No worker nodes found")
		}
	})
})

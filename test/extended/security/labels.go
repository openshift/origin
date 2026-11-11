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
	regexTemplate := fmt.Sprintf(`Error from server \(Forbidden\): nodes "%s" is forbidden: is not allowed to modify labels: %s`,
		regexp.QuoteMeta(nodeName), regexp.QuoteMeta(strings.Split(labelName, "=")[0]))
	pattern := regexp.MustCompile(regexTemplate)
	return pattern.MatchString(outPut)
}

// exportNodeProxyEnv exports the node's proxy settings, which only systemd units load from /etc/mco/proxy.env,
// so node-side kubectl can reach an API server that is only reachable through the proxy. The file is parsed as
// data rather than sourced so values are kept literally (e.g. '$' in a password) and nothing in it is executed.
const exportNodeProxyEnv = `if [ -f /etc/mco/proxy.env ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    [[ "$line" == *=* ]] || continue
    key="${line%%=*}"
    case "$key" in
      HTTP_PROXY|HTTPS_PROXY|NO_PROXY|http_proxy|https_proxy|no_proxy) export "$key=${line#*=}" ;;
    esac
  done < /etc/mco/proxy.env
fi
`

// testOpenshiftNodeLabeling attempts to apply the label to the given node by using node's
// kubeconfig present in /var/lib/kubelet/kubeconfig. It will capture the stdout/stderr of that execution
// and process it to verify that the forbidden labels aren't getting applied.
func testOpenshiftNodeLabeling(oc *exutil.CLI, node *corev1.Node, forbiddenLabel string) bool {
	out, err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c",
		exportNodeProxyEnv+fmt.Sprintf(`KUBECONFIG=/var/lib/kubelet/kubeconfig kubectl label nodes/%s %s `, node.Name, forbiddenLabel)).Output()

	if err == nil {
		// If for some reason the 'kubectl' command succeeded (instead of failing), printing the output
		// would help in debugging
		fmt.Printf("Output of the oc debug command, %s", out)
	}

	o.Expect(err).NotTo(o.BeNil())
	matched := matchForbiddenNodeLine(node.Name, forbiddenLabel, out)
	if !matched {
		fmt.Printf("Expected a Forbidden error for label %s, got: %s", forbiddenLabel, out)
	}
	return matched
}

var _ = g.Describe("[sig-node] [Conformance] Prevent openshift node labeling on update by the node", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("node-label-e2e", admissionapi.LevelPrivileged)

	g.It("TestOpenshiftNodeLabeling", func() {
		clusterAdminKubeClientset := oc.AdminKubeClient()

		workerNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes.Items)).NotTo(o.BeZero())
		if len(workerNodes.Items) > 0 {
			workerNode := &workerNodes.Items[0]
			forbiddenLabels := []string{`node-role.kubernetes.io/etcd1=""`, `node-role.kubernetes.io/etcd=""`}
			for _, forbiddenLabel := range forbiddenLabels {
				result := testOpenshiftNodeLabeling(oc, workerNode, forbiddenLabel)
				o.Expect(result).To(o.BeTrue())
			}
		}
	})
})

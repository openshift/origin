package node

import (
	"os/exec"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Feature:WorkerLatencyProfiles]", func() {
	g.It("should reject extreme latency profile", func() {
		latencyProfile := workerLatencyProfile()
		o.Expect(latencyProfile).To(o.Equal(""))
	})
})

func workerLatencyProfile() string {
	// We don't use exutil.NewCLI() here because it can't be called from BeforeEach()
	out, err := exec.Command(
		"oc", "--kubeconfig="+exutil.KubeConfigPath(),
		"get", "nodes.config.openshift.io", "cluster",
		"--template={{.spec.workerLatencyProfile}}",
	).CombinedOutput()
	latencyProfile := string(out)
	if err != nil {
		e2e.Logf("Could not check network plugin name: %v. Assuming a non-OpenShift plugin", err)
		latencyProfile = ""
	}
	return latencyProfile
}

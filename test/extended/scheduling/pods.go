package scheduling

import (
	"context"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/single_node"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// requirePodsOnDifferentNodesTest fails if any pods with the prefix in the namespace are scheduled to the same node
type requirePodsOnDifferentNodesTest struct {
	namespace string
	podPrefix string
}

func (p requirePodsOnDifferentNodesTest) run(oc *exutil.CLI) {
	pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods(p.namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Failf("unable to list pods: %v", err)
	}
	nodeNameMap := map[string]string{}
	for _, pod := range pods.Items {
		if !strings.Contains(pod.Name, p.podPrefix) {
			continue
		}
		if podName, ok := nodeNameMap[pod.Spec.NodeName]; ok {
			e2e.Failf("ns/%s pod %s and pod %s are running on the same node: %s", p.namespace, pod.Name, podName, pod.Spec.NodeName)
		} else {
			nodeNameMap[pod.Spec.NodeName] = pod.Name
		}
	}
}

var _ = g.Describe("[sig-scheduling][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("scheduling-pod-check")

	// TODO: Classify particular deployments according to whether they apply to
	// infra vs control-plane topologies, because this test may still be
	// interesting in clusters where contorl-plane topology is single-node but
	// infra topology is HighlyAvailable. However, such clusters are not
	// currently officially supported so this is not too urgent. on HA infra
	// SingleReplica control plane e.g. HAProxy router pods need to be
	// verified, but not openshift-apiserver.
	controlPlaneTopology, _ := single_node.GetTopologies(oc.KubeFramework())
	if controlPlaneTopology == configv1.SingleReplicaTopologyMode {
		e2eskipper.Skipf("Test is not relevant for single replica control-plane topologies")
	}

	g.BeforeEach(func() {
		var err error
		_, err = exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-ingress", podPrefix: "router-default"}.run(oc)
		})
	})

	g.Describe("The openshift-apiserver pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-apiserver", podPrefix: "apiserver"}.run(oc)
		})
	})

	g.Describe("The openshift-authentication pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-authentication", podPrefix: "oauth-openshift"}.run(oc)
		})
	})

	g.Describe("The openshift-console pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-console", podPrefix: "console"}.run(oc)
		})
	})

	g.Describe("The openshift-console pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-console", podPrefix: "downloads"}.run(oc)
		})
	})

	g.Describe("The openshift-etcd pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-etcd", podPrefix: "etcd-quorum-guard"}.run(oc)
		})
	})

	g.Describe("The openshift-image-registry pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-image-registry", podPrefix: "image-registry"}.run(oc)
		})
	})

	g.Describe("The openshift-monitoring pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-monitoring", podPrefix: "prometheus-adapter"}.run(oc)
		})
	})

	g.Describe("The openshift-monitoring pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-monitoring", podPrefix: "thanos-querier"}.run(oc)
		})
	})

	g.Describe("The openshift-oauth-apiserver pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-oauth-apiserver", podPrefix: "apiserver"}.run(oc)
		})
	})

	g.Describe("The openshift-operator-lifecycle-manager pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-operator-lifecycle-manager", podPrefix: "packageserver"}.run(oc)
		})
	})
})

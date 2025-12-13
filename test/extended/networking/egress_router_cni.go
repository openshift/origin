package networking

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	egressRouterCNIE2E        = "egress-router-cni-e2e"
	egressRouterCNIDeployment = "egress-router-cni-deployment"
	egressRouterCNINadObject  = "egress-router-cni-nad"
	// Manifests at testdata/egress-router-cni
	egressRouterCNIV4Manifest = "egress-router-cni-v4-cr.yaml"
	egressRouterCNIV6Manifest = "egress-router-cni-v6-cr.yaml"
	egressRouterCNILogs       = "/tmp/egress-router-log"
	// Match patterns are based on what is in testdata/egress-router-cni manifests
	ipv4MatchPattern = "IP Destinations: [80 UDP 10.100.3.0 8080 SCTP 203.0.113.26 80 8443 TCP 203.0.113.27 443]"
	ipv6MatchPattern = "IP Destinations: [80 UDP 10:100:3::0 8080 SCTP 203:0:113::26 80 8443 TCP 203:0:113::27 443]"
	timeOut          = 1 * time.Minute
	interval         = 5 * time.Second
)

var _ = g.Describe("[sig-network][Feature:EgressRouterCNI]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel(egressRouterCNIE2E, admissionapi.LevelPrivileged).SetManagedNamespace()

	g.It("should ensure ipv4 egressrouter cni resources are created [apigroup:operator.openshift.io]", g.Label("Size:M"), func() {
		doEgressRouterCNI(egressRouterCNIV4Manifest, oc, ipv4MatchPattern)
	})
	InOVNKubernetesContext(
		func() {
			g.It("should ensure ipv6 egressrouter cni resources are created [apigroup:operator.openshift.io]", g.Label("Size:M"), func() {
				doEgressRouterCNI(egressRouterCNIV6Manifest, oc, ipv6MatchPattern)
			})
		},
	)
})

func doEgressRouterCNI(manifest string, oc *exutil.CLI, matchString string) error {
	var deployment *appsv1.Deployment
	var podList *v1.PodList
	f := oc.KubeFramework()

	g.By("creating an egressroutercni object")
	egressRouterCNIYaml := exutil.FixturePath("testdata", "egress-router-cni", manifest)

	g.By(fmt.Sprintf("calling oc create -f %s", egressRouterCNIYaml))
	err := oc.AsAdmin().Run("create").Args("-f", egressRouterCNIYaml).Execute()
	e2e.ExpectNoError(err, "while creating egress-router-cni object")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("making sure egressroutercni deployment is created")
	o.Eventually(func() error {
		deployment, err = f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Get(context.Background(), egressRouterCNIDeployment, metav1.GetOptions{})
		return err
	}, timeOut, interval).Should(o.Succeed())

	g.By("checking network-attachment-definition resource is created")
	output, err := oc.AsAdmin().Run("get").Args("network-attachment-definition", "-o=jsonpath={.items[0].metadata.name}").Output()
	e2e.ExpectNoError(err, "while creating egress-router-cni network-attachement-definition")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).Should(o.ContainSubstring(egressRouterCNINadObject))

	g.By("getting a pod from deployment in running state")
	o.Eventually(func() error {
		podList, err = e2edeployment.GetPodsForDeployment(context.TODO(), f.ClientSet, deployment)
		return err
	}, timeOut, interval).Should(o.Succeed())

	o.Expect(podList.Items).NotTo(o.BeEmpty())
	pod := podList.Items[0]
	e2e.Logf("egress router cni pod %s is created\n", pod.Spec.Containers[0].Name)

	node := pod.Spec.NodeName
	expectNoError(err)
	g.By("checking for specific output in the egress-router log on the node " + node)
	o.Eventually(func() (string, error) {
		result, err := oc.AsAdmin().Run("debug").Args("node/"+node, "--", "chroot", "/host", "/bin/bash", "-c", "cat /tmp/egress-router-log").Output()
		return result, err
	}, timeOut, interval).Should(o.ContainSubstring(matchString))

	return nil
}

package single_node

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/apiserverauth"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func getOpenshiftNamespaces(f *framework.Framework) []corev1.Namespace {
	list, err := f.ClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	var openshiftNamespaces []corev1.Namespace
	for _, namespace := range list.Items {
		if strings.HasPrefix(namespace.Name, "openshift-") {
			openshiftNamespaces = append(openshiftNamespaces, namespace)
		}
	}

	return openshiftNamespaces
}

func getNamespaceDeployments(f *framework.Framework, namespace corev1.Namespace) []appsv1.Deployment {
	list, err := f.ClientSet.AppsV1().Deployments(namespace.Name).List(context.Background(), metav1.ListOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return list.Items
}

func getNamespaceStatefulSets(f *framework.Framework, namespace corev1.Namespace) []appsv1.StatefulSet {
	list, err := f.ClientSet.AppsV1().StatefulSets(namespace.Name).List(context.Background(), metav1.ListOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return list.Items
}

func GetTopologies(f *framework.Framework) (controlPlaneTopology, infraTopology v1.TopologyMode) {
	oc := exutil.NewCLIWithFramework(f)
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology
}

// isInfrastructureStatefulSet decides if a StatefulSet is considered "infrastructure" or
// "control plane" by comparing it against a known list
func isInfrastructureStatefulSet(statefulSet appsv1.StatefulSet) bool {
	infrastructureNamespaces := map[string][]string{
		"openshift-monitoring": {
			"alertmanager-main",
			"prometheus-k8s",
		},
	}

	namespaceInfraStatefulSets, _ := infrastructureNamespaces[statefulSet.Namespace]

	for _, infraStatefulSetName := range namespaceInfraStatefulSets {
		if statefulSet.Name == infraStatefulSetName {
			return true
		}
	}

	return false
}

// isInfrastructureDeployment decides if a deployment is considered "infrastructure" or
// "control plane" by comparing it against a known list
func isInfrastructureDeployment(deployment appsv1.Deployment) bool {
	infrastructureNamespaces := map[string][]string{
		"openshift-ingress": {
			"router-default",
		},
	}

	namespaceInfraDeployments, _ := infrastructureNamespaces[deployment.Namespace]

	for _, infraDeploymentName := range namespaceInfraDeployments {
		if deployment.Name == infraDeploymentName {
			return true
		}
	}

	return false
}

func validateReplicas(name, namespace string, replicas int) {
	gomega.Expect(replicas).To(gomega.Equal(1),
		"%s in %s namespace expected to have 1 replica but got %d", name, namespace, replicas)
}

func validateStatefulSetReplicas(statefulSet appsv1.StatefulSet, controlPlaneTopology,
	infraTopology v1.TopologyMode) {
	if isInfrastructureStatefulSet(statefulSet) {
		if infraTopology != v1.SingleReplicaTopologyMode {
			return
		}
	} else if controlPlaneTopology != v1.SingleReplicaTopologyMode {
		return
	}

	gomega.Expect(statefulSet.Spec.Replicas).ToNot(gomega.BeNil())

	validateReplicas(statefulSet.Name, statefulSet.Namespace, int(*statefulSet.Spec.Replicas))
}

func validateDeploymentReplicas(deployment appsv1.Deployment,
	controlPlaneTopology, infraTopology v1.TopologyMode) {
	if isInfrastructureDeployment(deployment) {
		if infraTopology != v1.SingleReplicaTopologyMode {
			return
		}
	} else if controlPlaneTopology != v1.SingleReplicaTopologyMode {
		return
	}

	gomega.Expect(deployment.Spec.Replicas).ToNot(gomega.BeNil())

	validateReplicas(deployment.Name, deployment.Namespace, int(*deployment.Spec.Replicas))
}

var _ = ginkgo.Describe("[sig-arch] Cluster topology single node tests", func() {
	f := framework.NewDefaultFramework("single-node")

	ginkgo.It("Verify that OpenShift components deploy one replica in SingleReplica topology mode", func() {
		controlPlaneTopology, infraTopology := GetTopologies(f)

		if controlPlaneTopology != v1.SingleReplicaTopologyMode && infraTopology != v1.SingleReplicaTopologyMode {
			e2eskipper.Skipf("Test is only relevant for single replica topologies")
		}

		for _, namespace := range getOpenshiftNamespaces(f) {
			for _, deployment := range getNamespaceDeployments(f, namespace) {
				validateDeploymentReplicas(deployment, controlPlaneTopology, infraTopology)
			}

			for _, statefulSet := range getNamespaceStatefulSets(f, namespace) {
				validateStatefulSetReplicas(statefulSet, controlPlaneTopology, infraTopology)
			}
		}
	})
})

var _ = ginkgo.Describe("[sig-arch] Single node upgrade tests", func() {
	defer ginkgo.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("sno-upgrade")

	ginkgo.It("[OTP][OCP-44597] should verify kube-apiserver startup-monitor mechanism on SNO upgrade [PstChkUpgrade][apigroup:config.openshift.io][apigroup:operator.openshift.io]", ote.Informing(), func() {
		ginkgo.By("1) Check if cluster is SNO")
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		if *controlPlaneTopology != v1.SingleReplicaTopologyMode {
			ginkgo.Skip("This is not a SNO cluster, skip.")
		}

		kubeApiserverCoStatus := map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}

		ginkgo.By("2) Get a master node")
		masterNode, getFirstMasterNodeErr := compat_otp.GetFirstMasterNode(oc)
		gomega.Expect(getFirstMasterNodeErr).NotTo(gomega.HaveOccurred())
		gomega.Expect(masterNode).NotTo(gomega.Equal(""))

		ginkgo.By("3) Check the kube-apiserver-last-known-good link file exists and is linked to a good version")
		cmd := "ls -l /etc/kubernetes/static-pod-resources/kube-apiserver-last-known-good"
		output, debugNodeWithChrootErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
		gomega.Expect(debugNodeWithChrootErr).NotTo(gomega.HaveOccurred())

		ginkgo.By("3.1) Check kube-apiserver-last-known-good file exists")
		gomega.Expect(output).Should(gomega.ContainSubstring("kube-apiserver-last-known-good"))
		ginkgo.By("3.2) Check file is linked to another file")
		gomega.Expect(output).Should(gomega.ContainSubstring("->"))
		ginkgo.By("3.3) Check linked file exists")
		gomega.Expect(output).Should(gomega.ContainSubstring("kube-apiserver-pod.yaml"))
		re := regexp.MustCompile(`kube-apiserver-pod-(\d+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) <= 1 {
			e2e.Failf("No match last-known-good config file for kube-apiserver found!")
		}
		lastGoodRevision, _ := strconv.Atoi(matches[1])
		gomega.Expect(lastGoodRevision).To(gomega.BeNumerically(">", 0))

		ginkgo.By("4) Check cluster operator kube-apiserver is normal, not degraded, and does not contain abnormal statuses")
		apiserverauth.CheckCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)

		ginkgo.By("5) Check the currentRevision kube-apiserver should be the same with last-known-good pointing")
		kasOperatorCurrentRevision, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeapiserver.operator", "cluster", "-o", "jsonpath={.status.nodeStatuses[0].currentRevision}").Output()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		n, _ := strconv.Atoi(kasOperatorCurrentRevision)
		gomega.Expect(n).To(gomega.Equal(lastGoodRevision))
	})
})

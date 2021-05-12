package single_node

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func getOpenshiftNamespaces(f *e2e.Framework) []corev1.Namespace {
	list, err := f.ClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	var openshiftNamespaces []corev1.Namespace
	for _, namespace := range list.Items {
		if strings.HasPrefix(namespace.Name, "openshift-") {
			openshiftNamespaces = append(openshiftNamespaces, namespace)
		}
	}

	return openshiftNamespaces
}

func getNamespaceDeployments(f *e2e.Framework, namespace corev1.Namespace) []appsv1.Deployment {
	list, err := f.ClientSet.AppsV1().Deployments(namespace.Name).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	return list.Items
}

func getTopologies(f *e2e.Framework) (controlPlaneTopology, infraTopology v1.TopologyMode) {
	oc := exutil.NewCLIWithFramework(f)
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology
}

func isInfrastructureDeployment(deployment appsv1.Deployment) bool {
	infrastructureNamespaces := map[string][]string{
		"openshift-ingress": {
			"router-default",
		},
	}

	namespaceInfraDeployments, ok := infrastructureNamespaces[deployment.Namespace]

	if !ok {
		return false
	}

	for _, infraDeploymentName := range namespaceInfraDeployments {
		if deployment.Name == infraDeploymentName {
			return true
		}
	}

	return false
}

func validateReplicas(deployment appsv1.Deployment,
	controlPlaneTopology, infraTopology v1.TopologyMode, failureAllowed bool) {
	if isInfrastructureDeployment(deployment) {
		if infraTopology != v1.SingleReplicaTopologyMode {
			return
		}
	} else if controlPlaneTopology != v1.SingleReplicaTopologyMode {
		return
	}

	Expect(deployment.Spec.Replicas).ToNot(BeNil())

	replicas := int(*deployment.Spec.Replicas)

	if !failureAllowed {
		Expect(replicas).To(Equal(1),
			"%s in %s namespace has wrong number of replicas", deployment.Name, deployment.Namespace)
	} else {
		if replicas == 1 {
			t := GinkgoT()
			t.Logf("Deployment %s in namespace %s has one replica, consider taking it off the topology allow-list",
				deployment.Name, deployment.Namespace)
		}
	}
}

func isAllowedToFail(deployment appsv1.Deployment) bool {
	// allowedToFail is a list of deployments that currently have 2 replicas even in single-replica
	// topology deployments, because their operator has yet to be made aware of the new API.
	// We will slowly remove deployments from this list once their operators have been made
	// aware until this list is empty and this function will be removed.
	allowedToFail := map[string][]string{
		"openshift-operator-lifecycle-manager": {
			"packageserver",
		},
	}

	namespaceAllowedToFailDeployments, ok := allowedToFail[deployment.Namespace]

	if !ok {
		return false
	}

	for _, allowedToFailDeploymentName := range namespaceAllowedToFailDeployments {
		if deployment.Name == allowedToFailDeploymentName {
			return true
		}
	}

	return false
}

var _ = Describe("[sig-arch] Cluster topology single node tests", func() {
	f := e2e.NewDefaultFramework("single-node")

	It("Verify that OpenShift components deploy one replica in SingleReplica topology mode", func() {
		controlPlaneTopology, infraTopology := getTopologies(f)

		if controlPlaneTopology != v1.SingleReplicaTopologyMode && infraTopology != v1.SingleReplicaTopologyMode {
			e2eskipper.Skipf("Test is only relevant for single replica topologies")
		}

		for _, namespace := range getOpenshiftNamespaces(f) {
			for _, deployment := range getNamespaceDeployments(f, namespace) {
				validateReplicas(deployment, controlPlaneTopology, infraTopology, isAllowedToFail(deployment))
			}
		}
	})
})

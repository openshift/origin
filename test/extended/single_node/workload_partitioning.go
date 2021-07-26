package single_node

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func skipIfNotSingleNode(oc *exutil.CLI) {
	g.By("checking topology")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	controlPlaneTopology := infra.Status.ControlPlaneTopology
	infraTopology := infra.Status.InfrastructureTopology

	g.By(fmt.Sprintf("controlPlaneTopology: `%s`, infraTopology: `%s`", controlPlaneTopology, infraTopology))

	if controlPlaneTopology != v1.SingleReplicaTopologyMode && infraTopology != v1.SingleReplicaTopologyMode {
		e2eskipper.Skipf("test is only relevant for single replica topologies")
	}
}

var _ = g.Describe("[sig-arch] workload partitioning", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("single-node")

	namespacesNotYetUpdated := sets.NewString(
		"openshift-config-managed", // this namespace runs no pods, so it will never be updated
		"openshift-config",         // this namespace runs no pods, so it will never be updated
	)

	g.It("should be annotated with: workload.openshift.io/management: {effect: PreferredDuringScheduling}", func() {
		ctx := context.TODO()

		skipIfNotSingleNode(oc)

		kubeClient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		pods, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		annotationForPreferringManagementCores := "workload.openshift.io/management"
		annotationValueForPreferringManagementCores := "{\"effect\": \"PreferredDuringScheduling\"}"
		unexpectedPodsWithAnnotation := sets.NewString()
		corePodsAnnotatedProperly := sets.NewString()
		corePodsMissingAnnotation := sets.NewString()
		for _, pod := range pods.Items {
			nsPodName := pod.Namespace + "/" + pod.Name
			if !strings.HasPrefix(pod.Namespace, "openshift-") {
				if val := pod.Annotations[annotationForPreferringManagementCores]; val == annotationValueForPreferringManagementCores {
					unexpectedPodsWithAnnotation.Insert(nsPodName)
				}
				continue
			}
			targetsManagement := strings.Contains(pod.Annotations[annotationForPreferringManagementCores], annotationValueForPreferringManagementCores)
			if targetsManagement {
				corePodsAnnotatedProperly.Insert(nsPodName)
			} else {
				corePodsMissingAnnotation.Insert(nsPodName)
			}
		}

		if len(corePodsMissingAnnotation) != 0 {
			g.Fail(fmt.Sprintf("pods that are missing annotation: %v", corePodsMissingAnnotation.List()))
		}

		if len(unexpectedPodsWithAnnotation) != 0 {
			g.Fail(fmt.Sprintf("pods that unexpectedly have annotation: %v", unexpectedPodsWithAnnotation.List()))
		}
	})

	g.It("should be annotated with: workload.openshift.io/allowed: management", func() {
		ctx := context.TODO()

		skipIfNotSingleNode(oc)

		kubeClient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		namespaces, err := kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		annotationForAllowingManagementCores := "workload.openshift.io/allowed"
		annotationValueForAllowingManagementCores := "management"
		unexpectedNamespacesAllowingManagementCores := sets.NewString()
		coreNamespacesAnnotatedProperly := sets.NewString()
		coreNamespacesWithoutAnnotation := sets.NewString()
		for _, ns := range namespaces.Items {
			if !strings.HasPrefix(ns.Name, "openshift-") {
				if val := ns.Annotations[annotationForAllowingManagementCores]; strings.Contains(val, annotationValueForAllowingManagementCores) {
					unexpectedNamespacesAllowingManagementCores.Insert(ns.Name)
				}
				continue
			}
			allowsManagement := strings.Contains(ns.Annotations[annotationForAllowingManagementCores], annotationValueForAllowingManagementCores)
			if allowsManagement {
				coreNamespacesAnnotatedProperly.Insert(ns.Name)
			} else {
				if !namespacesNotYetUpdated.Has(ns.Name) {
					coreNamespacesWithoutAnnotation.Insert(ns.Name)
				}
			}
		}

		if len(coreNamespacesWithoutAnnotation) != 0 {
			g.Fail(fmt.Sprintf("namespaces that are missing annotation: %v", coreNamespacesWithoutAnnotation.List()))
		}

		if len(unexpectedNamespacesAllowingManagementCores) != 0 {
			g.Fail(fmt.Sprintf("namespaces that unexpectedly have annotation: %v", unexpectedNamespacesAllowingManagementCores.List()))
		}
	})
})

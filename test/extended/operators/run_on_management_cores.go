package operators

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-arch] OCP Payload", func() {
	defer g.GinkgoRecover()

	namespacesNotYetUpdated := sets.NewString(
		"openshift-config-managed", // this namespace runs no pods, so it will never be updated
		"openshift-config",         // this namespace runs no pods, so it will never be updated
	)
	podPrefixesNotYetUpdated := podPrefixChecker{
		podPrefixes: []string{
			"namespace.name/pod-prefix-like-from-a-deployment-",
		},
	}

	g.It("should run pods on management cores", func() {
		ctx := context.TODO()

		kubeClient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		pods, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		annotationForTargetingManagementCores := "openshift.io/target-core-set"
		// this should only be a single coreset I imagine
		annotationValueForTargetingManagementCores := "management"
		unexpectedPodsTargetingManagementCores := sets.NewString()
		corePodsTargetingManagement := sets.NewString()
		corePodsThatLostTheirTarget := sets.NewString()
		for _, pod := range pods.Items {
			nsPodName := pod.Namespace + "/" + pod.Name
			if !strings.HasPrefix(pod.Namespace, "openshift-") {
				if val := pod.Annotations[annotationForTargetingManagementCores]; val == annotationValueForTargetingManagementCores {
					unexpectedPodsTargetingManagementCores.Insert(nsPodName)
				}
				continue
			}
			targetsManagement := strings.Contains(pod.Annotations[annotationForTargetingManagementCores], annotationValueForTargetingManagementCores)
			if targetsManagement {
				corePodsTargetingManagement.Insert(nsPodName)
			} else {
				if !podPrefixesNotYetUpdated.hasPrefix(nsPodName) {
					corePodsThatLostTheirTarget.Insert(nsPodName)
				}
			}
		}

		// allow some new pods to arrive, but cap the max at a smallish value so that we don't miss our ratchet opportunity
		corePodsToRemoveFromWhitelist := sets.NewString()
		for _, targetingPod := range corePodsTargetingManagement.List() {
			if podPrefixesNotYetUpdated.hasPrefix(targetingPod) {
				corePodsToRemoveFromWhitelist.Insert(targetingPod)
			}
		}
		if len(corePodsToRemoveFromWhitelist) > 5 {
			g.Fail(fmt.Sprintf("pods need to be removed from the whitelist: %v", corePodsToRemoveFromWhitelist.List()))
		}

		if len(corePodsThatLostTheirTarget) != 0 {
			g.Fail(fmt.Sprintf("pods that used to allow management targeting have lost it: %v", corePodsThatLostTheirTarget.List()))
		}

		if len(unexpectedPodsTargetingManagementCores) != 0 {
			g.Fail(fmt.Sprintf("pods that should not target management cores but are: %v", unexpectedPodsTargetingManagementCores.List()))
		}
	})

	g.It("should allow targeting management cores", func() {
		ctx := context.TODO()

		kubeClient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		namespaces, err := kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		annotationForTargetingManagementCores := "openshift.io/target-core-sets-allowed"
		// you probably make this a comma delimited list so you can have management,infra,awesome
		annotationValueForTargetingManagementCores := "management"
		unexpectedNamespacesTargetingManagementCores := sets.NewString()
		coreNamespacesTargetingManagement := sets.NewString()
		coreNamespacesThatLostTheirTarget := sets.NewString()
		for _, ns := range namespaces.Items {
			if !strings.HasPrefix(ns.Name, "openshift-") {
				if val := ns.Annotations[annotationForTargetingManagementCores]; strings.Contains(val, annotationValueForTargetingManagementCores) {
					unexpectedNamespacesTargetingManagementCores.Insert(ns.Name)
				}
				continue
			}
			allowsManagement := strings.Contains(ns.Annotations[annotationForTargetingManagementCores], annotationValueForTargetingManagementCores)
			if allowsManagement {
				coreNamespacesTargetingManagement.Insert(ns.Name)
			} else {
				if !namespacesNotYetUpdated.Has(ns.Name) {
					coreNamespacesThatLostTheirTarget.Insert(ns.Name)
				}
			}
		}

		// allow some new namespaces to arrive, but cap the max at a smallish value so that we don't miss our ratchet opportunity
		coreNamespacesToRemoveFromWhitelist := coreNamespacesTargetingManagement.Difference(namespacesNotYetUpdated)
		if len(coreNamespacesToRemoveFromWhitelist) > 5 {
			g.Fail(fmt.Sprintf("namespaces need to be removed from the whitelist: %v", coreNamespacesToRemoveFromWhitelist.List()))
		}

		if len(coreNamespacesThatLostTheirTarget) != 0 {
			g.Fail(fmt.Sprintf("namespaces that used to allow management targeting have lost it: %v", coreNamespacesThatLostTheirTarget.List()))
		}

		if len(unexpectedNamespacesTargetingManagementCores) != 0 {
			g.Fail(fmt.Sprintf("namespaces that should not target management cores but are: %v", unexpectedNamespacesTargetingManagementCores.List()))
		}
	})
})

type podPrefixChecker struct {
	podPrefixes []string
}

func (c podPrefixChecker) hasPrefix(podName string) bool {
	for _, prefix := range c.podPrefixes {
		if strings.HasPrefix(podName, prefix) {
			return true
		}
	}
	return false
}

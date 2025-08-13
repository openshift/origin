package operators

import (
	"context"
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operator-daemonsets")

	// Daemonsets shipped with the platform must be able to upgrade without disruption to workloads.
	// Daemonsets that are in the data path must gracefully shutdown and redirect workload traffic, or
	// mitigate outage by holding requests briefly (very briefly!). Daemonsets that are part of control
	// plane (anything that changes state) must already be able to be upgraded without disruption by
	// having any components that call them retry when unavailable up to a specific duration. Therefore
	// all workloads must individually be graceful, and so a core daemonset can upgrade up to 10% or 33%
	// of pods at a time. In the future, we may allow this percentage to be tuned via a global config,
	// and this test would enforce that.
	//
	// Use 33% maxUnavailable if you are a workload that has no impact on other workload. This ensures
	// that if there is a bug in the newly rolled out workload, 2/3 of instances remain working.
	// Workloads in this category include the spot instance termination signal observer which listens
	// for when the cloud signals a node that it will be shutdown in 30s. At worst, only 1/3 of machines
	// would be impacted by a bug and at best the new code would roll out that much faster in very large
	// spot instance machine sets.
	//
	// Use 10% maxUnavailable or maxSurge in all other cases, most especially if you have ANY impact on
	// user workloads. This limits the additional load placed on the cluster to a more reasonable degree
	// during an upgrade as new pods start and then establish connections.
	//
	// Currently only applies to daemonsets that don't explicitly target the control plane.
	g.It("should only include cluster daemonsets that have maxUnavailable or maxSurge update of 10 percent or maxUnavailable of 33 percent", func() {
		// iterate over the references to find valid images
		daemonSets, err := oc.KubeFramework().ClientSet.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list daemonsets: %v", err)
		}

		masterReq, err := labels.NewRequirement("node-role.kubernetes.io/master", selection.Exists, []string{})
		if err != nil {
			e2e.Failf("unable to build label requirement: %v", err)
		}
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// These shouldn't be DaemonSets as they end up being scheduled to all nodes because contrary to selfhosted OCP
		// there are no master nodes. Today it is unclear how networking will look like in the future so it isn't worth
		// chaning yet. Future work and removal of these exceptions is tracked in https://issues.redhat.com/browse/HOSTEDCP-279
		hyperShiftExceptions := sets.NewString(
			"openshift-multus/multus-admission-controller",
			"openshift-sdn/sdn-controller",
		)

		exceptions := sets.NewString(
			// Managed service exceptions https://issues.redhat.com/browse/OSD-26323
			"openshift-security/splunkforwarder-ds",
			"openshift-validation-webhook/validation-webhook",

			// This is a temporary daemon set used to deploy a config to nodes, and then
			// it is deleted later.
			"openshift-multus/cni-sysctl-allowlist-ds",
		)

		var debug []string
		var invalidDaemonSets []string
		for _, ds := range daemonSets.Items {
			if !strings.HasPrefix(ds.Namespace, "openshift-") {
				continue
			}
			if ds.Spec.Selector != nil {
				var labelSet = labels.Set(ds.Spec.Template.Spec.NodeSelector)
				if !labelSet.AsSelector().Empty() && masterReq.Matches(labelSet) {
					continue
				}
			}
			if *controlPlaneTopology == configv1.ExternalTopologyMode && hyperShiftExceptions.Has(ds.Namespace+"/"+ds.Name) {
				continue
			}
			if exceptions.Has(ds.Namespace + "/" + ds.Name) {
				continue
			}

			key := fmt.Sprintf("%s/%s", ds.Namespace, ds.Name)
			switch {
			case ds.Spec.UpdateStrategy.RollingUpdate == nil,
				ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable == nil:
				violation := fmt.Sprintf("expected daemonset %s to have a maxUnavailable strategy of 10%% or 33%%", key)
				invalidDaemonSets = append(invalidDaemonSets, violation)
				debug = append(debug, violation)
			case ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal != "10%" &&
				ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal != "33%" &&
				ds.Spec.UpdateStrategy.RollingUpdate.MaxSurge.StrVal != "10%":
				violation := fmt.Sprintf("expected daemonset %s to have maxUnavailable 10%% or 33%% (see comment) instead of %s, or maxSurge 10%% instead of %s", key, ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String(), ds.Spec.UpdateStrategy.RollingUpdate.MaxSurge.String())
				invalidDaemonSets = append(invalidDaemonSets, violation)
				debug = append(debug, violation)
			default:
				debug = append(debug, fmt.Sprintf("daemonset %s has %s", key, ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String()))
			}
		}

		sort.Strings(debug)
		e2e.Logf("Daemonset configuration in payload:\n%s", strings.Join(debug, "\n"))

		// Users are not allowed to add new violations
		if len(invalidDaemonSets) > 0 {
			e2e.Failf("Daemonsets found that do not meet platform requirements for update strategy:\n  %s", strings.Join(invalidDaemonSets, "\n  "))
		}
	})
})

package operators

import (
	"context"
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operator-workloads")

	// Workloads shipped to the platform that run off control-plane nodes should present a consistent
	// HA story by default. We focus on minimizing the impact of single node failures by ensuring workloads
	// are spread. We require 2 additional nodes when control-plane are not schedulable for workloads, which
	// we cannot increase without breaking old clusters.
	//
	// * If the workload is highly available (replica count > 1) and can tolerate the loss of one replica,
	//   the default configuration for that workload should be:
	//   * Two replicas
	//   * Hard anti-affinity on hostname (no two pods on the same node)
	//   * Use the maxUnavailable rollout strategy on deployments (prefer 25% by default for a value)
	// * If the workload requires >= 3 replicas:
	//   * Set soft spreading constraints so that pods prefer to land on separate nodes (will be violated
	//     on two node clusters)
	//   * Use maxSurge for deployments if possible since the spreading rules are soft
	//
	// In a future release we will introduce the descheduler by default, which will periodically rebalance
	// the cluster to ensure spreading for key components is honored. At that time we will remove hard
	// anti-affinity constraints, and recommend components move to a surge model during upgrades.
	g.It("should configure components that remain available through single node failures", func() {
		// iterate over the references to find valid images
		deployments, err := oc.KubeFramework().ClientSet.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list deployments: %v", err)
		}
		statefulsets, err := oc.KubeFramework().ClientSet.AppsV1().StatefulSets("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list statefulsets: %v", err)
		}
		var items []runtime.Object
		for i := range deployments.Items {
			items = append(items, &deployments.Items[i])
		}
		for i := range statefulsets.Items {
			items = append(items, &deployments.Items[i])
		}

		// workloads that are broken and need to be fixed, every entry here must have a bug associated
		// e.g. "<kind>/openshift-x/foo": "link to bug"
		knownBugs := map[string]string{}

		masterReq, err := labels.NewRequirement("node-role.kubernetes.io/master", selection.Exists, []string{})
		if err != nil {
			e2e.Failf("unable to build label requirement: %v", err)
		}

		var debug []string
		var knownBroken []string
		var invalidWorkloads []string
		for _, workload := range items {
			var meta *metav1.ObjectMeta
			var template *corev1.PodTemplateSpec
			switch t := workload.(type) {
			case *appsv1.Deployment:
				t.Kind = "Deployment"
				if t.Spec.Replicas == nil || *t.Spec.Replicas < 2 {
					continue
				}
				meta = &t.ObjectMeta
				template = &t.Spec.Template
			case *appsv1.StatefulSet:
				t.Kind = "StatefulSet"
				if t.Spec.Replicas == nil || *t.Spec.Replicas < 2 {
					continue
				}
				meta = &t.ObjectMeta
				template = &t.Spec.Template
			default:
				panic("not an object")
			}

			if !strings.HasPrefix(meta.Namespace, "openshift-") {
				continue
			}

			// exclude workloads that are master specific
			if template.Spec.NodeSelector != nil {
				if masterReq.Matches(labels.Set(template.Spec.NodeSelector)) {
					continue
				}
			}
			var toleratesMaster bool
			for _, tolerates := range template.Spec.Tolerations {
				if tolerates.Key == "node-role.kubernetes.io/master" && tolerates.Operator == corev1.TolerationOpExists {
					toleratesMaster = true
				}
			}
			if toleratesMaster {
				continue
			}

			key := fmt.Sprintf("%s/%s/%s", workload.GetObjectKind().GroupVersionKind().Kind, meta.Namespace, meta.Name)

			if !isHardAntiAffinity(template.Spec.Affinity, template.Spec.TopologySpreadConstraints) {
				rule := fmt.Sprintf("%s/%s", key, "use_hard_anti_affinity")
				violation := fmt.Sprintf("%s must set pod anti-affinity required during scheduling, with topologyKey 'kubernetes.io/hostname' so that multiple pods are not on the same node", key)
				if bug, ok := knownBugs[rule]; ok {
					knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
				} else {
					invalidWorkloads = append(invalidWorkloads, violation)
				}
			}

			// SPECIAL CASES:
			// * registry and router allow configuration of replicas and rollout strategy, we need to check the default one only and check for default config

			switch t := workload.(type) {
			case *appsv1.Deployment:
				if *t.Spec.Replicas > 2 {
					rule := fmt.Sprintf("%s/%s", key, "too_many_replicas")
					violation := fmt.Sprintf("%s: has %d replicas but is expected to have both hard affinity and fit onto 2 nodes", rule, *t.Spec.Replicas)
					if bug, ok := knownBugs[rule]; ok {
						knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
					} else {
						invalidWorkloads = append(invalidWorkloads, violation)
					}
				}
				switch {
				case t.Spec.Strategy.RollingUpdate != nil && t.Spec.Strategy.RollingUpdate.MaxSurge != nil:
					rule := fmt.Sprintf("%s/%s", key, "using_max_surge")
					violation := fmt.Sprintf("%s: using maxSurge, but has 2 replicas and with hard affinity would not fit into 2 nodes", rule)
					if bug, ok := knownBugs[rule]; ok {
						knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
					} else {
						invalidWorkloads = append(invalidWorkloads, violation)
					}
				// TODO: maybe this could be 33% too, hard to say whether this is truly the same
				case t.Spec.Strategy.RollingUpdate != nil && t.Spec.Strategy.RollingUpdate.MaxUnavailable != nil && t.Spec.Strategy.RollingUpdate.MaxUnavailable.String() != "25%":
					rule := fmt.Sprintf("%s/%s", key, "non_default_max_unavailable")
					violation := fmt.Sprintf("%s: has maxUnavailable %q, but should be using 25%% for consistency", rule, t.Spec.Strategy.RollingUpdate.MaxUnavailable.String())
					if bug, ok := knownBugs[rule]; ok {
						knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
					} else {
						invalidWorkloads = append(invalidWorkloads, violation)
					}
				}
			case *appsv1.StatefulSet:
				if *t.Spec.Replicas > 2 {
					rule := fmt.Sprintf("%s/%s", key, "too_many_replicas")
					violation := fmt.Sprintf("%s: has %d replicas but is expected to have both hard affinity and fit onto 2 nodes", rule, *t.Spec.Replicas)
					if bug, ok := knownBugs[rule]; ok {
						knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
					} else {
						invalidWorkloads = append(invalidWorkloads, violation)
					}
				}
			}
		}

		sort.Strings(debug)
		e2e.Logf("Workload configuration in payload:\n%s", strings.Join(debug, "\n"))

		// All known bugs are listed as flakes so we can see them as dashboards
		if len(knownBroken) > 0 {
			sort.Strings(knownBroken)
			result.Flakef("Workloads with outstanding bugs in payload:\n%s", strings.Join(knownBroken, "\n"))
		}

		// Users are not allowed to add new violations
		if len(invalidWorkloads) > 0 {
			e2e.Failf("Workloads found that do not meet platform requirements for HA strategy:\n  %s", strings.Join(invalidWorkloads, "\n  "))
		}
	})
})

func isHardAntiAffinity(affinity *corev1.Affinity, spread []corev1.TopologySpreadConstraint) bool {
	if affinity != nil && affinity.PodAntiAffinity != nil {
		for _, term := range affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			if term.TopologyKey == "kubernetes.io/hostname" {
				return true
			}
		}
	}
	return false
}

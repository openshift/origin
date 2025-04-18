package operators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operator-resources")

	// Pods that are part of the control plane should set both cpu and memory requests, but require an exception
	// to set limits on memory (CPU limits are generally not allowed). This enforces the rules described in
	// https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md#resources-and-limits.
	//
	// This test enforces all pods in the openshift-*, kube-*, and default namespace have requests set for both
	// CPU and memory, and no limits set. Known bugs will transform this to a flake. Otherwise the test will fail.
	//
	// Release architects can justify an exception with text but must ensure CONVENTIONS.md is updated to document
	// why the exception is granted.
	g.It("should set requests but not limits", func() {
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		exemptNamespaces := append([]string{
			// Must-gather runs are excluded from this rule
			"openshift-must-gather",
		},
			// Managed service namespaces - https://issues.redhat.com/browse/OSD-21708
			exutil.ManagedServiceNamespaces.UnsortedList()...,
		)

		// pods that have a bug opened, every entry here must have a bug associated
		knownBrokenPods := map[string]string{
			//"<apiVersion>/<kind>/<namespace>/<name>/(initContainer|container)/<container_name>/<violation_type>": "<url to bug>",

			// Managed service pods that have limits but not requests, that are in platform namespaces
			"apps/v1/Deployment/openshift-monitoring/configure-alertmanager-operator/container/configure-alertmanager-operator/limit[cpu]":      "https://issues.redhat.com/browse/OSD-21708",
			"apps/v1/Deployment/openshift-monitoring/configure-alertmanager-operator/container/configure-alertmanager-operator/request[cpu]":    "https://issues.redhat.com/browse/OSD-21708",
			"apps/v1/Deployment/openshift-monitoring/configure-alertmanager-operator/container/configure-alertmanager-operator/limit[memory]":   "https://issues.redhat.com/browse/OSD-21708",
			"apps/v1/Deployment/openshift-monitoring/configure-alertmanager-operator/container/configure-alertmanager-operator/request[memory]": "https://issues.redhat.com/browse/OSD-21708",
			"batch/v1/Job/openshift-monitoring/<batch_job>/container/osd-cluster-ready/request[cpu]":                                            "https://issues.redhat.com/browse/OSD-21708",
			"batch/v1/Job/openshift-monitoring/<batch_job>/container/osd-cluster-ready/request[memory]":                                         "https://issues.redhat.com/browse/OSD-21708",
			"batch/v1/Job/openshift-monitoring/<batch_job>/container/osd-rebalance-infra-nodes/request[cpu]":                                    "https://issues.redhat.com/browse/OSD-21708",
			"batch/v1/Job/openshift-monitoring/<batch_job>/container/osd-rebalance-infra-nodes/request[memory]":                                 "https://issues.redhat.com/browse/OSD-21708",

			// ovn pods
			"apps/v1/DaemonSet/openshift-multus/cni-sysctl-allowlist-ds/container/kube-multus-additional-cni-plugins/request[cpu]":    "https://issues.redhat.com/browse/TRT-1871",
			"apps/v1/DaemonSet/openshift-multus/cni-sysctl-allowlist-ds/container/kube-multus-additional-cni-plugins/request[memory]": "https://issues.redhat.com/browse/TRT-1871",
		}

		// pods with an exception granted, the value should be the justification and the approver (a release architect)
		exceptionGranted := map[string]string{
			//"<apiVersion>/<kind>/<namespace>/<name>/(initContainer|container)/<container_name>/<violation_type>": "<github handle of approver>: <brief description of the reason for the exception>",

			// CPU limits on these containers may be inappropriate in the future
			"v1/Pod/openshift-etcd/installer-<revision>-<node>/container/installer/limit[cpu]":                          "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-etcd/installer-<revision>-<node>/container/installer/limit[memory]":                       "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-etcd/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":                       "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-etcd/revision-pruner-<revision>-<node>/container/pruner/limit[memory]":                    "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/installer-<revision>-<node>/container/installer/limit[cpu]":                "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/installer-<revision>-<node>/container/installer/limit[memory]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/revision-pruner-<revision>-<node>/container/pruner/limit[memory]":          "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/installer-<revision>-<node>/container/installer/limit[cpu]":       "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/installer-<revision>-<node>/container/installer/limit[memory]":    "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":    "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/revision-pruner-<revision>-<node>/container/pruner/limit[memory]": "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/installer-<revision>-<node>/container/installer/limit[cpu]":                "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/installer-<revision>-<node>/container/installer/limit[memory]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/revision-pruner-<revision>-<node>/container/pruner/limit[memory]":          "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",

			"apps/v1/Deployment/openshift-monitoring/thanos-querier/container/thanos-query/limit[memory]": "smarterclayton: granted a temporary exception (reasses in 4.10) until Thanos can properly control resource usage from arbitrary queries",

			"apps/v1/DaemonSet/openshift-network-operator/iptables-alerter/container/iptables-alerter/limit[cpu]": "sdodson: supposed to be in the background, doesn't care if it gets throttled or delayed",
		}

		reNormalizeRunOnceNames := regexp.MustCompile(`^(installer-|revision-pruner-)[\d]+-`)
		reNormalizeRetryNames := regexp.MustCompile(`-retry-[\d]+-`)

		waitingForFix := sets.NewString()
		notAllowed := sets.NewString()
		possibleFuture := sets.NewString()
	podLoop:
		for _, pod := range pods.Items {
			// Only pods in the openshift-*, kube-*, and default namespaces are considered
			if !strings.HasPrefix(pod.Namespace, "openshift-") && !strings.HasPrefix(pod.Namespace, "kube-") && pod.Namespace != "default" {
				continue
			}

			for _, ns := range exemptNamespaces {
				if pod.Namespace == ns {
					continue podLoop
				}
			}
			// var controlPlaneTarget bool
			// selector := labels.SelectorFromSet(pod.Spec.NodeSelector)
			// if !selector.Empty() && selector.Matches(labels.Set(map[string]string{"node-role.kubernetes.io/master": ""})) {
			// 	controlPlaneTarget = true
			// }

			// Find a unique string that identifies who creates the pod, or the pod itself
			var controller string
			for _, ref := range pod.OwnerReferences {
				if ref.Controller == nil || !*ref.Controller {
					continue
				}
				// simple hack to make the rules cluster better, if we get new hierarchies just add more checks here
				switch ref.Kind {
				case "ReplicaSet":
					if i := strings.LastIndex(ref.Name, "-"); i != -1 {
						name := ref.Name[0:i]
						if deploy, err := oc.KubeFramework().ClientSet.AppsV1().Deployments(pod.Namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
							if apierrors.IsNotFound(err) {
								e2e.Logf("ignoring replicaset %s because no owning deployment %s exists", ref.Name, name)
								// Ignore this pod entirely because it most likely
								// belongs to an orphaned replicaset.
								continue podLoop
							} else {
								e2e.Failf("unable to get deployment %s for replicaset %s: %v", name, ref.Name, err)
							}
						} else {
							ref.Name = deploy.Name
							ref.Kind = "Deployment"
							ref.APIVersion = "apps/v1"
						}
					}
				case "Job":
					ref.Name = "<batch_job>"
				case "Node":
					continue
				}
				controller = fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, pod.Namespace, ref.Name)
				break
			}
			if len(controller) == 0 {
				if len(pod.GenerateName) > 0 {
					name := strings.ReplaceAll(pod.GenerateName, pod.Spec.NodeName, "<node>")
					if pod.Spec.RestartPolicy != v1.RestartPolicyAlways {
						name = reNormalizeRunOnceNames.ReplaceAllString(name, "$1<revision>")
					}
					controller = fmt.Sprintf("v1/Pod/%s/%s", pod.Namespace, name)
				} else {
					name := strings.ReplaceAll(pod.Name, pod.Spec.NodeName, "<node>")
					if pod.Spec.RestartPolicy != v1.RestartPolicyAlways {
						name = reNormalizeRunOnceNames.ReplaceAllString(name, "$1<revision>-")
					}
					controller = fmt.Sprintf("v1/Pod/%s/%s", pod.Namespace, name)
				}
			}

			// Remove -retry-#- for, e.g. openshift-etcd/installer-<revision>-retry-1-<node>
			controller = reNormalizeRetryNames.ReplaceAllString(controller, "-")

			// These rules apply to both init and regular containers
			for containerType, containers := range map[string][]v1.Container{
				"initContainer": pod.Spec.InitContainers,
				"container":     pod.Spec.Containers,
			} {
				for _, c := range containers {
					key := fmt.Sprintf("%s/%s/%s", controller, containerType, c.Name)

					// Pods may not set limits
					if len(c.Resources.Limits) > 0 {
						for resource, v := range c.Resources.Limits {
							if resource == "management.workload.openshift.io/cores" { // limits are allowed on management.workload.openshift.io/cores resources
								continue
							}
							rule := fmt.Sprintf("%s/%s[%s]", key, "limit", resource)
							if len(exceptionGranted[rule]) == 0 {
								violation := fmt.Sprintf("%s defines a limit on %s of %s which is not allowed", key, resource, v.String())
								if bug, ok := knownBrokenPods[rule]; ok {
									waitingForFix.Insert(fmt.Sprintf("%s (bug %s)", violation, bug))
								} else {
									notAllowed.Insert(fmt.Sprintf("%s (rule: %q)", violation, rule))
								}
							}
						}
					}

					annotationForPreferringManagementCores := "target.workload.openshift.io/management"
					wpPodAnnotation := strings.Replace(pod.Annotations[annotationForPreferringManagementCores], " ", "", -1) // some pods have a space after the : in their annotation definition

					// Pods must have at least CPU and memory requests
					for _, resource := range []string{"cpu", "memory"} {
						if len(wpPodAnnotation) > 0 && resource == "cpu" { // don't check for CPU request if the pod has a WP annotation
							continue
						}

						v := c.Resources.Requests[v1.ResourceName(resource)]
						if !v.IsZero() {
							continue
						}
						rule := fmt.Sprintf("%s/%s[%s]", key, "request", resource)
						violation := fmt.Sprintf("%s does not have a %s request", key, resource)
						if len(exceptionGranted[rule]) == 0 {
							if bug, ok := knownBrokenPods[rule]; ok {
								waitingForFix.Insert(fmt.Sprintf("%s (bug %s)", violation, bug))
							} else {
								if containerType == "initContainer" {
									possibleFuture.Insert(fmt.Sprintf("%s (candidate rule: %q)", violation, rule))
								} else {
									notAllowed.Insert(fmt.Sprintf("%s (rule: %q)", violation, rule))
								}
							}
						}
					}
				}
			}
		}

		// Some things we may start checking in the future
		if len(possibleFuture) > 0 {
			e2e.Logf("Pods in platform namespaces had resource request/limit that we may enforce in the future:\n\n%s", strings.Join(possibleFuture.List(), "\n"))
		}

		// Users are not allowed to add new violations
		if len(notAllowed) > 0 {
			e2e.Failf("Pods in platform namespaces are not following resource request/limit rules or do not have an exception granted:\n  %s", strings.Join(notAllowed.List(), "\n  "))
		}

		// All known bugs are listed as flakes so we can see them as dashboards
		if len(waitingForFix) > 0 {
			result.Flakef("Pods in platform namespaces had known broken resource request/limit that have not been resolved:\n\n%s", strings.Join(waitingForFix.List(), "\n"))
		}
	})
})

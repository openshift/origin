package scheduler

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/component-base/metrics/legacyregistry"
)

const (
	// UnitByte is the unit of measure in bytes.
	unitByte = "byte"
	// UnitCore is the unit of measure in CPU cores.
	unitCore = "core"
	// UnitInteger is the unit of measure in integers.
	unitInteger = "integer"
)

var resourcesOnce sync.Once

// registerResourceMetrics registers a O(pods) cardinality metric that
// reports the current resources requested by all pods on the cluster within
// the Kubernetes resource model. Metrics are broken down by pod, node, resource,
// and phase of lifecycle. Each pod returns two series per resource - one for
// their aggregate usage (required to schedule) and one for their phase specific
// usage. This allows admins to assess the cost per resource at different phases
// of startup and compare to actual resource usage.
func registerResourceMetrics(podLister corelisters.PodLister, nodeLister corelisters.NodeLister) {
	resourcesOnce.Do(func() {
		podResources := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Subsystem: "kube_scheduler",
				Name:      "pod_resources",
				Help:      "Resources requested by workloads on the cluster, broken down by lifecyle phase and pod.",
			},
			[]string{"namespace", "pod", "lifecycle", "container", "type", "node", "resource", "unit"},
		)
		nodeResources := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Subsystem: "kube_scheduler",
				Name:      "node_resources",
				Help:      "Resources provided by nodes in the cluster.",
			},
			[]string{"node", "resource", "unit"},
		)

		descCh := make(chan *prometheus.Desc, 1)

		nodeResources.Describe(descCh)
		legacyregistry.RawMustRegister(&nodeResourceCollector{
			desc:   <-descCh,
			lister: nodeLister,
		})
		podResources.Describe(descCh)
		legacyregistry.RawMustRegister(&podResourceCollector{
			desc:   <-descCh,
			lister: podLister,
		})
	})
}

type podResourceCollector struct {
	desc   *prometheus.Desc
	lister corelisters.PodLister
}

func (c *podResourceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *podResourceCollector) Collect(ch chan<- prometheus.Metric) {
	desc := c.desc
	nodes, err := c.lister.List(labels.Everything())
	if err != nil {
		return
	}
	for _, p := range nodes {
		lifecycle, reqs, currentReqs, limits, currentLimits := podRequestsAndLimitsByLifecycle(p)
		for _, t := range []struct {
			lifecycle string
			name      string
			resources v1.ResourceList
		}{
			{lifecycle: "", name: "requests", resources: currentReqs},
			{lifecycle: lifecycle, name: "requests", resources: reqs},
			{lifecycle: "", name: "limits", resources: currentLimits},
			{lifecycle: lifecycle, name: "limits", resources: limits},
		} {
			req := t.resources
			for resourceName, val := range req {
				switch resourceName {
				case v1.ResourceCPU:
					ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
						float64(val.MilliValue())/1000,
						p.Namespace, p.Name, t.lifecycle, "", t.name, p.Spec.NodeName, sanitizeLabelName(string(resourceName)), string(unitCore),
					)
				case v1.ResourceStorage:
					fallthrough
				case v1.ResourceEphemeralStorage:
					fallthrough
				case v1.ResourceMemory:
					ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
						float64(val.Value()),
						p.Namespace, p.Name, t.lifecycle, "", t.name, p.Spec.NodeName, sanitizeLabelName(string(resourceName)), string(unitByte),
					)
				default:
					if isHugePageResourceName(resourceName) {
						ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
							float64(val.Value()),
							p.Namespace, p.Name, t.lifecycle, "", t.name, p.Spec.NodeName, sanitizeLabelName(string(resourceName)), string(unitByte),
						)
					}
					if isAttachableVolumeResourceName(resourceName) {
						ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
							float64(val.Value()),
							p.Namespace, p.Name, t.lifecycle, "", t.name, p.Spec.NodeName, sanitizeLabelName(string(resourceName)), string(unitByte),
						)
					}
					if isExtendedResourceName(resourceName) {
						ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
							float64(val.Value()),
							p.Namespace, p.Name, t.lifecycle, "", t.name, p.Spec.NodeName, sanitizeLabelName(string(resourceName)), string(unitInteger),
						)
					}
				}
			}
		}
	}
}

type nodeResourceCollector struct {
	desc   *prometheus.Desc
	lister corelisters.NodeLister
}

func (c *nodeResourceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *nodeResourceCollector) Collect(ch chan<- prometheus.Metric) {
	desc := c.desc
	nodes, err := c.lister.List(labels.Everything())
	if err != nil {
		return
	}
	for _, n := range nodes {
		req := n.Status.Allocatable
		for resourceName, val := range req {
			switch resourceName {
			case v1.ResourceCPU:
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
					float64(val.MilliValue())/1000,
					n.Name, sanitizeLabelName(string(resourceName)), string(unitCore),
				)
			case v1.ResourceStorage:
				fallthrough
			case v1.ResourceEphemeralStorage:
				fallthrough
			case v1.ResourceMemory:
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
					float64(val.Value()),
					n.Name, sanitizeLabelName(string(resourceName)), string(unitByte),
				)
			default:
				if isHugePageResourceName(resourceName) {
					ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
						float64(val.Value()),
						n.Name, sanitizeLabelName(string(resourceName)), string(unitByte),
					)
				}
				if isAttachableVolumeResourceName(resourceName) {
					ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
						float64(val.Value()),
						n.Name, sanitizeLabelName(string(resourceName)), string(unitByte),
					)
				}
				if isExtendedResourceName(resourceName) {
					ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
						float64(val.Value()),
						n.Name, sanitizeLabelName(string(resourceName)), string(unitInteger),
					)
				}
			}
		}
	}
}

// addResourceList adds the resources in newList to list
func addResourceList(list, newList v1.ResourceList) {
	for name, quantity := range newList {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list
func maxResourceList(list, new v1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
			continue
		} else {
			if quantity.Cmp(value) > 0 {
				list[name] = quantity.DeepCopy()
			}
		}
	}
}

// podRequestsAndLimitsByLifecycle returns a dictionary of all defined resources summed up for all
// containers of the pod. If PodOverhead feature is enabled, pod overhead is added to the
// total container resource requests and to the total container limits which have a
// non-zero quantity.
func podRequestsAndLimitsByLifecycle(pod *v1.Pod) (lifecycle string, reqs, currentReqs, limits, currentLimits v1.ResourceList) {
	var terminal, initializing, running bool
	switch {
	case len(pod.Spec.NodeName) == 0:
		lifecycle = "Pending"
	case pod.Status.Phase == v1.PodSucceeded, pod.Status.Phase == v1.PodFailed:
		lifecycle = "Completed"
		terminal = true
	default:
		if len(pod.Spec.InitContainers) > 0 && !hasConditionStatus(pod.Status.Conditions, v1.PodInitialized, v1.ConditionTrue) {
			lifecycle = "Initializing"
			initializing = true
		} else {
			lifecycle = "Running"
			running = true
		}
	}
	if terminal {
		return
	}

	reqs, limits, currentReqs, currentLimits = make(v1.ResourceList, 4), make(v1.ResourceList, 4), make(v1.ResourceList, 4), make(v1.ResourceList, 4)
	for _, container := range pod.Spec.Containers {
		addResourceList(reqs, container.Resources.Requests)
		addResourceList(limits, container.Resources.Limits)

		if running {
			addResourceList(currentReqs, container.Resources.Requests)
			addResourceList(currentLimits, container.Resources.Limits)
		}
	}
	// init containers define the minimum of any resource
	var currentInitializingContainer string
	if len(pod.Spec.InitContainers) > 0 {
		currentInitializingContainer = pod.Spec.InitContainers[0].Name
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if status.State.Terminated != nil {
			continue
		}
		currentInitializingContainer = status.Name
		break
	}
	for _, container := range pod.Spec.InitContainers {
		maxResourceList(reqs, container.Resources.Requests)
		maxResourceList(limits, container.Resources.Limits)

		if initializing && currentInitializingContainer == container.Name {
			maxResourceList(currentReqs, container.Resources.Requests)
			maxResourceList(currentLimits, container.Resources.Limits)
		}
	}

	// if PodOverhead feature is supported, add overhead for running a pod
	// to the sum of reqeuests and to non-zero limits:
	if pod.Spec.Overhead != nil {
		addResourceList(reqs, pod.Spec.Overhead)
		for name, quantity := range pod.Spec.Overhead {
			if value, ok := limits[name]; ok && !value.IsZero() {
				value.Add(quantity)
				limits[name] = value
			}
		}
		if initializing || running {
			addResourceList(reqs, pod.Spec.Overhead)
			for name, quantity := range pod.Spec.Overhead {
				if value, ok := limits[name]; ok && !value.IsZero() {
					value.Add(quantity)
					limits[name] = value
				}
			}
		}
	}

	return
}

func hasConditionStatus(conditions []v1.PodCondition, name v1.PodConditionType, status v1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type != name {
			continue
		}
		return condition.Status == status
	}
	return false
}

var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeLabelName(s string) string {
	return invalidLabelCharRE.ReplaceAllString(s, "_")
}

func isHugePageResourceName(name v1.ResourceName) bool {
	return strings.HasPrefix(string(name), v1.ResourceHugePagesPrefix)
}

func isAttachableVolumeResourceName(name v1.ResourceName) bool {
	return strings.HasPrefix(string(name), v1.ResourceAttachableVolumesPrefix)
}

func isExtendedResourceName(name v1.ResourceName) bool {
	if isNativeResource(name) || strings.HasPrefix(string(name), v1.DefaultResourceRequestsPrefix) {
		return false
	}
	// Ensure it satisfies the rules in IsQualifiedName() after converted into quota resource name
	nameForQuota := fmt.Sprintf("%s%s", v1.DefaultResourceRequestsPrefix, string(name))
	if errs := validation.IsQualifiedName(nameForQuota); len(errs) != 0 {
		return false
	}
	return true
}

func isNativeResource(name v1.ResourceName) bool {
	return !strings.Contains(string(name), "/") ||
		isPrefixedNativeResource(name)
}

func isPrefixedNativeResource(name v1.ResourceName) bool {
	return strings.Contains(string(name), v1.ResourceDefaultNamespacePrefix)
}

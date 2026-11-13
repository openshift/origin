package nodeselectoradjuster

// The NodeSelectorAdjuster admission plugin adds the
// node-role.kubernetes.io/control-plane node selector to qualifying pods. It only
// activates on standalone OpenShift clusters, detected by
// POD_NAMESPACE=openshift-kube-apiserver. On hosted control plane (HCP)
// clusters the plugin does not register itself and takes no action, allowing
// qualifying pods to be scheduled on data plane worker nodes without
// modification.

import (
	"context"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/admission"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
)

const (
	// PluginName is the name used to identify this plugin in the admission chain.
	PluginName = "scheduling.openshift.io/NodeSelectorAdjuster"

	// controlPlaneRoleKey is the node role label used as a node selector key
	controlPlaneRoleKey = "node-role.kubernetes.io/control-plane"

	// vpaOperatorLabelKey / vpaOperatorLabelValue identify the VPA operator pod.
	vpaOperatorLabelKey   = "k8s-app"
	vpaOperatorLabelValue = "vertical-pod-autoscaler-operator"
	// vpaOperatorNamespace is the namespace the VPA operator is expected to run in.
	vpaOperatorNamespace = "openshift-vertical-pod-autoscaler"

	// croOperatorLabelKey / croOperatorLabelValue identify the CRO operator pod.
	croOperatorLabelKey   = "clusterresourceoverride.operator"
	croOperatorLabelValue = "true"
	// croOperatorNamespace is the namespace the CRO operator is expected to run in.
	croOperatorNamespace = "openshift-cluster-resource-override"

	// cmaOperatorLabelKey / cmaOperatorLabelValue identify the CMA operator pod.
	cmaOperatorLabelKey   = "name"
	cmaOperatorLabelValue = "custom-metrics-autoscaler-operator"

	// cmaOperatorNamespace is the namespace the CMA operator is expected to run in.
	cmaOperatorNamespace = "openshift-keda"

	// standaloneEnvVar is the environment variable checked at start-up.
	// It is injected by the downward API and reflects the namespace the
	// kube-apiserver pod runs in.
	standaloneEnvVar = "POD_NAMESPACE"
	// standaloneEnvValue is the namespace used by the kube-apiserver on a
	// standalone OpenShift cluster.
	standaloneEnvValue = "openshift-kube-apiserver"
)

// IsStandalone reports whether the current process is running inside a standalone
// OpenShift cluster. It is checked once at start-up to decide whether the plugin
// should register itself.
func IsStandalone() bool {
	return os.Getenv(standaloneEnvVar) == standaloneEnvValue
}

// Register adds the plugin to the admission plugin registry. It must only be
// called when IsStandalone() returns true.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(_ io.Reader) (admission.Interface, error) {
		return &nodeSelectorAdjuster{
			Handler: admission.NewHandler(admission.Create),
		}, nil
	})
}

// nodeSelectorAdjuster implements admission.MutationInterface.
type nodeSelectorAdjuster struct {
	*admission.Handler
}

var _ admission.MutationInterface = &nodeSelectorAdjuster{}

// Admit examines newly-created Pod objects and, for qualifying pods, adds the control-plane
// node selector so that they run on control-plane nodes on standalone clusters.
func (p *nodeSelectorAdjuster) Admit(_ context.Context, attr admission.Attributes, _ admission.ObjectInterfaces) error {
	if attr.GetResource().GroupResource() != corev1.Resource("pods") || attr.GetSubresource() != "" {
		return nil
	}

	pod, ok := attr.GetObject().(*coreapi.Pod)
	if !ok {
		return admission.NewForbidden(attr, fmt.Errorf("unexpected object type: %T", attr.GetObject()))
	}

	if !requiresNodeSelectorAdjustment(pod) {
		return nil
	}

	addControlPlaneNodeSelector(pod)
	return nil
}

// ValidateInitialization satisfies admission.InitializationValidator. The plugin
// has no external dependencies to validate.
func (p *nodeSelectorAdjuster) ValidateInitialization() error {
	return nil
}

// requiresNodeSelectorAdjustment returns true when the pod carries a label that
// opts it in to control-plane node placement and lives in a namespace where that
// label is expected. Control-plane-adjacent Day 2 operators can be added here.
func requiresNodeSelectorAdjustment(pod *coreapi.Pod) bool {
	// for VPA, we only want to update if the node selector is the default from
	// https://github.com/openshift/vertical-pod-autoscaler-operator/blob/main/config/manager/manager.yaml
	if pod.Labels[vpaOperatorLabelKey] == vpaOperatorLabelValue &&
		pod.Namespace == vpaOperatorNamespace && len(pod.Spec.NodeSelector) == 1 &&
		pod.Spec.NodeSelector["kubernetes.io/os"] == "linux" {
		return true
	}
	// for CRO, we only want to update if the node selector empty
	if pod.Labels[croOperatorLabelKey] == croOperatorLabelValue &&
		pod.Namespace == croOperatorNamespace && len(pod.Spec.NodeSelector) == 0 {
		return true
	}
	// for CMA, we want to update if the node selector is empty
	// and if it has a toleration that would tolerate the master NoSchedule taint
	if pod.Labels[cmaOperatorLabelKey] == cmaOperatorLabelValue &&
		pod.Namespace == cmaOperatorNamespace && len(pod.Spec.NodeSelector) == 0 {
		masterTaint := coreapi.Taint{
			Key:    "node-role.kubernetes.io/master",
			Effect: coreapi.TaintEffectNoSchedule,
		}
		for _, tol := range pod.Spec.Tolerations {
			if toleratesTaint(tol, masterTaint) {
				return true
			}
		}
	}
	return false
}

// toleratesTaint checks if a toleration tolerates a given taint, following the
// same rules as corev1.Toleration.ToleratesTaint: an empty effect matches all
// effects, the Exists operator matches any value, and an empty key with Exists
// matches all keys.
func toleratesTaint(tol coreapi.Toleration, taint coreapi.Taint) bool {
	if len(tol.Effect) > 0 && tol.Effect != taint.Effect {
		return false
	}
	if len(tol.Key) > 0 && tol.Key != taint.Key {
		return false
	}
	switch tol.Operator {
	case "", coreapi.TolerationOpEqual:
		return tol.Value == taint.Value
	case coreapi.TolerationOpExists:
		return true
	default:
		return false
	}
}

// addControlPlaneNodeSelector ensures spec.nodeSelector contains the control-plane role key.
func addControlPlaneNodeSelector(pod *coreapi.Pod) {
	if pod.Spec.NodeSelector == nil {
		pod.Spec.NodeSelector = map[string]string{}
	}
	pod.Spec.NodeSelector[controlPlaneRoleKey] = ""
}

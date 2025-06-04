/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podtopologyspread

import (
	"context"
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/validation"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/parallelize"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/feature"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/names"
	"k8s.io/kubernetes/pkg/scheduler/util"
)

const (
	// ErrReasonConstraintsNotMatch is used for PodTopologySpread filter error.
	ErrReasonConstraintsNotMatch = "node(s) didn't match pod topology spread constraints"
	// ErrReasonNodeLabelNotMatch is used when the node doesn't hold the required label.
	ErrReasonNodeLabelNotMatch = ErrReasonConstraintsNotMatch + " (missing required label)"
)

var systemDefaultConstraints = []v1.TopologySpreadConstraint{
	{
		TopologyKey:       v1.LabelHostname,
		WhenUnsatisfiable: v1.ScheduleAnyway,
		MaxSkew:           3,
	},
	{
		TopologyKey:       v1.LabelTopologyZone,
		WhenUnsatisfiable: v1.ScheduleAnyway,
		MaxSkew:           5,
	},
}

// PodTopologySpread is a plugin that ensures pod's topologySpreadConstraints is satisfied.
type PodTopologySpread struct {
	systemDefaulted                              bool
	parallelizer                                 parallelize.Parallelizer
	defaultConstraints                           []v1.TopologySpreadConstraint
	sharedLister                                 framework.SharedLister
	services                                     corelisters.ServiceLister
	replicationCtrls                             corelisters.ReplicationControllerLister
	replicaSets                                  appslisters.ReplicaSetLister
	statefulSets                                 appslisters.StatefulSetLister
	enableNodeInclusionPolicyInPodTopologySpread bool
	enableMatchLabelKeysInPodTopologySpread      bool
}

var _ framework.PreFilterPlugin = &PodTopologySpread{}
var _ framework.FilterPlugin = &PodTopologySpread{}
var _ framework.PreScorePlugin = &PodTopologySpread{}
var _ framework.ScorePlugin = &PodTopologySpread{}
var _ framework.EnqueueExtensions = &PodTopologySpread{}

// Name is the name of the plugin used in the plugin registry and configurations.
const Name = names.PodTopologySpread

// Name returns name of the plugin. It is used in logs, etc.
func (pl *PodTopologySpread) Name() string {
	return Name
}

// New initializes a new plugin and returns it.
func New(_ context.Context, plArgs runtime.Object, h framework.Handle, fts feature.Features) (framework.Plugin, error) {
	if h.SnapshotSharedLister() == nil {
		return nil, fmt.Errorf("SnapshotSharedlister is nil")
	}
	args, err := getArgs(plArgs)
	if err != nil {
		return nil, err
	}
	if err := validation.ValidatePodTopologySpreadArgs(nil, &args); err != nil {
		return nil, err
	}
	pl := &PodTopologySpread{
		parallelizer:       h.Parallelizer(),
		sharedLister:       h.SnapshotSharedLister(),
		defaultConstraints: args.DefaultConstraints,
		enableNodeInclusionPolicyInPodTopologySpread: fts.EnableNodeInclusionPolicyInPodTopologySpread,
		enableMatchLabelKeysInPodTopologySpread:      fts.EnableMatchLabelKeysInPodTopologySpread,
	}
	if args.DefaultingType == config.SystemDefaulting {
		pl.defaultConstraints = systemDefaultConstraints
		pl.systemDefaulted = true
	}
	if len(pl.defaultConstraints) != 0 {
		if h.SharedInformerFactory() == nil {
			return nil, fmt.Errorf("SharedInformerFactory is nil")
		}
		pl.setListers(h.SharedInformerFactory())
	}
	return pl, nil
}

func getArgs(obj runtime.Object) (config.PodTopologySpreadArgs, error) {
	ptr, ok := obj.(*config.PodTopologySpreadArgs)
	if !ok {
		return config.PodTopologySpreadArgs{}, fmt.Errorf("want args to be of type PodTopologySpreadArgs, got %T", obj)
	}
	return *ptr, nil
}

func (pl *PodTopologySpread) setListers(factory informers.SharedInformerFactory) {
	pl.services = factory.Core().V1().Services().Lister()
	pl.replicationCtrls = factory.Core().V1().ReplicationControllers().Lister()
	pl.replicaSets = factory.Apps().V1().ReplicaSets().Lister()
	pl.statefulSets = factory.Apps().V1().StatefulSets().Lister()
}

// EventsToRegister returns the possible events that may make a Pod
// failed by this plugin schedulable.
func (pl *PodTopologySpread) EventsToRegister(_ context.Context) ([]framework.ClusterEventWithHint, error) {
	return []framework.ClusterEventWithHint{
		// All ActionType includes the following events:
		// - Add. An unschedulable Pod may fail due to violating topology spread constraints,
		// adding an assigned Pod may make it schedulable.
		// - UpdatePodLabel. Updating on an existing Pod's labels (e.g., removal) may make
		// an unschedulable Pod schedulable.
		// - Delete. An unschedulable Pod may fail due to violating an existing Pod's topology spread constraints,
		// deleting an existing Pod may make it schedulable.
		{Event: framework.ClusterEvent{Resource: framework.Pod, ActionType: framework.Add | framework.UpdatePodLabel | framework.Delete}, QueueingHintFn: pl.isSchedulableAfterPodChange},
		// Node add|delete|update maybe lead an topology key changed,
		// and make these pod in scheduling schedulable or unschedulable.
		//
		// A note about UpdateNodeTaint event:
		// NodeAdd QueueingHint isn't always called because of the internal feature called preCheck.
		// As a common problematic scenario,
		// when a node is added but not ready, NodeAdd event is filtered out by preCheck and doesn't arrive.
		// In such cases, this plugin may miss some events that actually make pods schedulable.
		// As a workaround, we add UpdateNodeTaint event to catch the case.
		// We can remove UpdateNodeTaint when we remove the preCheck feature.
		// See: https://github.com/kubernetes/kubernetes/issues/110175
		{Event: framework.ClusterEvent{Resource: framework.Node, ActionType: framework.Add | framework.Delete | framework.UpdateNodeLabel | framework.UpdateNodeTaint}, QueueingHintFn: pl.isSchedulableAfterNodeChange},
	}, nil
}

func involvedInTopologySpreading(incomingPod, podWithSpreading *v1.Pod) bool {
	return incomingPod.Spec.NodeName != "" && incomingPod.Namespace == podWithSpreading.Namespace
}

func (pl *PodTopologySpread) isSchedulableAfterPodChange(logger klog.Logger, pod *v1.Pod, oldObj, newObj interface{}) (framework.QueueingHint, error) {
	originalPod, modifiedPod, err := util.As[*v1.Pod](oldObj, newObj)
	if err != nil {
		return framework.Queue, err
	}

	if (modifiedPod != nil && !involvedInTopologySpreading(modifiedPod, pod)) || (originalPod != nil && !involvedInTopologySpreading(originalPod, pod)) {
		logger.V(5).Info("the added/updated/deleted pod is unscheduled or has different namespace with target pod, so it doesn't make the target pod schedulable",
			"pod", klog.KObj(pod), "originalPod", klog.KObj(originalPod))
		return framework.QueueSkip, nil
	}

	constraints, err := pl.getConstraints(pod)
	if err != nil {
		return framework.Queue, err
	}

	// Pod is modified. Return Queue when the label(s) matching topologySpread's selector is added, changed, or deleted.
	if modifiedPod != nil && originalPod != nil {
		if reflect.DeepEqual(modifiedPod.Labels, originalPod.Labels) {
			logger.V(5).Info("the updated pod is unscheduled or has no updated labels or has different namespace with target pod, so it doesn't make the target pod schedulable",
				"pod", klog.KObj(pod), "modifiedPod", klog.KObj(modifiedPod))
			return framework.QueueSkip, nil
		}
		for _, c := range constraints {
			if c.Selector.Matches(labels.Set(originalPod.Labels)) != c.Selector.Matches(labels.Set(modifiedPod.Labels)) {
				// This modification makes this Pod match(or not match) with this constraint.
				// Maybe now the scheduling result of topology spread gets changed by this change.
				logger.V(5).Info("a scheduled pod's label was updated and it makes the updated pod match or unmatch the pod's topology spread constraints",
					"pod", klog.KObj(pod), "modifiedPod", klog.KObj(modifiedPod))
				return framework.Queue, nil
			}
		}
		// This modification of labels doesn't change whether this Pod would match selector or not in any constraints.
		logger.V(5).Info("a scheduled pod's label was updated, but it's a change unrelated to the pod's topology spread constraints",
			"pod", klog.KObj(pod), "modifiedPod", klog.KObj(modifiedPod))
		return framework.QueueSkip, nil
	}

	// Pod is added. Return Queue when the added Pod has a label that matches with topologySpread's selector.
	if modifiedPod != nil {
		if podLabelsMatchSpreadConstraints(constraints, modifiedPod.Labels) {
			logger.V(5).Info("a scheduled pod was created and it matches with the pod's topology spread constraints",
				"pod", klog.KObj(pod), "createdPod", klog.KObj(modifiedPod))
			return framework.Queue, nil
		}
		logger.V(5).Info("a scheduled pod was created, but it doesn't matches with the pod's topology spread constraints",
			"pod", klog.KObj(pod), "createdPod", klog.KObj(modifiedPod))
		return framework.QueueSkip, nil
	}

	// Pod is deleted. Return Queue when the deleted Pod has a label that matches with topologySpread's selector.
	if podLabelsMatchSpreadConstraints(constraints, originalPod.Labels) {
		logger.V(5).Info("a scheduled pod which matches with the pod's topology spread constraints was deleted, and the pod may be schedulable now",
			"pod", klog.KObj(pod), "deletedPod", klog.KObj(originalPod))
		return framework.Queue, nil
	}
	logger.V(5).Info("a scheduled pod was deleted, but it's unrelated to the pod's topology spread constraints",
		"pod", klog.KObj(pod), "deletedPod", klog.KObj(originalPod))

	return framework.QueueSkip, nil
}

// getConstraints extracts topologySpreadConstraint(s) from the Pod spec.
// If the Pod doesn't have any topologySpreadConstraint, it returns default constraints.
func (pl *PodTopologySpread) getConstraints(pod *v1.Pod) ([]topologySpreadConstraint, error) {
	var constraints []topologySpreadConstraint
	var err error
	if len(pod.Spec.TopologySpreadConstraints) > 0 {
		// We have feature gating in APIServer to strip the spec
		// so don't need to re-check feature gate, just check length of Constraints.
		constraints, err = pl.filterTopologySpreadConstraints(
			pod.Spec.TopologySpreadConstraints,
			pod.Labels,
			v1.DoNotSchedule,
		)
		if err != nil {
			return nil, fmt.Errorf("obtaining pod's hard topology spread constraints: %w", err)
		}
	} else {
		constraints, err = pl.buildDefaultConstraints(pod, v1.DoNotSchedule)
		if err != nil {
			return nil, fmt.Errorf("setting default hard topology spread constraints: %w", err)
		}
	}
	return constraints, nil
}

func (pl *PodTopologySpread) isSchedulableAfterNodeChange(logger klog.Logger, pod *v1.Pod, oldObj, newObj interface{}) (framework.QueueingHint, error) {
	originalNode, modifiedNode, err := util.As[*v1.Node](oldObj, newObj)
	if err != nil {
		return framework.Queue, err
	}

	constraints, err := pl.getConstraints(pod)
	if err != nil {
		return framework.Queue, err
	}

	// framework.Add/framework.Update: return Queue when node has topologyKey in its labels, else return QueueSkip.
	//
	// TODO: we can filter out node update events in a more fine-grained way once preCheck is completely removed.
	// See: https://github.com/kubernetes/kubernetes/issues/110175
	if modifiedNode != nil {
		if !nodeLabelsMatchSpreadConstraints(modifiedNode.Labels, constraints) {
			logger.V(5).Info("the created/updated node doesn't match pod topology spread constraints",
				"pod", klog.KObj(pod), "node", klog.KObj(modifiedNode))
			return framework.QueueSkip, nil
		}
		logger.V(5).Info("node that match topology spread constraints was created/updated, and the pod may be schedulable now",
			"pod", klog.KObj(pod), "node", klog.KObj(modifiedNode))
		return framework.Queue, nil
	}

	// framework.Delete: return Queue when node has topologyKey in its labels, else return QueueSkip.
	if !nodeLabelsMatchSpreadConstraints(originalNode.Labels, constraints) {
		logger.V(5).Info("the deleted node doesn't match pod topology spread constraints", "pod", klog.KObj(pod), "node", klog.KObj(originalNode))
		return framework.QueueSkip, nil
	}
	logger.V(5).Info("node that match topology spread constraints was deleted, and the pod may be schedulable now",
		"pod", klog.KObj(pod), "node", klog.KObj(originalNode))
	return framework.Queue, nil
}

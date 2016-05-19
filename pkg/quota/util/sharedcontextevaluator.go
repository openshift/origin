package util

import (
	"fmt"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
)

// SharedContextEvaluator provides an implementation for quota.Evaluator
type SharedContextEvaluator struct {
	*generic.GenericEvaluator
	UsageComputerFactory UsageComputerFactory
}

var _ quota.Evaluator = &SharedContextEvaluator{}

// NewSharedContextEvaluator creates an evaluator object that allows to share context while computing usage of
// single namespace. Context is represented by an object returned by usageComputerFactory and is destroyed
// when the namespace is processed.
func NewSharedContextEvaluator(
	name string,
	groupKind unversioned.GroupKind,
	operationResources map[admission.Operation][]kapi.ResourceName,
	matchedResourceNames []kapi.ResourceName,
	matchesScopeFunc generic.MatchesScopeFunc,
	getFuncByNamespace generic.GetFuncByNamespace,
	listFuncByNamespace generic.ListFuncByNamespace,
	constraintsFunc generic.ConstraintsFunc,
	usageComputerFactory UsageComputerFactory,
) quota.Evaluator {

	rnSet := sets.String{}
	for _, resourceNames := range operationResources {
		rnSet.Insert(quota.ToSet(resourceNames).List()...)
	}

	return &SharedContextEvaluator{
		GenericEvaluator: &generic.GenericEvaluator{
			Name:                       name,
			InternalGroupKind:          groupKind,
			InternalOperationResources: operationResources,
			MatchedResourceNames:       matchedResourceNames,
			MatchesScopeFunc:           matchesScopeFunc,
			GetFuncByNamespace:         getFuncByNamespace,
			ListFuncByNamespace:        listFuncByNamespace,
			ConstraintsFunc:            constraintsFunc,
			UsageFunc: func(object runtime.Object) kapi.ResourceList {
				comp := usageComputerFactory()
				return comp.Usage(object)
			},
		},
		UsageComputerFactory: usageComputerFactory,
	}
}

// UsageComputer knows how to measure usage associated with an object. Its implementation can store arbitrary
// data during `Usage()` run as a context while namespace is being evaluated.
type UsageComputer interface {
	Usage(object runtime.Object) kapi.ResourceList
}

// UsageComputerFactory returns a usage computer used during namespace evaluation.
type UsageComputerFactory func() UsageComputer

// Usage evaluates usage of given object.
func (sce *SharedContextEvaluator) Usage(object runtime.Object) kapi.ResourceList {
	usageComp := sce.UsageComputerFactory()
	return usageComp.Usage(object)
}

// UsageStats calculates latest observed usage stats for all objects. UsageComputerFactory is used to create a
// UsageComputer object whose Usage is called on every object in a namespace.
func (sce *SharedContextEvaluator) UsageStats(options quota.UsageStatsOptions) (quota.UsageStats, error) {
	// default each tracked resource to zero
	result := quota.UsageStats{Used: kapi.ResourceList{}}
	for _, resourceName := range sce.MatchedResourceNames {
		result.Used[resourceName] = resource.MustParse("0")
	}
	list, err := sce.ListFuncByNamespace(options.Namespace, kapi.ListOptions{})
	if err != nil {
		return result, fmt.Errorf("%s: Failed to list %v: %v", sce.Name, sce.GroupKind(), err)
	}
	_, err = meta.Accessor(list)
	if err != nil {
		return result, fmt.Errorf("%s: Unable to understand list result %#v", sce.Name, list)
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return result, fmt.Errorf("%s: Unable to understand list result %#v (%v)", sce.Name, list, err)
	}

	context := sce.UsageComputerFactory()

	for _, item := range items {
		// need to verify that the item matches the set of scopes
		matchesScopes := true
		for _, scope := range options.Scopes {
			if !sce.MatchesScope(scope, item) {
				matchesScopes = false
			}
		}
		// only count usage if there was a match
		if matchesScopes {
			result.Used = quota.Add(result.Used, context.Usage(item))
		}
	}
	return result, nil
}

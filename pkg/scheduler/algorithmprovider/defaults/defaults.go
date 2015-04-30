// This is the default algorithm provider for the Origin scheduler.
package defaults

import (
	kscheduler "github.com/GoogleCloudPlatform/kubernetes/pkg/scheduler"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/algorithmprovider/defaults"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/factory"

	"github.com/openshift/origin/pkg/scheduler"
)

func init() {
	defaultPredicates, err := defaultPredicates()
	if err != nil {
		panic(err)
	}
	defaultPriorities, err := defaultPriorities()
	if err != nil {
		panic(err)
	}
	factory.RegisterAlgorithmProvider(scheduler.DefaultProvider, defaultPredicates, defaultPriorities)

	// Register non-default origin predicates/priorities here
	// factory.RegisterFitPredicateFactory(...)
	// factory.RegisterPriorityConfigFactory(...)
}

func defaultPredicates() (util.StringSet, error) {
	// Fit is determined by project node label selector query.
	matchProjectNodeSelector := "MatchProjectNodeSelector"
	factory.RegisterFitPredicateFactory(
		matchProjectNodeSelector,
		func(args factory.PluginFactoryArgs) kscheduler.FitPredicate {
			return scheduler.NewProjectSelectorMatchPredicate(args.NodeInfo)
		},
	)

	// Get predicates from k8s default provider.
	// If we decide not to use all the predicates from k8s default provider,
	// chery-pick individual predicates from <k8s>/plugin/pkg/scheduler/algorithmprovider/defaults/defaults.go
	kprovider, err := factory.GetAlgorithmProvider(factory.DefaultProvider)
	if err != nil {
		return nil, err
	}

	originDefaultPredicates := kprovider.FitPredicateKeys
	// Add default origin predicates
	originDefaultPredicates.Insert(matchProjectNodeSelector)

	return originDefaultPredicates, nil
}

func defaultPriorities() (util.StringSet, error) {
	// Get priority functions from k8s default provider.
	// If we decide not to use all the priority functions from k8s default provider,
	// chery-pick individual priority function from <k8s>/plugin/pkg/scheduler/algorithmprovider/defaults/defaults.go
	kprovider, err := factory.GetAlgorithmProvider(factory.DefaultProvider)
	if err != nil {
		return nil, err
	}

	OriginDefaultPriorities := kprovider.PriorityFunctionKeys
	// Add default origin priority function keys
	// OriginDefaultPriorities.Insert(...)

	return OriginDefaultPriorities, nil
}

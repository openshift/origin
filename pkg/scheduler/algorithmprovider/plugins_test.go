package algorithmprovider

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/factory"

	"github.com/openshift/origin/pkg/scheduler"
)

var (
	algorithmProviderNames = []string{
		scheduler.DefaultProvider,
	}
)

func TestDefaultConfigExists(t *testing.T) {
	p, err := factory.GetAlgorithmProvider(scheduler.DefaultProvider)
	if err != nil {
		t.Errorf("error retrieving default provider: %v", err)
	}
	if p == nil {
		t.Error("algorithm provider config should not be nil")
	}
	if len(p.FitPredicateKeys) == 0 {
		t.Error("default algorithm provider shouldn't have 0 fit predicates")
	}
}

func TestAlgorithmProviders(t *testing.T) {
	for _, pn := range algorithmProviderNames {
		p, err := factory.GetAlgorithmProvider(pn)
		if err != nil {
			t.Errorf("error retrieving '%s' provider: %v", pn, err)
			break
		}
		if len(p.PriorityFunctionKeys) == 0 {
			t.Errorf("%s algorithm provider shouldn't have 0 priority functions", pn)
		}
		for _, pf := range p.PriorityFunctionKeys.List() {
			if !factory.IsPriorityFunctionRegistered(pf) {
				t.Errorf("priority function %s is not registered but is used in the %s algorithm provider", pf, pn)
			}
		}
		for _, fp := range p.FitPredicateKeys.List() {
			if !factory.IsFitPredicateRegistered(fp) {
				t.Errorf("fit predicate %s is not registered but is used in the %s algorithm provider", fp, pn)
			}
		}
	}
}

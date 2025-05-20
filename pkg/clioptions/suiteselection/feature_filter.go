package suiteselection

import (
	"context"
	"fmt"
	"regexp"

	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type featureGateFilter struct {
	enabled  sets.String
	disabled sets.String
}

func newFeatureGateFilter(ctx context.Context, configClient clientconfigv1.Interface) (*featureGateFilter, error) {
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	desiredVersion := clusterVersion.Status.Desired.Version
	if len(desiredVersion) == 0 && len(clusterVersion.Status.History) > 0 {
		desiredVersion = clusterVersion.Status.History[0].Version
	}

	ret := &featureGateFilter{
		enabled:  sets.NewString(),
		disabled: sets.NewString(),
	}
	found := false
	for _, featureGateValues := range featureGate.Status.FeatureGates {
		if featureGateValues.Version != desiredVersion {
			continue
		}
		found = true
		for _, enabled := range featureGateValues.Enabled {
			ret.enabled.Insert(string(enabled.Name))
		}
		for _, disabled := range featureGateValues.Disabled {
			ret.disabled.Insert(string(disabled.Name))
		}
		break
	}
	if !found {
		return nil, fmt.Errorf("no featuregates found")
	}

	return ret, nil
}

func (f *featureGateFilter) includeTest(name string) bool {
	featureGates := []string{}
	matches := featureGateRegex.FindAllStringSubmatch(name, -1)
	for _, match := range matches {
		if len(match) < 2 {
			panic(fmt.Errorf("regexp match %v is invalid: len(match) < 2 for %v", match, name))
		}
		featureGate := match[1]
		featureGates = append(featureGates, featureGate)
	}

	if f.disabled.HasAny(featureGates...) {
		return false
	}

	// It is important that we always return true if we don't know the status of the gate.
	// This generally means we have no opinion on whether the feature is on or off.
	// We expect the default case to be on, as this is what would happen after a feature is promoted,
	// and the gate is removed.
	return true
}

func includeNonFeatureGateTest(name string) bool {
	return featureGateRegex.FindAllStringSubmatch(name, -1) == nil
}

var (
	featureGateRegex = regexp.MustCompile(`\[OCPFeatureGate:([^]]*)\]`)
)

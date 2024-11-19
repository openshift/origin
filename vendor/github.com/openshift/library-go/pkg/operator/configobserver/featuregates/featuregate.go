package featuregates

import (
	"fmt"
	"slices"

	configv1 "github.com/openshift/api/config/v1"
)

// FeatureGate indicates whether a given feature is enabled or not
// This interface is heavily influenced by k8s.io/component-base, but not exactly compatible.
type FeatureGate interface {
	// Enabled returns true if the key is enabled.
	Enabled(key configv1.FeatureGateName) bool
	// KnownFeatures returns a slice of strings describing the FeatureGate's known features.
	KnownFeatures() []configv1.FeatureGateName
}

type featureGate struct {
	enabled  []configv1.FeatureGateName
	disabled []configv1.FeatureGateName
}

func NewFeatureGate(enabled, disabled []configv1.FeatureGateName) FeatureGate {
	return &featureGate{
		enabled:  enabled,
		disabled: disabled,
	}
}

func (f *featureGate) Enabled(key configv1.FeatureGateName) bool {
	if slices.Contains(f.enabled, key) {
		return true
	}
	if slices.Contains(f.disabled, key) {
		return false
	}

	panic(fmt.Errorf("feature %q is not registered in FeatureGates %v", key, f.KnownFeatures()))
}

func (f *featureGate) KnownFeatures() []configv1.FeatureGateName {
	allKnown := make([]configv1.FeatureGateName, 0, len(f.enabled)+len(f.disabled))
	allKnown = append(allKnown, f.enabled...)
	allKnown = append(allKnown, f.disabled...)

	return allKnown
}

package featuregates

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
)

// NewObserveFeatureFlagsFunc produces a configobserver for feature gates.  If non-nil, the featureWhitelist filters
// feature gates to a known subset (instead of everything).  The featureBlacklist will stop certain features from making
// it through the list.  The featureBlacklist should be empty, but for a brief time, some featuregates may need to skipped.
// @smarterclayton will live forever in shame for being the first to require this for "IPv6DualStack".
func NewObserveFeatureFlagsFunc(featureWhitelist sets.Set[configv1.FeatureGateName], featureBlacklist sets.Set[configv1.FeatureGateName], configPath []string, featureGateAccess FeatureGateAccess) configobserver.ObserveConfigFunc {
	return (&featureFlags{
		allowAll:          len(featureWhitelist) == 0,
		featureWhitelist:  featureWhitelist,
		featureBlacklist:  featureBlacklist,
		configPath:        configPath,
		featureGateAccess: featureGateAccess,
	}).ObserveFeatureFlags
}

type featureFlags struct {
	allowAll         bool
	featureWhitelist sets.Set[configv1.FeatureGateName]
	// we add a forceDisableFeature list because we've now had bad featuregates break individual operators.  Awesome.
	featureBlacklist  sets.Set[configv1.FeatureGateName]
	configPath        []string
	featureGateAccess FeatureGateAccess
}

// ObserveFeatureFlags fills in --feature-flags for the kube-apiserver
func (f *featureFlags) ObserveFeatureFlags(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	prunedExistingConfig := configobserver.Pruned(existingConfig, f.configPath)

	errs := []error{}

	if !f.featureGateAccess.AreInitialFeatureGatesObserved() {
		// if we haven't observed featuregates yet, return the existing
		return prunedExistingConfig, nil
	}

	featureGates, err := f.featureGateAccess.CurrentFeatureGates()
	if err != nil {
		return prunedExistingConfig, append(errs, err)
	}
	observedConfig := map[string]interface{}{}
	newConfigValue := f.getWhitelistedFeatureNames(featureGates)

	currentConfigValue, _, err := unstructured.NestedStringSlice(existingConfig, f.configPath...)
	if err != nil {
		errs = append(errs, err)
		// keep going on read error from existing config
	}
	if !reflect.DeepEqual(currentConfigValue, newConfigValue) {
		recorder.Eventf("ObserveFeatureFlagsUpdated", "Updated %v to %s", strings.Join(f.configPath, "."), strings.Join(newConfigValue, ","))
	}

	if err := unstructured.SetNestedStringSlice(observedConfig, newConfigValue, f.configPath...); err != nil {
		recorder.Warningf("ObserveFeatureFlags", "Failed setting %v: %v", strings.Join(f.configPath, "."), err)
		return prunedExistingConfig, append(errs, err)
	}

	return configobserver.Pruned(observedConfig, f.configPath), errs
}

func (f *featureFlags) getWhitelistedFeatureNames(featureGates FeatureGate) []string {
	newConfigValue := []string{}
	formatEnabledFunc := func(fs configv1.FeatureGateName) string {
		return fmt.Sprintf("%v=true", fs)
	}
	formatDisabledFunc := func(fs configv1.FeatureGateName) string {
		return fmt.Sprintf("%v=false", fs)
	}

	for _, knownFeatureGate := range featureGates.KnownFeatures() {
		if f.featureBlacklist.Has(knownFeatureGate) {
			continue
		}
		// only add whitelisted feature flags
		if !f.allowAll && !f.featureWhitelist.Has(knownFeatureGate) {
			continue
		}

		if featureGates.Enabled(knownFeatureGate) {
			newConfigValue = append(newConfigValue, formatEnabledFunc(knownFeatureGate))
		} else {
			newConfigValue = append(newConfigValue, formatDisabledFunc(knownFeatureGate))
		}
	}

	return newConfigValue
}

func StringsToFeatureGateNames(in []string) []configv1.FeatureGateName {
	out := []configv1.FeatureGateName{}
	for _, curr := range in {
		out = append(out, configv1.FeatureGateName(curr))
	}

	return out
}

func FeatureGateNamesToStrings(in []configv1.FeatureGateName) []string {
	out := []string{}
	for _, curr := range in {
		out = append(out, string(curr))
	}

	return out
}

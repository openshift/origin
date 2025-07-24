package filters

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/test/extensions"
)

// ClusterStateFilter filters tests based on cluster environment
type ClusterStateFilter struct {
	config *clusterdiscovery.ClusterConfiguration
}

func NewClusterStateFilter(config *clusterdiscovery.ClusterConfiguration) *ClusterStateFilter {
	return &ClusterStateFilter{config: config}
}

func (f *ClusterStateFilter) Name() string {
	return "cluster-state"
}

func (f *ClusterStateFilter) Filter(_ context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	// Generate skip list once for all tests
	skips := f.generateSkips()

	filtered := make(extensions.ExtensionTestSpecs, 0, len(tests))
	for _, test := range tests {
		if f.matchTest(test.Name, skips) {
			filtered = append(filtered, test)
		}
	}
	return filtered, nil
}

func (f *ClusterStateFilter) ShouldApply() bool {
	switch {
	case f.config == nil:
		logrus.Debug("Cluster state filter disabled: cluster config is nil")
		return false
	case f.config.ProviderName == "skeleton":
		logrus.Info("Cluster state filter disabled: provider is 'skeleton' (used for dry-run)")
		return false
	default:
		return true
	}
}

// Regular expressions for parsing test labels
var (
	apiGroupRegex    = regexp.MustCompile(`\[apigroup:([^]]*)\]`)
	featureGateRegex = regexp.MustCompile(`\[OCPFeatureGate:([^]]*)\]`)
)

// generateSkips creates the list of skip patterns based on cluster configuration
func (f *ClusterStateFilter) generateSkips() []string {
	skips := []string{fmt.Sprintf("[Skipped:%s]", f.config.ProviderName)}

	if f.config.IsIBMROKS {
		skips = append(skips, "[Skipped:ibmroks]")
	}
	if f.config.NetworkPlugin != "" {
		skips = append(skips, fmt.Sprintf("[Skipped:Network/%s]", f.config.NetworkPlugin))
		if f.config.NetworkPluginMode != "" {
			skips = append(skips, fmt.Sprintf("[Skipped:Network/%s/%s]", f.config.NetworkPlugin, f.config.NetworkPluginMode))
		}
	}

	if f.config.Disconnected {
		skips = append(skips, "[Skipped:Disconnected]")
	}

	if f.config.IsProxied {
		skips = append(skips, "[Skipped:Proxy]")
	}

	if f.config.SingleReplicaTopology {
		skips = append(skips, "[Skipped:SingleReplicaTopology]")
	}

	if !f.config.HasIPv4 {
		skips = append(skips, "[Feature:Networking-IPv4]")
	}
	if !f.config.HasIPv6 {
		skips = append(skips, "[Feature:Networking-IPv6]")
	}
	if !f.config.HasIPv4 || !f.config.HasIPv6 {
		// lack of "]" is intentional; this matches multiple tags
		skips = append(skips, "[Feature:IPv6DualStack")
	}

	if !f.config.HasSCTP {
		skips = append(skips, "[Feature:SCTPConnectivity]")
	}

	if f.config.HasNoOptionalCapabilities {
		skips = append(skips, "[Skipped:NoOptionalCapabilities]")
	}

	return skips
}

// matchTest implements the cluster-based test matching logic with pre-generated skips
func (f *ClusterStateFilter) matchTest(name string, skips []string) bool {
	// Check skip conditions
	for _, skip := range skips {
		if strings.Contains(name, skip) {
			logrus.WithField("test", name).WithField("skip", skip).Debug("Skipping test")
			return false
		}
	}

	// Check API groups
	requiredAPIGroups := []string{}
	matches := apiGroupRegex.FindAllStringSubmatch(name, -1)
	for _, match := range matches {
		if len(match) < 2 {
			panic(fmt.Errorf("regexp match %v is invalid: len(match) < 2 for %v", match, name))
		}
		apigroup := match[1]
		requiredAPIGroups = append(requiredAPIGroups, apigroup)
	}

	if len(requiredAPIGroups) > 0 && (f.config.APIGroups == nil || !f.config.APIGroups.HasAll(requiredAPIGroups...)) {
		available := "none"
		if f.config.APIGroups != nil {
			available = strings.Join(f.config.APIGroups.UnsortedList(), ",")
		}

		logrus.WithField("test", name).
			WithField("requiredAPIGroups", requiredAPIGroups).
			WithField("availableGroups", available).
			Debug("Skipping test")
		return false
	}

	// Apply feature gate filtering - keep this last
	featureGates := []string{}
	matches = featureGateRegex.FindAllStringSubmatch(name, -1)
	for _, match := range matches {
		if len(match) < 2 {
			panic(fmt.Errorf("regexp match %v is invalid: len(match) < 2 for %v", match, name))
		}
		featureGate := match[1]
		featureGates = append(featureGates, featureGate)
	}

	if len(featureGates) == 0 {
		return true
	}

	// If any of the required feature gates are disabled, skip the test
	if f.config.DisabledFeatureGates != nil && f.config.DisabledFeatureGates.HasAny(featureGates...) {
		logrus.WithField("test", name).
			WithField("disabledFeatureGates", f.config.DisabledFeatureGates.UnsortedList()).
			WithField("requiredFeatureGates", featureGates).
			Debug("Skipping test")
		return false
	}

	// It is important that we always return true if we don't know the status of the gate.
	// This generally means we have no opinion on whether the feature is on or off.
	// We expect the default case to be on, as this is what would happen after a feature is promoted,
	// and the gate is removed.
	//
	// Therefore, if there are any feature gates defined at all in the cluster, we should
	// run the tests as long as none of the required gates are explicitly disabled.
	// If there are no feature gates at all in the cluster, we should not run feature gate tests.
	hasAnyFeatureGates := (f.config.EnabledFeatureGates != nil && f.config.EnabledFeatureGates.Len() > 0) ||
		(f.config.DisabledFeatureGates != nil && f.config.DisabledFeatureGates.Len() > 0)
	return hasAnyFeatureGates
}

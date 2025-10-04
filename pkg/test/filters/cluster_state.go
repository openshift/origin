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
	skips  []string
}

func NewClusterStateFilter(config *clusterdiscovery.ClusterConfiguration) *ClusterStateFilter {
	if config == nil {
		logrus.Warn("Cluster state filter is disabled, cluster config is nil")
		return &ClusterStateFilter{}
	}

	skips := []string{fmt.Sprintf("[Skipped:%s]", config.ProviderName)}

	if config.IsIBMROKS {
		skips = append(skips, "[Skipped:ibmroks]")
	}
	if config.NetworkPlugin != "" {
		skips = append(skips, fmt.Sprintf("[Skipped:Network/%s]", config.NetworkPlugin))
		if config.NetworkPluginMode != "" {
			skips = append(skips, fmt.Sprintf("[Skipped:Network/%s/%s]", config.NetworkPlugin, config.NetworkPluginMode))
		}
	}

	if config.Disconnected {
		skips = append(skips, "[Skipped:Disconnected]")
	}

	if config.IsProxied {
		skips = append(skips, "[Skipped:Proxy]")
	}

	if config.SingleReplicaTopology {
		skips = append(skips, "[Skipped:SingleReplicaTopology]")
	}

	if !config.HasIPv4 {
		skips = append(skips, "[Feature:Networking-IPv4]")
	}
	if !config.HasIPv6 {
		skips = append(skips, "[Feature:Networking-IPv6]")
	}
	if !config.HasIPv4 || !config.HasIPv6 {
		// lack of "]" is intentional; this matches multiple tags
		skips = append(skips, "[Feature:IPv6DualStack")
	}

	if !config.HasSCTP {
		skips = append(skips, "[Feature:SCTPConnectivity]")
	}

	if config.HasNoOptionalCapabilities {
		skips = append(skips, "[Skipped:NoOptionalCapabilities]")
	}

	if config.HypervisorConfig == nil {
		skips = append(skips, "[Requires:HypervisorSSHConfig]")
	}

	logrus.WithField("skips", skips).Info("Generated skips for cluster state")

	return &ClusterStateFilter{
		config: config,
		skips:  skips,
	}
}

func (f *ClusterStateFilter) Name() string {
	return "cluster-state"
}

func (f *ClusterStateFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if f.config == nil {
		return tests, nil
	}

	filtered := make(extensions.ExtensionTestSpecs, 0, len(tests))
	for _, test := range tests {
		if f.matchTest(test.Name) {
			filtered = append(filtered, test)
		}
	}
	return filtered, nil
}

func (f *ClusterStateFilter) ShouldApply() bool {
	return f.config != nil
}

// Regular expressions for parsing test labels
var (
	apiGroupRegex    = regexp.MustCompile(`\[apigroup:([^]]*)\]`)
	featureGateRegex = regexp.MustCompile(`\[OCPFeatureGate:([^]]*)\]`)
)

// matchTest implements the cluster-based test matching logic
func (f *ClusterStateFilter) matchTest(name string) bool {
	// Check skip conditions
	for _, skip := range f.skips {
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

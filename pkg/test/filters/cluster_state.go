package filters

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

// matchTest implements the cluster-based test matching logic
func (f *ClusterStateFilter) matchTest(name string) bool {
	var skips []string
	skips = append(skips, fmt.Sprintf("[Skipped:%s]", f.config.ProviderName))

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

	// Check skip conditions
	for _, skip := range skips {
		if strings.Contains(name, skip) {
			return false
		}
	}

	// Apply API group filtering
	if f.config.APIGroups != nil {
		requiredGroups := []string{}
		matches := apiGroupRegex.FindAllStringSubmatch(name, -1)
		for _, match := range matches {
			if len(match) < 2 {
				panic(fmt.Errorf("regexp match %v is invalid: len(match) < 2 for %v", match, name))
			}
			apigroup := match[1]
			requiredGroups = append(requiredGroups, apigroup)
		}
		if !f.config.APIGroups.HasAll(requiredGroups...) {
			return false
		}
	}

	// Apply feature gate filtering
	featureGates := []string{}
	matches := featureGateRegex.FindAllStringSubmatch(name, -1)
	for _, match := range matches {
		if len(match) < 2 {
			panic(fmt.Errorf("regexp match %v is invalid: len(match) < 2 for %v", match, name))
		}
		featureGate := match[1]
		featureGates = append(featureGates, featureGate)
	}

	if f.config.DisabledFeatureGates != nil && f.config.DisabledFeatureGates.HasAny(featureGates...) {
		return false
	}

	return f.config.EnabledFeatureGates != nil && f.config.EnabledFeatureGates.HasAll(featureGates...)
}

// Regular expressions for parsing test labels
var (
	apiGroupRegex    = regexp.MustCompile(`\[apigroup:([^]]*)\]`)
	featureGateRegex = regexp.MustCompile(`\[OCPFeatureGate:([^]]*)\]`)
)

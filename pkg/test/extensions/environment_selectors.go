package extensions

import (
	"strings"

	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	extensionflags "github.com/openshift-eng/openshift-tests-extension/pkg/flags"
	"github.com/openshift-eng/openshift-tests-extension/pkg/util/sets"
	configv1 "github.com/openshift/api/config/v1"
)

const rosaClusterTypeTag = "red-hat-clustertype"

// IsROSACluster returns true when the infrastructure platform status contains
// the product tag shared by ROSA Classic and ROSA HCP clusters.
func IsROSACluster(platformStatus *configv1.PlatformStatus) bool {
	if platformStatus == nil || platformStatus.AWS == nil {
		return false
	}

	for _, tag := range platformStatus.AWS.ResourceTags {
		if tag.Key == rosaClusterTypeTag && strings.EqualFold(tag.Value, "rosa") {
			return true
		}
	}
	return false
}

// Each exclusion is a set of literal name fragments. Multiple fragments are
// equivalent to placing a wildcard between them in the existing job skip
// expressions, without requiring regex escaping.
var rosaExclusions = [][]string{
	{"users can manipulate groups"},
	{"well-known endpoint should be reachable"},
	{"builds installing subscription content"},
	{"The HAProxy router should expose prometheus metrics for a route"},
	{"oc adm must-gather runs successfully for audit logs"},
	{"OAuth server has the correct token and certificate fallback semantics"},
	{"OAuthClientWithRedirectURIs must validate request URIs according to oauth-client definition"},
	{"should be present on all masters and work"},
	{"when installed on the cluster should have a AlertmanagerReceiversNotConfigured alert in firing state"},
	{"Node Lifecycle should run through the lifecycle of a node"},
	{"NetworkPolicy between server and client should allow ingress access from updated pod"},
	{"Managed cluster should set requests but not limits"},
	{"Managed cluster should ensure platform components have system-* priority class associated"},
	{"cloud-provider-aws-e2e", "loadbalancer NLB internal should be reachable with hairpinning traffic"},
	{"cloud-provider-aws-e2e", "loadbalancer NLB should be reachable with target-node-labels"},
	{"blocks manual creation of EndpointSlices pointing to the cluster or service network"},
	{"blocks manual creation of Endpoints pointing to the cluster or service network"},
	{"Netpol NetworkPolicy between server and client should deny egress from all pods in a namespace"},
	{"Netpol NetworkPolicy between server and client should deny egress from pods based on PodSelector"},
}

var rosaClassicExclusions = [][]string{
	{"CSI Mock volume expansion Expansion with recovery should allow recovery if controller expansion fails with infeasible error"},
	{"CPU Partitioning cluster infrastructure should be configured correctly"},
	{"sig-network-edge", "GatewayAPIController"},
	{"sig-olmv1", "NewOLM", "Catalog should serve FBC"},
	{"cloud-provider-aws-e2e-openshift", "AWSServiceLBNetworkSecurityGroup"},
	{"cloud-provider-aws-e2e", "loadbalancer CLB internal should be reachable with hairpinning traffic"},
	{"sig-auth", "SCC", "should not have pod creation failures during install"},
	{"sig-imageregistry", "should redirect on blob pull"},
	{"CSI Mock volume expansion", "should record target size in allocated resources"},
	{"Conntrack proxy implementation should not be vulnerable to the invalid conntrack state bug"},
	{"CSRs from machines that are not recognized by the cloud provider are not approved"},
	{"InPlacePodVerticalScaling", "pod-resize"},
	{"Feature:NetworkSegmentation"},
	{"OCPFeatureGate:NetworkSegmentation"},
}

var rosaHCPExclusions = [][]string{
	{"etcd leader changes are not excessive"},
	{"ocp payload should be based on existing source OLM version should contain the source commit id"},
	{"OLM should have imagePullPolicy:IfNotPresent on thier deployments"},
	{"check registry.redhat.io is available and samples operator can import sample imagestreams"},
	{"NetworkPolicy between server and client should deny ingress access to updated pod"},
	{"NetworkPolicy between server and client should stop enforcing policies after they are deleted"},
	{"In-tree Volumes", "Driver: local", "subPath should support"},
	{"DRA", "kubelet", "DynamicResourceAllocation"},
	{"[FeatureGate:DRAExtendedResource]"},
	{"decrease memory limit below usage"},
	{"CSI Mock selinux on mount SELinuxMount"},
	{"CSI Mock volume expansion Expansion with recovery"},
	{"MutableCSINodeAllocatableCount", "Attach Limit Exceeded should transition pod to failed state"},
	{"PersistentVolumes-local", "Two pods mounting"},
	{"PersistentVolumes-local", "One pod requesting one prebound PVC"},
	{"Services should fallback to local terminating endpoints", "externalTrafficPolicy=Local"},
	{"Services should fallback to terminating endpoints when there are no ready endpoints with externallTrafficPolicy=Cluster"},
	{"Deployment should not disrupt a cloud load-balancer", "connectivity during rollout"},
	{"Load balancer healthcheck port and path should be 10256/healthz"},
	{"GatewayAPIController", "Ensure HTTPRoute object is created"},
	{"GatewayAPIController", "Ensure LB", "service", "and dnsRecord are created for a Gateway object"},
	{"AWSServiceLBNetworkSecurityGroup"},
	{"cloud-provider-aws-e2e", "BYO", "Security Group"},
	{"cloud-provider-aws-e2e", "transition from", "SG"},
	{"InPlace Resize Container"},
	{"CSI", "csi-hostpath", "provisioning"},
	{"CSI", "csi-hostpath-groupsnapshot", "VolumeGroupSnapshot", "Ephemeral-volume", "ephemeral"},
	{"should support seccomp runtime/default"},
	{"PersistentVolumes-local", "block"},
	{"Pre-provisioned PV", "ntfs"},
	{"Probe configuration", "OTP", "terminationGracePeriodSeconds", "OCP-44493"},
	{"Netpol", "NetworkPolicy between server and client", "should support a 'default-deny-ingress' policy"},
	{"Netpol", "NetworkPolicy between server and client", "should enforce ingress policy allowing any port traffic to a server on a specific protocol"},
}

func filterByROSA(specs et.ExtensionTestSpecs) {
	addROSAExclusions(specs, rosaExclusions, et.FactEquals("product", "ROSA"), "[Skipped:ROSA]")
	addROSAExclusions(specs, rosaClassicExclusions, et.And(
		et.FactEquals("product", "ROSA"),
		et.TopologyEquals("HighlyAvailable"),
	), "[Skipped:ROSA-Classic]")
	addROSAExclusions(specs, rosaHCPExclusions, et.And(
		et.FactEquals("product", "ROSA"),
		et.TopologyEquals("External"),
	), "[Skipped:ROSA-HCP]")
}

func addROSAExclusions(specs et.ExtensionTestSpecs, exclusions [][]string, environmentSelector, label string) {
	selectFunctions := make([]et.SelectFunction, 0, len(exclusions))
	for _, exclusion := range exclusions {
		selectFunctions = append(selectFunctions, et.NameContainsAll(exclusion...))
	}
	selected := specs.SelectAny(selectFunctions)
	for _, spec := range selected {
		if spec.Labels == nil {
			spec.Labels = sets.New[string]()
		}
	}
	selected.
		Exclude(environmentSelector).
		AddLabel(label)
}

// filterDiscoveredSpecsByROSA applies Origin's ROSA exclusions to the combined
// inventory, including specs supplied by external test binaries.
func filterDiscoveredSpecsByROSA(specs ExtensionTestSpecs, envFlags EnvironmentFlags) (ExtensionTestSpecs, error) {
	baseSpecs := make(et.ExtensionTestSpecs, 0, len(specs))
	wrappedByBase := make(map[*et.ExtensionTestSpec]*ExtensionTestSpec, len(specs))
	for _, spec := range specs {
		baseSpecs = append(baseSpecs, spec.ExtensionTestSpec)
		wrappedByBase[spec.ExtensionTestSpec] = spec
	}

	unprocessedSpecs := baseSpecs.Select(func(spec *et.ExtensionTestSpec) bool {
		return !spec.Labels.Has("[Skipped:ROSA]") &&
			!spec.Labels.Has("[Skipped:ROSA-Classic]") &&
			!spec.Labels.Has("[Skipped:ROSA-HCP]")
	})
	filterByROSA(unprocessedSpecs)
	filtered, err := baseSpecs.FilterByEnvironment(toEnvironmentalFlags(envFlags))
	if err != nil {
		return nil, err
	}

	ret := make(ExtensionTestSpecs, 0, len(filtered))
	for _, spec := range filtered {
		ret = append(ret, wrappedByBase[spec])
	}
	return ret, nil
}

func toEnvironmentalFlags(envFlags EnvironmentFlags) extensionflags.EnvironmentalFlags {
	ret := extensionflags.EnvironmentalFlags{Facts: map[string]string{}}
	for _, flag := range envFlags {
		switch flag.Name {
		case apiGroup:
			ret.APIGroups = append(ret.APIGroups, flag.Value)
		case architecture:
			ret.Architecture = flag.Value
		case externalConnectivity:
			ret.ExternalConnectivity = flag.Value
		case fact:
			if key, value, ok := strings.Cut(flag.Value, "="); ok {
				ret.Facts[key] = value
			}
		case featureGate:
			ret.FeatureGates = append(ret.FeatureGates, flag.Value)
		case network:
			ret.Network = flag.Value
		case networkStack:
			ret.NetworkStack = flag.Value
		case optionalCapability:
			ret.OptionalCapabilities = append(ret.OptionalCapabilities, flag.Value)
		case platform:
			ret.Platform = flag.Value
		case topology:
			ret.Topology = flag.Value
		case upgrade:
			ret.Upgrade = flag.Value
		case version:
			ret.Version = flag.Value
		}
	}
	return ret
}

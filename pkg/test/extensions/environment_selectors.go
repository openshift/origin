package extensions

import (
	"strings"

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

// filterDiscoveredSpecsByROSA applies Origin's ROSA exclusions after the
// inventories from all extension binaries have been combined.
func filterDiscoveredSpecsByROSA(specs ExtensionTestSpecs, envFlags EnvironmentFlags) ExtensionTestSpecs {
	filtered := make(ExtensionTestSpecs, 0, len(specs))
	for _, spec := range specs {
		if !IsTestExcludedByROSA(spec.Name, envFlags) {
			filtered = append(filtered, spec)
		}
	}
	return filtered
}

// IsTestExcludedByROSA reports whether a test name matches the exclusions for
// the ROSA product and topology described by envFlags.
func IsTestExcludedByROSA(name string, envFlags EnvironmentFlags) bool {
	product := ""
	topologyValue := ""
	for _, flag := range envFlags {
		switch flag.Name {
		case fact:
			if key, value, ok := strings.Cut(flag.Value, "="); ok && key == "product" {
				product = value
			}
		case topology:
			topologyValue = flag.Value
		}
	}
	if product != "ROSA" {
		return false
	}

	exclusions := make([][]string, 0, len(rosaExclusions)+len(rosaClassicExclusions)+len(rosaHCPExclusions))
	exclusions = append(exclusions, rosaExclusions...)
	switch topologyValue {
	case "HighlyAvailable":
		exclusions = append(exclusions, rosaClassicExclusions...)
	case "External":
		exclusions = append(exclusions, rosaHCPExclusions...)
	}

	return matchesAnyROSAExclusion(name, exclusions)
}

func matchesAnyROSAExclusion(name string, exclusions [][]string) bool {
	for _, exclusion := range exclusions {
		matches := true
		for _, fragment := range exclusion {
			if !strings.Contains(name, fragment) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

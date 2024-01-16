package names

import "k8s.io/apimachinery/pkg/types"

// some names

// OperatorConfig is the name of the CRD that defines the complete
// operator configuration
const OPERATOR_CONFIG = "cluster"

// CLUSTER_CONFIG is the name of the higher-level cluster configuration
// and status object.
const CLUSTER_CONFIG = "cluster"

// PROXY_CONFIG is the name of the default proxy object.
const PROXY_CONFIG = "cluster"

// INFRASTRUCTURE_CONFIG is the name of the default infrastructure object.
const INFRASTRUCTURE_CONFIG = "cluster"

// APPLIED_PREFIX is the prefix applied to the config maps
// where we store previously applied configuration
const APPLIED_PREFIX = "applied-"

// APPLIED_NAMESPACE is the namespace where applied configuration
// configmaps are stored.
// Should match 00_namespace.yaml
const APPLIED_NAMESPACE = "openshift-network-operator"

// MULTUS_NAMESPACE is the namespace where applied configuration
// configmaps are stored.
// Should match 00_namespace.yaml
const MULTUS_NAMESPACE = "openshift-multus"

// ALLOWLIST_CONFIG_NAME is the name of the allowlist ConfigMap
const ALLOWLIST_CONFIG_NAME = "cni-sysctl-allowlist"

// IgnoreObjectErrorAnnotation is an annotation we can set on objects
// to signal to the reconciler that we don't care if they fail to create
// or update. Useful when we want to make a CR for which the CRD may not exist yet.
const IgnoreObjectErrorAnnotation = "networkoperator.openshift.io/ignore-errors"

// CreateOnlyAnnotation is an annotation on all objects that
// tells the CNO reconciliation engine to ignore this object if it already exists.
const CreateOnlyAnnotation = "networkoperator.openshift.io/create-only"

// CreateWaitAnnotation is an annotation on all objects that
// tells the CNO reconciliation engine to ignore creating this object until conditions are met.
const CreateWaitAnnotation = "networkoperator.openshift.io/create-wait"

// NonCriticalAnnotation is an annotation on Deployments/DaemonSets to indicate
// that they are not critical to the functioning of the pod network
const NonCriticalAnnotation = "networkoperator.openshift.io/non-critical"

// GenerateStatusLabel can be set by the various Controllers to tell the
// StatusController that this object is relevant, and should be included
// when generating status from deployed pods.
// Currently, this is looked for on Deployments, DaemonSets, and StatefulSets.
// Its value reflects which cluster the resource belongs to. This helps avoid an overlap
// in Hypershift where there can be multiple CNO instances running in the management cluster.
const GenerateStatusLabel = "networkoperator.openshift.io/generates-operator-status"

// StandAloneClusterName is a value used for GenerateStatusLabel label when running in non-Hypershift environments
const StandAloneClusterName = "stand-alone"

// NetworkMigrationAnnotation is an annotation on the networks.operator.openshift.io CR to indicate
// that executing network migration (switching the default network type of the cluster) is allowed.
const NetworkMigrationAnnotation = "networkoperator.openshift.io/network-migration"

// NetworkIPFamilyAnnotation is an annotation on the OVN networks.operator.openshift.io daemonsets
// to indicate the current IP Family mode of the cluster: "single-stack" or "dual-stack"
const NetworkIPFamilyModeAnnotation = "networkoperator.openshift.io/ip-family-mode"

// ClusterNetworkCIDRsAnnotation is an annotation on the OVN networks.operator.openshift.io daemonsets
// to indicate the current list of clusterNetwork CIDRs available to the cluster.
const ClusterNetworkCIDRsAnnotation = "networkoperator.openshift.io/cluster-network-cidr"

// NetworkHybridOverlayAnnotatiion is an annotation on the OVN networks.operator.io.daemonsets
// to indicate the current state of of the Hybrid overlay on the cluster: "enabled" or "disabled"
const NetworkHybridOverlayAnnotation = "networkoperator.openshift.io/hybrid-overlay-status"

// IPsecEnableAnnotation is an annotation on the OVN networks.operator.openshift.io
// daemonsets to indicate if ipsec is enabled for the OVN networks.
const IPsecEnableAnnotation = "networkoperator.openshift.io/ipsec-enabled"

// RolloutHungAnnotation is set to "" if it is detected that a rollout
// (i.e. DaemonSet or Deployment) is not making progress, unset otherwise.
const RolloutHungAnnotation = "networkoperator.openshift.io/rollout-hung"

// CopyFromAnnotation is an annotation that allows copying resources from specified clusters
// value format: cluster/namespace/name
const CopyFromAnnotation = "network.operator.openshift.io/copy-from"

// ClusterNameAnnotation is an annotation that specifies the cluster an object belongs to
const ClusterNameAnnotation = "network.operator.openshift.io/cluster-name"

// RelatedClusterObjectsAnnotation is an annotation that allows deleting resources for specified clusters
// value format: cluster/group/resource/namespace/name
const RelatedClusterObjectsAnnotation = "network.operator.openshift.io/relatedClusterObjects"

// MULTUS_VALIDATING_WEBHOOK is the name of the ValidatingWebhookConfiguration for multus-admission-controller
// that is used in multus admission controller deployment
const MULTUS_VALIDATING_WEBHOOK = "multus.openshift.io"

// ADDL_TRUST_BUNDLE_CONFIGMAP_NS is the namespace for one or more
// ConfigMaps that contain user provided trusted CA bundles.
const ADDL_TRUST_BUNDLE_CONFIGMAP_NS = "openshift-config"

// TRUSTED_CA_BUNDLE_CONFIGMAP_KEY is the name of the data key containing
// the PEM encoded trust bundle.
const TRUSTED_CA_BUNDLE_CONFIGMAP_KEY = "ca-bundle.crt"

// TRUSTED_CA_BUNDLE_CONFIGMAP is the name of the ConfigMap
// containing the combined user/system trust bundle.
const TRUSTED_CA_BUNDLE_CONFIGMAP = "trusted-ca-bundle"

// TRUSTED_CA_BUNDLE_CONFIGMAP_NS is the namespace that hosts the
// ADDL_TRUST_BUNDLE_CONFIGMAP and TRUST_BUNDLE_CONFIGMAP
// ConfigMaps.
const TRUSTED_CA_BUNDLE_CONFIGMAP_NS = "openshift-config-managed"

// TRUSTED_CA_BUNDLE_CONFIGMAP_LABEL is the name of the label that
// determines whether or not to inject the combined ca certificate
const TRUSTED_CA_BUNDLE_CONFIGMAP_LABEL = "config.openshift.io/inject-trusted-cabundle"

// SYSTEM_TRUST_BUNDLE is the full path to the file containing
// the system trust bundle.
const SYSTEM_TRUST_BUNDLE = "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"

// OpenShiftComponent mirrors https://github.com/openshift/api/blob/master/annotations/annotations.go#L33 but a zero-diff
// tidy and vendor result in a non-building project, so working from a copy here until the next dep update.
const OpenShiftComponent = "openshift.io/owning-component"

// ClusterNetworkOperatorJiraComponent is the jira component name for the cluster-network-operator
const ClusterNetworkOperatorJiraComponent = "Networking / cluster-network-operator"

// NetworkTypeMigrationAnnotation is an annotation on the OVN networks.operator.openshift.io CR to indicate
// that executing network type live migration
const NetworkTypeMigrationAnnotation = "network.openshift.io/network-type-migration"

// MachineConfigPoolsUpdating is the reason string NetworkTypeMigrationTargetCNIInUse and NetworkTypeMigrationMTUReady
// conditions to indicate if MCP is updating
const MachineConfigPoolsUpdating string = "MachineConfigPoolsUpdating"

// Status condition types of network.config for live migration
const (
	// NetworkTypeMigrationInProgress is the condition type for network type live migration to indicate if the migration
	// is in progress
	NetworkTypeMigrationInProgress string = "NetworkTypeMigrationInProgress"
	// NetworkTypeMigrationTargetCNIAvailable is the condition type for network type live migration to indicate if
	// target CNI is available
	NetworkTypeMigrationTargetCNIAvailable string = "NetworkTypeMigrationTargetCNIAvailable"
	// NetworkTypeMigrationTargetCNIInUse is the condition type for network type live migration to indicate if the
	// target CNI in use
	NetworkTypeMigrationTargetCNIInUse string = "NetworkTypeMigrationTargetCNIInUse"
	// NetworkTypeMigrationOriginalCNIPurged is the condition type for network type live migration to indicate if the
	// original CNI has been purged
	NetworkTypeMigrationOriginalCNIPurged string = "NetworkTypeMigrationOriginalCNIPurged"
	// NetworkTypeMigrationMTUReady is the condition type for network type live migration to indicate if the routable
	// MTU is set
	NetworkTypeMigrationMTUReady string = "NetworkTypeMigrationMTUReady"
)

// Proxy returns the namespaced name "cluster" in the
// default namespace.
func Proxy() types.NamespacedName {
	return types.NamespacedName{
		Name: PROXY_CONFIG,
	}
}

// TrustedCABundleConfigMap returns the namespaced name of the ConfigMap
// openshift-config-managed/trusted-ca-bundle trust bundle.
func TrustedCABundleConfigMap() types.NamespacedName {
	return types.NamespacedName{
		Namespace: TRUSTED_CA_BUNDLE_CONFIGMAP_NS,
		Name:      TRUSTED_CA_BUNDLE_CONFIGMAP,
	}
}

// constants for namespace and custom resource names
// namespace in which ingress controller objects are created
const IngressControllerNamespace = "openshift-ingress-operator"

// namespace representing host network traffic
// this is also the namespace where to set the ingress label
const HostNetworkNamespace = "openshift-host-network"

// label for ingress policy group
const PolicyGroupLabelIngress = "policy-group.network.openshift.io/ingress"

// legacy label for ingress policy group
const PolicyGroupLabelLegacy = "network.openshift.io/policy-group"

// we use empty label values for policy groups
const PolicyGroupLabelIngressValue = ""

// value for legacy policy group label
const PolicyGroupLabelLegacyValue = "ingress"

// default ingress controller name
const DefaultIngressControllerName = "default"

// single stack IP family mode
const IPFamilySingleStack = "single-stack"

// dual stack IP family mode
const IPFamilyDualStack = "dual-stack"

// EnvApiOverrideHost is an environment variable that, if set, allows overriding the host / port
// of the apiserver, but only for rendered manifests. CNO itself will not use it
const EnvApiOverrideHost = "APISERVER_OVERRIDE_HOST"
const EnvApiOverridePort = "APISERVER_OVERRIDE_PORT"

// ManagementClusterName provides the name of the management cluster, for use with Hypershift.
const ManagementClusterName = "management"

// DefaultClusterName provides the name of the default cluster, for use with Hypershift (or non-Hypershift)
const DefaultClusterName = "default"

// DashboardNamespace is the namespace where dashboards are created
const DashboardNamespace = "openshift-config-managed"

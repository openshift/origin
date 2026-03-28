// TNF node replacement: shared configuration structs.
package two_node

import (
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
)

// staticPodOperatorInstallerNamespaces lists namespaces where control-plane static-pod operators run app=installer Pods.
// Completed installer pods for a deleted node can block new installs when the replacement Node reuses that name.
var staticPodOperatorInstallerNamespaces = []string{
	"openshift-kube-apiserver",
	"openshift-kube-controller-manager",
	"openshift-kube-scheduler",
	services.EtcdNamespace,
}

// HypervisorConnection contains SSH connection details for the hypervisor.
type HypervisorConnection struct {
	Config         core.SSHConfig
	KnownHostsPath string
}

// NodeInfo contains information about a cluster node
type NodeInfo struct {
	Name           string
	IP             string
	VMName         string // Hypervisor VM name
	MachineName    string // OpenShift Machine name
	MachineHash    string // Machine name hash component
	BMCSecretName  string // BMC secret name
	BMHName        string // BareMetalHost name
	MAC            string // MAC address
	KnownHostsPath string // SSH known_hosts file path
}

// EtcdResources contains etcd-related Kubernetes resource names
type EtcdResources struct {
	PeerSecretName           string
	ServingSecretName        string
	ServingMetricsSecretName string
}

// JobTracking contains test job names
type JobTracking struct {
	AuthJobName                string
	AfterSetupJobName          string
	UpdateSetupJobTargetName   string
	UpdateSetupJobSurvivorName string
}

// TestExecution tracks test state and configuration
type TestExecution struct {
	SetupCompleted               bool   // True if BeforeEach completed successfully
	GlobalBackupDir              string // Directory containing backup YAMLs
	HasAttemptedNodeProvisioning bool
	BackupUsedForRecovery        bool   // Set to true if recovery used the backup
	RedfishIP                    string // Gateway IP for BMC access
	// PreReplacementChassisID / PreReplacementNodeUID: captured immediately before deleting the
	// replacement target Node; used to compare whether k8s.ovn.org/node-chassis-id reappears on
	// the new Node object (same name) during BMH provisioning / MCS boots.
	PreReplacementChassisID string
	PreReplacementNodeUID   string
	// PreReplacementTargetOVSSystemID is the target node's host OVS external_ids:system-id (SSH) before VM destroy.
	// OVN-K publishes k8s.ovn.org/node-chassis-id from this value; it is not read from the peer's OVS.
	PreReplacementTargetOVSSystemID string
	// SurvivorLibvirtDiskPaths is the set of hypervisor file paths backing the surviving control-plane VM's
	// disks (captured at setup via dumpxml + vol-path). recreateTargetVM / recovery refuse to move or
	// replace any path in this set so a bad target XML or vol-path resolution cannot wipe the survivor's qcow2.
	SurvivorLibvirtDiskPaths map[string]struct{}
}

// TNFTestConfig contains all configuration for two-node test execution
type TNFTestConfig struct {
	Hypervisor    HypervisorConnection
	TargetNode    NodeInfo
	SurvivingNode NodeInfo
	EtcdResources EtcdResources
	Jobs          JobTracking
	Execution     TestExecution
}

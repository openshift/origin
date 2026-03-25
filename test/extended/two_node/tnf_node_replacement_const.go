// TNF node replacement: namespaces, resource names, and phase-oriented timeouts.
package two_node

import "time"

// Constants
const (
	backupDirName = "tnf-node-replacement-backup"

	// OpenShift namespaces
	machineAPINamespace = "openshift-machine-api"
	// etcdClusterOperatorCRName is the cluster-scoped Etcd CR CEO reconciles (status.nodeStatuses is keyed by node name only).
	etcdClusterOperatorCRName = "cluster"
	// kubeAPIServerOperatorCRName is the cluster-scoped KubeAPIServer CR cluster-kube-apiserver-operator reconciles
	// (status.nodeStatuses is keyed by node name only; same-name replacement needs the row dropped like Etcd).
	kubeAPIServerOperatorCRName = "cluster"
	ovnKubernetesNamespace      = "openshift-ovn-kubernetes"
	// ovnkubeNodeSBDBContainer is the sidecar in app=ovnkube-node that runs ovn-sbctl against the local SB socket.
	// Note: app=ovnkube-control-plane pods on recent OpenShift releases often do not include an sbdb container
	// (only kube-rbac-proxy + ovnkube-cluster-manager); chassis cleanup must fall back to a surviving ovnkube-node.
	ovnkubeNodeSBDBContainer = "sbdb"

	// Phases log "[stage timing]" with elapsed wall time so timeouts below can be compared to observed duration
	// when a step fails or a cluster is unusually slow.

	// Phase-oriented timeouts (cross-check with "[stage timing]" lines when adjusting caps).
	shortK8sClientTimeout      = 15 * time.Second // short client List/Get/exec contexts (e.g. ovn-sbctl)
	stonithCleanupRoundTimeout = 1 * time.Minute  // per stonith cleanup round
	etcdThreeMinutePollTimeout = 3 * time.Minute  // etcd stop, API responsive waits, node removal from API, phase 2 API wait
	// etcdPhase1StartAfterStonithTimeout caps polling for etcd to run on the survivor after pcs stonith confirm
	// (phase 1). Longer than etcdThreeMinutePollTimeout because static pods and CEO reconciliation after fencing
	// routinely outlast the shorter generic etcd/API waits.
	etcdPhase1StartAfterStonithTimeout  = 7 * time.Minute
	vmLibvirtRunningTimeout             = 3 * time.Minute  // WaitForVMState after VM recreate
	clusterOperatorStabilizationTimeout = 10 * time.Minute // cluster operator stabilization (long polls)
	// CEO tnf-update-setup Job completion per node (survivor + target waited in parallel in restorePacemakerCluster).
	ceoUpdateSetupJobWaitTimeout = 3 * time.Minute
	// Pacemaker: both nodes online in pcs after CEO jobs (SSH via survivor).
	pacemakerNodesOnlineTimeout = 2 * time.Minute

	// Machine.status.nodeRef after BMH provisions a replacement control-plane host (often slower than generic API waits).
	machineNodeRefWaitTimeout = 10 * time.Minute

	// Node recovery: wait for CSR approval first, then allow time for node to become Ready.
	// The Ready timeout only starts after CSR is approved so we don't burn time on machine-approver.
	csrApprovalWaitTimeout   = 3 * time.Minute // max wait for machine-approver / manual CSR approval
	nodeReadyAfterCSRTimeout = 2 * time.Minute // node Ready after CSR (network + containers)

	// Retry configuration
	maxDeleteRetries = 3
	// etcdOperatorNodeStatusCleanupMaxAttempts is UpdateStatus retries on 409 conflict while dropping a deleted
	// node's operator nodeStatuses row (CEO / cluster-kube-apiserver-operator may write status concurrently).
	etcdOperatorNodeStatusCleanupMaxAttempts = 5

	// deleteBMHMachineMaxAttempts is how many times we issue a regular Delete for BMH/Machine before failing (no finalizer strip).
	// Five rounds with deleteBMHMachineRetryInterval yields ~15m budget for BMO/Ironic to finish deprovisioning between attempts.
	deleteBMHMachineMaxAttempts = 5
	// deleteBMHMachineRetryInterval is the wait between failed delete attempts so BMO/Ironic can reconcile.
	deleteBMHMachineRetryInterval = 3 * time.Minute
	// deleteAttemptTimeout caps each Delete call; 3-minute blocks give the API/webhook and BMO time to process.
	deleteAttemptTimeout = 3 * time.Minute
	// deleteGetTimeout caps the existence-check Get after each Delete; 20s is enough for a simple Get.
	deleteGetTimeout = 20 * time.Second

	// recoverBMHTerminatingMaxChecks is how many times recoverBMHFromBackup polls while the BMH is deleting before failing.
	recoverBMHTerminatingMaxChecks = 3
	// recoverBMHTerminatingPollInterval is the wait between those polls (checks at ~0, 2m, 4m).
	recoverBMHTerminatingPollInterval = 2 * time.Minute
	// recoverBMHTerminatingMaxWait caps total wall time for that recovery wait (fits 3 checks + intervals).
	recoverBMHTerminatingMaxWait = 6 * time.Minute

	// baremetalOperatorDeploymentName is the default Deployment name (OCP); metal3/dev-scripts use "metal3-baremetal-operator".
	// waitForBaremetalOperatorDeploymentReady discovers the actual name by trying both.
	baremetalOperatorDeploymentName       = "baremetal-operator"
	baremetalOperatorDeploymentNameMetal3 = "metal3-baremetal-operator"

	// Pacemaker configuration
	pacemakerCleanupWaitTime  = 15 * time.Second
	pacemakerJournalLines     = 25
	stonithConfirmSettleWait  = 5 * time.Second // brief wait after stonith confirm before checking etcd
	stonithCleanupMaxAttempts = 3               // tryStonithDisableCleanup runs resource cleanup this many times

	// Provisioning timeouts (Ironic; baremetal CI / dev-scripts)
	bmhProvisioningTimeout = 12 * time.Minute

	// Resource types
	secretResourceType  = "secret"
	bmhResourceType     = "bmh"
	machineResourceType = "machines.machine.openshift.io"

	// Wait for BMO deployment ready before BMH/Machine deletes; after node loss BMO may need to reschedule onto survivor.
	baremetalOperatorDeploymentWaitTimeout = 5 * time.Minute

	// baremetalOperatorWebhookServiceName is the Service that exposes the BMH validating webhook; we wait for it to have endpoints before creating a BMH
	baremetalOperatorWebhookServiceName = "baremetal-operator-webhook-service"
	// Wait for webhook to be ready before creating BMH (avoids "no endpoints available" on create)
	baremetalWebhookWaitTimeout  = 5 * time.Minute
	baremetalWebhookPollInterval = 15 * time.Second

	// Output formats
	yamlOutputFormat = "yaml"

	// Annotations
	bmhDetachedAnnotation    = "baremetalhost.metal3.io/detached=''" // for oc annotate
	bmhDetachedAnnotationKey = "baremetalhost.metal3.io/detached"    // for API annotations map

	// Base names for dynamic resource names
	etcdPeerSecretBaseName           = "etcd-peer"
	etcdServingSecretBaseName        = "etcd-serving"
	etcdServingMetricsSecretBaseName = "etcd-serving-metrics"
	tnfAuthJobBaseName               = "tnf-auth-job"
	tnfAfterSetupJobBaseName         = "tnf-after-setup-job"
	tnfUpdateSetupJobBaseName        = "tnf-update-setup-job"

	// Redfish BMC port (used with net.JoinHostPort for BMH address authority)
	redfishPort = "8000"

	// Virsh commands
	virshProvisioningBridge = "ostestpr"

	// Template paths (relative to test/extended/ - framework FixturePath will prefix automatically)
	templateBaseDir     = "testdata/two_node"
	bmhTemplatePath     = templateBaseDir + "/baremetalhost-template.yaml"
	machineTemplatePath = templateBaseDir + "/machine-template.yaml"

	// File patterns
	vmXMLFilePattern = "/tmp/%s.xml"

	// Fresh backing disk for node replacement: default size when original disk size cannot be read (e.g. missing or qemu-img info fails)
	defaultFreshDiskSize = "100G"
	// Suffix for backing disk backup on hypervisor; restore only when recovering from backup
	backingDiskBackupSuffix = ".tnf-backup"

	// East-west connectivity: we use PodNetworkConnectivityCheck from surviving node to target node's network-check-target.
	networkDiagnosticsNamespace = "openshift-network-diagnostics"
	// East-west check after replacement while the OVN data plane converges after chassis-del and new node registration.
	// The successful path runs this before recycling survivor ovnkube-node or restarting all OVN-K pods (reserved for
	// failure recovery), so the cap must cover slow SB/port_binding and dataplane catch-up.
	eastWestConnectivityTimeout = 12 * time.Minute
	ovnkubeRestartSettleWait    = 90 * time.Second
	// recoverOVNKForNodeReplacement lists/deletes multiple OVN-K pods; 15s is too tight under API load.
	ovnkubeRecoveryAPITimeout = 2 * time.Minute
	// After deleting ovnkube-node on survivor + replacement, wait until both DaemonSet pods are Ready again.
	ovnkubeNodeAfterRestartWaitTimeout  = 5 * time.Minute
	ovnkubeNodeAfterRestartPollInterval = 10 * time.Second
	machineNodeRefPollInterval          = 15 * time.Second
	eastWestConnectivityPollInterval    = 15 * time.Second
	// Minimum port_bindings expected for a node's chassis when queried from the other node's SB view.
	// In healthy post-replacement clusters we can observe only the gateway binding (count=1), so requiring >=2
	// causes false negatives.
	minPortBindingsForNodeChassis = 1
	// SB programming/replication can lag east-west success after chassis-del / new node registration; poll before failing.
	ovnSBPortBindingsWaitTimeout  = 10 * time.Minute
	ovnSBPortBindingsPollInterval = 20 * time.Second
	// After chassis-del, poll SB until this node's hostname (and pre-replace Chassis.name, when known) no longer
	// appears in Chassis — proof before any replacement provisioning. OVN-K still sets node-chassis-id from local
	// OVS system-id; identical system-id on the new host can republish the same annotation even when SB is clean.
	ovnSBChassisAbsentWaitTimeout  = 5 * time.Minute
	ovnSBChassisAbsentPollInterval = 5 * time.Second

	// Static pod revision bump: after node replacement, operators may not re-run installers on the replaced node.
	// We patch kube-apiserver/KCM/scheduler to Trace then back to Normal to force a new revision and re-install on all control-plane nodes.
	staticPodRevisionBumpSettleWait = 90 * time.Second // time to allow installers to start after patching to Trace before reverting
	// After revision bump, static-pod installers must write manifests on the replacement control-plane node.
	staticPodManifestsWaitTimeout  = 2 * time.Minute
	staticPodManifestsPollInterval = 45 * time.Second
)

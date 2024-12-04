package constants

const (
	// XXX
	//
	// Add a constant here, if and only if: it's exported (of course) and it's reused across the entire project.
	// Otherwise, prefer an unexported const in a specific package.
	//
	// XXX

	// CurrentImageAnnotationKey is used to get the current OS image pullspec for a machine
	CurrentImageAnnotationKey = "machineconfiguration.openshift.io/currentImage"
	// DesiredImageAnnotationKey is used to specify the desired OS image pullspec for a machine
	DesiredImageAnnotationKey = "machineconfiguration.openshift.io/desiredImage"

	// CurrentMachineConfigAnnotationKey is used to fetch current MachineConfig for a machine
	CurrentMachineConfigAnnotationKey = "machineconfiguration.openshift.io/currentConfig"
	// DesiredMachineConfigAnnotationKey is used to specify the desired MachineConfig for a machine
	DesiredMachineConfigAnnotationKey = "machineconfiguration.openshift.io/desiredConfig"
	// MachineConfigDaemonStateAnnotationKey is used to fetch the state of the daemon on the machine.
	MachineConfigDaemonStateAnnotationKey = "machineconfiguration.openshift.io/state"
	// DesiredDrainerAnnotationKey is set by the MCD to indicate drain/uncordon requests
	DesiredDrainerAnnotationKey = "machineconfiguration.openshift.io/desiredDrain"
	// LastAppliedDrainerAnnotationKey is set by the controller to indicate the last request applied
	LastAppliedDrainerAnnotationKey = "machineconfiguration.openshift.io/lastAppliedDrain"
	// DrainerStateDrain is used for drainer annotation as a value to indicate needing a drain
	DrainerStateDrain = "drain"
	// DrainerStateUncordon is used for drainer annotation as a value to indicate needing an uncordon
	DrainerStateUncordon = "uncordon"
	// ClusterControlPlaneTopologyAnnotationKey is set by the node controller by reading value from
	// controllerConfig. MCD uses the annotation value to decide drain action on the node.
	ClusterControlPlaneTopologyAnnotationKey = "machineconfiguration.openshift.io/controlPlaneTopology"
	// OpenShiftOperatorManagedLabel is used to filter out kube objects that don't need to be synced by the MCO
	OpenShiftOperatorManagedLabel = "openshift.io/operator-managed"
	// ControllerConfigResourceVersionKey is used for the certificate writer to indicate the last controllerconfig object it synced upon
	ControllerConfigResourceVersionKey = "machineconfiguration.openshift.io/lastSyncedControllerConfigResourceVersion"
	// ControllerConfigSyncServerCA is used to determine if we have already synced the server CA for this version of the controller config
	ControllerConfigSyncServerCA = "machineconfiguration.openshift.io/lastObservedServerCAAnnotation"
	// GeneratedByVersionAnnotationKey is used to tag the controllerconfig to synchronize the MCO and MCC
	GeneratedByVersionAnnotationKey = "machineconfiguration.openshift.io/generated-by-version"

	// MachineConfigDaemonStateWorking is set by daemon when it is beginning to apply an update.
	MachineConfigDaemonStateWorking = "Working"
	// MachineConfigDaemonStateDone is set by daemon when it is done applying an update.
	MachineConfigDaemonStateDone = "Done"
	// MachineConfigDaemonStateDegraded is set by daemon when an error not caused by a bad MachineConfig
	// is thrown during an update.
	MachineConfigDaemonStateDegraded = "Degraded"
	// MachineConfigDaemonRebooting is used to indicate a reboot is either queued or is in progress.
	MachineConfigDaemonStateRebooting = "Rebooting"
	// MachineConfigDaemonStateUnreconcilable is set by the daemon when a MachineConfig cannot be applied.
	MachineConfigDaemonStateUnreconcilable = "Unreconcilable"
	// MachineConfigDaemonReasonAnnotationKey is set by the daemon when it needs to report a human readable reason for its state. E.g. when state flips to degraded/unreconcilable.
	MachineConfigDaemonReasonAnnotationKey = "machineconfiguration.openshift.io/reason"
	// MachineConfigDaemonPostConfigAction is set by the daemon when it needs to report a human readable post config action that takes place during update.
	MachineConfigDaemonPostConfigAction = "machineconfiguration.openshift.io/post-config-action"
	// MachineConfigDaemonFinalizeFailureAnnotationKey is set by the daemon when ostree fails to finalize
	MachineConfigDaemonFinalizeFailureAnnotationKey = "machineconfiguration.openshift.io/ostree-finalize-staged-failure"
	// InitialNodeAnnotationsFilePath defines the path at which it will find the node annotations it needs to set on the node once it comes up for the first time.
	// The Machine Config Server writes the node annotations to this path.
	InitialNodeAnnotationsFilePath = "/etc/machine-config-daemon/node-annotations.json"
	// InitialNodeAnnotationsBakPath defines the path of InitialNodeAnnotationsFilePath when the initial bootstrap is done. We leave it around for debugging and reconciling.
	InitialNodeAnnotationsBakPath = "/etc/machine-config-daemon/node-annotation.json.bak"

	// IgnitionSystemdPresetFile is where Ignition writes initial enabled/disabled systemd unit configs
	// This should be removed on boot after MCO takes over, so if any of these are deleted we can go back
	// to initial system settings
	IgnitionSystemdPresetFile = "/etc/systemd/system-preset/20-ignition.preset"

	// EtcPivotFile is used by the `pivot` command
	// For more information, see https://github.com/openshift/pivot/pull/25/commits/c77788a35d7ee4058d1410e89e6c7937bca89f6c#diff-04c6e90faac2675aa89e2176d2eec7d8R44
	EtcPivotFile = "/etc/pivot/image-pullspec"

	// MachineConfigEncapsulatedPath contains all of the data from a MachineConfig object
	// except the Spec/Config object; this supports inverting+encapsulating a MachineConfig
	// object so that Ignition can process it on first boot, and then the MCD can act on
	// non-Ignition fields such as the osImageURL and kernelArguments.
	MachineConfigEncapsulatedPath = "/etc/ignition-machine-config-encapsulated.json"

	// MachineConfigEncapsulatedBakPath defines the path where the machineconfigdaemom-firstboot.service
	// will leave a copy of the encapsulated MachineConfig in MachineConfigEncapsulatedPath after
	// processing for debugging and auditing purposes.
	MachineConfigEncapsulatedBakPath = "/etc/ignition-machine-config-encapsulated.json.bak"

	// MachineConfigDaemonForceFile if present causes the MCD to skip checking the validity of the
	// "currentConfig" state.  Create this file (empty contents is fine) if you wish the MCD
	// to proceed and attempt to "reconcile" to the new "desiredConfig" state regardless.
	MachineConfigDaemonForceFile = "/run/machine-config-daemon-force"

	// coreUser is "core" and currently the only permissible user name
	CoreUserName  = "core"
	CoreGroupName = "core"

	// changes to registries.conf will cause a crio reload and require extra logic about whether to drain
	ContainerRegistryConfPath = "/etc/containers/registries.conf"

	// changes to registries.conf will cause a crio reload
	ContainerRegistryPolicyPath = "/etc/containers/policy.json"

	// changes to registries.d will cause a crio reload
	SigstoreRegistriesConfigDir = "/etc/containers/registries.d"

	// changes to /etc/crio/policies will cause a crio reload
	CrioPoliciesDir = "/etc/crio/policies"

	// changes to openshift-config-user-ca-bundle.crt will cause an update-ca-trust and crio restart
	UserCABundlePath = "/etc/pki/ca-trust/source/anchors/openshift-config-user-ca-bundle.crt"

	// Changes to this directory should not trigger reboots because they are firstboot-only
	OpenShiftNMStateConfigDir = "/etc/nmstate/openshift"

	// SSH Keys for user "core" will only be written at /home/core/.ssh
	CoreUserSSHPath = "/home/" + CoreUserName + "/.ssh"

	// SSH keys in RHCOS 8 will be written to /home/core/.ssh/authorized_keys
	RHCOS8SSHKeyPath = CoreUserSSHPath + "/authorized_keys"

	// SSH keys in RHCOS 9 / FCOS / SCOS will be written to /home/core/.ssh/authorized_keys.d/ignition
	RHCOS9SSHKeyPath = CoreUserSSHPath + "/authorized_keys.d/ignition"

	// CRIOServiceName is used to specify reloads and restarts of the CRI-O service
	CRIOServiceName = "crio"

	// DaemonReloadCommand is used to specify reloads and restarts of the systemd manager configuration
	DaemonReloadCommand = "daemon-reload"

	// UpdateCATrustServiceName is a service present on CoresOS nodes that runs the update-ca-trust command
	UpdateCATrustServiceName = "coreos-update-ca-trust.service"

	// UpdateCATrustCommand will be used to run update-ca-trust directly. This is a fallback for scenarios
	// where the above service doesn't exist, for example on RHEL nodes.
	UpdateCATrustCommand = "update-ca-trust"

	// DefaultCRIOSocketPath is the default path to the CRI-O socket
	DefaultCRIOSocketPath = "/var/run/crio/crio.sock"

	// KubeletAuthFile is the path to the kubelet auth file.
	KubeletAuthFile = "/var/lib/kubelet/config.json"

	// MinFreeStorageAfterPrefetch is the minimum amount of storage
	// available on the root filesystem after prefetching images.
	MinFreeStorageAfterPrefetch = "16Gi"

	// GPGNoRebootPath is the path MCO expects will contain GPG key updates. MCO will attempt to only reload crio for
	// changes to this path. Note that other files added to the parent directory will not be handled specially
	GPGNoRebootPath = "/etc/machine-config-daemon/no-reboot/containers-gpg.pub"

	// ImageRegistryDrainOverrideConfigmap is the name of the Configmap a user can apply to force all
	// image registry changes to not drain
	ImageRegistryDrainOverrideConfigmap = "image-registry-override-drain"
)

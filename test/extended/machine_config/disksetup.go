package machine_config

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	// diskSetupMCPrefix is the name prefix the installer gives every MachineConfig it
	// generates from a machine pool's diskSetup stanza.
	diskSetupMCPrefix = "01-disk-setup-"

	// diskSetupPartlabelPrefix is the by-partlabel symlink directory that the generated
	// filesystem and systemd unit both address the partition through.
	diskSetupPartlabelPrefix = "/dev/disk/by-partlabel/"

	// azureLunDevicePrefix is the device path the installer partitions on Azure; the data
	// disk's LUN is appended to it.
	azureLunDevicePrefix = "/dev/disk/azure/scsi1/lun"

	// diskSetupDebugNamespace is the namespace node debug pods are created in.
	diskSetupDebugNamespace = "default"

	// prjquotaMountOption is required on data disks so the kubelet can enforce project
	// quotas on the mounted filesystem.
	prjquotaMountOption = "prjquota"

	// machineAnnotation links a Node to the Machine that provisioned it.
	machineAnnotation = "machine.openshift.io/machine"

	// gibiByte converts the API's diskSizeGB into the byte count lsblk reports.
	gibiByte = 1024 * 1024 * 1024

	// diskSizeTolerance is the fraction of the requested size the observed block device is
	// allowed to fall short by, since the partition table costs some addressable space.
	diskSizeTolerance = 0.98

	// diskStateTimeout bounds how long to wait for a node's disk to reach the expected
	// state. The disks are set up by Ignition on first boot, so they are normally ready
	// well before these tests run; this only absorbs a slow or restarting debug pod.
	diskStateTimeout = 2 * time.Minute

	// diskStateInterval is how often to retry collecting a node's disk state. Each attempt
	// starts a debug pod, so this is deliberately coarse.
	diskStateInterval = 20 * time.Second
)

// expectedDisk describes one data disk the install-config under test is expected to have
// configured. These values are the contract between this test and the CI step that
// generates the install-config
// (ci-operator/step-registry/ipi/conf/azure/multidisk in openshift/release); changing one
// without the other fails the suite rather than silently skipping it.
type expectedDisk struct {
	// role is the machine role whose pool declares the disk.
	role string
	// mountPath is where the disk's filesystem is expected to be mounted.
	mountPath string
	// lun is the logical unit number the disk is requested at in the install-config.
	lun int32
	// sizeGB is the diskSizeGB requested in the install-config.
	sizeGB int32
	// storageAccountType is the managedDisk.storageAccountType requested in the install-config.
	storageAccountType string
}

// expectedDisks is the disk layout this suite asserts: two extra disks on the control plane
// and two on the compute nodes. etcd disk setup is only valid on the control plane, so the
// compute pool carries two user-defined disks instead.
var expectedDisks = []expectedDisk{
	{role: "master", mountPath: "/var/lib/etcd", lun: 0, sizeGB: 64, storageAccountType: "Premium_LRS"},
	{role: "master", mountPath: "/var/lib/containers", lun: 1, sizeGB: 32, storageAccountType: "StandardSSD_LRS"},
	{role: "worker", mountPath: "/var/lib/containers", lun: 0, sizeGB: 32, storageAccountType: "Premium_LRS"},
	{role: "worker", mountPath: "/var/lib/kubelet", lun: 1, sizeGB: 16, storageAccountType: "StandardSSD_LRS"},
}

// ignitionDiskConfig is a minimal view of the Ignition config the installer embeds in a
// disk-setup MachineConfig. It decodes only the fields asserted on here so that it stays
// compatible across Ignition spec revisions.
type ignitionDiskConfig struct {
	Storage struct {
		Disks []struct {
			Device     string `json:"device"`
			Partitions []struct {
				Label *string `json:"label"`
			} `json:"partitions"`
		} `json:"disks"`
		Filesystems []struct {
			Device       string   `json:"device"`
			Format       *string  `json:"format"`
			MountOptions []string `json:"mountOptions"`
			Path         *string  `json:"path"`
		} `json:"filesystems"`
	} `json:"storage"`
	Systemd struct {
		Units []struct {
			Name    string `json:"name"`
			Enabled *bool  `json:"enabled"`
		} `json:"units"`
	} `json:"systemd"`
}

// findmntResult decodes `findmnt --json`.
type findmntResult struct {
	Filesystems []struct {
		Source  string `json:"source"`
		FSType  string `json:"fstype"`
		Options string `json:"options"`
	} `json:"filesystems"`
}

// diskSetup is a disk-setup MachineConfig decoded into the facts the assertions need.
type diskSetup struct {
	// mcName is the MachineConfig's name.
	mcName string
	// role is the machine role the MachineConfig targets.
	role string
	// partLabel is the GPT partition label applied to the data disk. Note that this is not
	// the platformDiskID: the installer uses the disk setup's type as the label for etcd and
	// swap disks, and strips every non-alphanumeric character from it. Use the LUN, not this,
	// to pair a disk setup with its Azure data disk.
	partLabel string
	// backingDevice is the platform device partitioned, e.g. /dev/disk/azure/scsi1/lun0.
	backingDevice string
	// format is the filesystem written to the partition.
	format string
	// mountPath is where the filesystem is mounted.
	mountPath string
	// mountOptions are the options the filesystem is created with.
	mountOptions []string
	// unitName is the systemd unit the installer enables to mount the disk.
	unitName string
}

// partitionDevice is the by-partlabel path the filesystem and systemd unit address.
func (d diskSetup) partitionDevice() string {
	return diskSetupPartlabelPrefix + d.partLabel
}

// lun extracts the Azure LUN the disk is attached at from the partitioned device path. The
// LUN is unique per virtual machine and is what identifies the data disk, so it is the
// reliable way to pair a disk setup with its entry in the Machine provider spec.
func (d diskSetup) lun() (int32, error) {
	if !strings.HasPrefix(d.backingDevice, azureLunDevicePrefix) {
		return 0, fmt.Errorf("device %q is not an Azure LUN device", d.backingDevice)
	}
	lun, err := strconv.Atoi(strings.TrimPrefix(d.backingDevice, azureLunDevicePrefix))
	if err != nil {
		return 0, fmt.Errorf("device %q has a malformed LUN: %w", d.backingDevice, err)
	}
	return int32(lun), nil
}

// getDiskSetups returns every disk-setup MachineConfig in the cluster, decoded.
func getDiskSetups(ctx context.Context, client *machineconfigclient.Clientset) []diskSetup {
	mcList, err := client.MachineconfigurationV1().MachineConfigs().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error listing MachineConfigs.")

	setups := []diskSetup{}
	for _, mc := range mcList.Items {
		if !strings.HasPrefix(mc.Name, diskSetupMCPrefix) {
			continue
		}
		setups = append(setups, decodeDiskSetup(mc))
	}
	return setups
}

// decodeDiskSetup parses a disk-setup MachineConfig into a diskSetup.
func decodeDiskSetup(mc mcfgv1.MachineConfig) diskSetup {
	ign := ignitionDiskConfig{}
	err := json.Unmarshal(mc.Spec.Config.Raw, &ign)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error decoding the Ignition config of MachineConfig %q.", mc.Name))

	o.Expect(ign.Storage.Disks).To(o.HaveLen(1), fmt.Sprintf("MachineConfig %q should configure exactly one disk.", mc.Name))
	o.Expect(ign.Storage.Disks[0].Partitions).To(o.HaveLen(1), fmt.Sprintf("MachineConfig %q should configure exactly one partition.", mc.Name))
	o.Expect(ign.Storage.Filesystems).To(o.HaveLen(1), fmt.Sprintf("MachineConfig %q should configure exactly one filesystem.", mc.Name))
	o.Expect(ign.Systemd.Units).To(o.HaveLen(1), fmt.Sprintf("MachineConfig %q should configure exactly one systemd unit.", mc.Name))

	partition := ign.Storage.Disks[0].Partitions[0]
	o.Expect(partition.Label).NotTo(o.BeNil(), fmt.Sprintf("MachineConfig %q should label its partition.", mc.Name))

	filesystem := ign.Storage.Filesystems[0]
	o.Expect(filesystem.Format).NotTo(o.BeNil(), fmt.Sprintf("MachineConfig %q should set a filesystem format.", mc.Name))

	// An enabled unit is what actually activates the disk; a disabled one would leave the
	// partition formatted but never mounted.
	unit := ign.Systemd.Units[0]
	o.Expect(unit.Enabled).NotTo(o.BeNil(), fmt.Sprintf("MachineConfig %q should set the enabled state of its systemd unit.", mc.Name))
	o.Expect(*unit.Enabled).To(o.BeTrue(), fmt.Sprintf("MachineConfig %q should enable its systemd unit.", mc.Name))

	setup := diskSetup{
		mcName:        mc.Name,
		role:          mc.Labels["machineconfiguration.openshift.io/role"],
		partLabel:     *partition.Label,
		backingDevice: ign.Storage.Disks[0].Device,
		format:        *filesystem.Format,
		mountOptions:  filesystem.MountOptions,
		unitName:      ign.Systemd.Units[0].Name,
	}
	if filesystem.Path != nil {
		setup.mountPath = *filesystem.Path
	}
	return setup
}

// indexDiskSetups keys the disk setups by role and mount path, failing on duplicates so
// that a malformed layout is reported rather than silently resolved to the first match.
func indexDiskSetups(setups []diskSetup) map[string]diskSetup {
	indexed := map[string]diskSetup{}
	for _, setup := range setups {
		key := setup.role + " " + setup.mountPath
		_, duplicate := indexed[key]
		o.Expect(duplicate).To(o.BeFalse(),
			fmt.Sprintf("Found more than one disk setup mounting %s on the %s pool.", setup.mountPath, setup.role))
		indexed[key] = setup
	}
	return indexed
}

// diskSetupFor returns the disk setup for a role and mount path, failing when it is absent.
// Every disk in expectedDisks must be configured for this suite to be meaningful, so a
// missing one is a contract mismatch with the install-config rather than a reason to skip.
func diskSetupFor(indexed map[string]diskSetup, role, mountPath string) diskSetup {
	setup, ok := indexed[role+" "+mountPath]
	if !ok {
		configured := make([]string, 0, len(indexed))
		for key := range indexed {
			configured = append(configured, key)
		}
		g.Fail(fmt.Sprintf("Expected a disk setup mounting %s on the %s pool. The cluster was installed with: %v. "+
			"The install-config used by this job and the expectedDisks table in this test have diverged.",
			mountPath, role, configured))
	}
	return setup
}

// getNodesWithRole returns the nodes carrying the given role, failing when there are none.
func getNodesWithRole(oc *exutil.CLI, role string) []corev1.Node {
	nodes, err := GetNodesByRole(oc, role)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting nodes with role %q.", role))
	o.Expect(nodes).NotTo(o.BeEmpty(), fmt.Sprintf("Expected at least one node with role %q.", role))
	return nodes
}

// nodeDiskState is everything one node reports about a single data disk, gathered in a
// single debug pod so that a check costs one pod rather than one per command.
type nodeDiskState struct {
	// resolvedPartition is the block device the by-partlabel symlink points at.
	resolvedPartition string
	// partitionFSType is the filesystem blkid finds on the partition.
	partitionFSType string
	// mountSource, mountFSType and mountOptions describe the mount at the expected path.
	mountSource  string
	mountFSType  string
	mountOptions string
	// rootSource is the device backing /, used to prove the mount is a separate disk.
	rootSource string
	// resolvedBackingDevice is the block device the Azure LUN symlink points at.
	resolvedBackingDevice string
	// partitionParent is the whole device the mounted partition was carved out of, which
	// must be the Azure LUN the MachineConfig partitioned.
	partitionParent string
}

// collectNodeDiskState gathers a node's view of one data disk in one debug pod.
//
// Paths are passed to the shell as positional parameters rather than interpolated into the
// script, so that a mount path containing a space or a shell metacharacter cannot change
// what runs.
func collectNodeDiskState(oc *exutil.CLI, nodeName, partDevice, mountPath, backingDevice string) (nodeDiskState, error) {
	const sep = "---8<---"
	script := fmt.Sprintf(`
readlink -f "$1"; echo '%[1]s'
blkid -s TYPE -o value "$1"; echo '%[1]s'
findmnt --json --output SOURCE,FSTYPE,OPTIONS --target "$2"; echo '%[1]s'
findmnt --first-only --noheadings --output SOURCE --target /; echo '%[1]s'
readlink -f "$3"; echo '%[1]s'
lsblk --noheadings --output PKNAME "$1"
`, sep)

	out, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, diskSetupDebugNamespace,
		"sh", "-c", script, "disk-setup-check", partDevice, mountPath, backingDevice)
	if err != nil {
		return nodeDiskState{}, fmt.Errorf("running the disk inspection script on node %q: %w", nodeName, err)
	}

	sections := strings.Split(out, sep)
	if len(sections) != 6 {
		return nodeDiskState{}, fmt.Errorf("expected 6 sections from node %q, got %d in: %s", nodeName, len(sections), out)
	}

	state := nodeDiskState{
		resolvedPartition:     strings.TrimSpace(sections[0]),
		partitionFSType:       strings.TrimSpace(sections[1]),
		rootSource:            strings.TrimSpace(sections[3]),
		resolvedBackingDevice: strings.TrimSpace(sections[4]),
	}

	if parent := strings.TrimSpace(sections[5]); parent != "" {
		state.partitionParent = "/dev/" + parent
	}

	mounts := findmntResult{}
	if err := json.Unmarshal([]byte(sections[2]), &mounts); err != nil {
		return nodeDiskState{}, fmt.Errorf("decoding findmnt output for %s on node %q: %w", mountPath, nodeName, err)
	}
	if len(mounts.Filesystems) == 0 {
		return nodeDiskState{}, fmt.Errorf("nothing is mounted at %s on node %q", mountPath, nodeName)
	}
	state.mountSource = mounts.Filesystems[0].Source
	state.mountFSType = mounts.Filesystems[0].FSType
	state.mountOptions = mounts.Filesystems[0].Options

	return state, nil
}

// assertDiskIsMountedAt asserts the full chain for one data disk on every node of its pool:
// the partition exists under its label, it holds an xfs filesystem, and that filesystem is
// mounted at the expected path on a device distinct from the root filesystem.
//
// The last part is what distinguishes a disk that was set up from one that was merely
// attached: without it the mount point silently falls back to the root disk and the
// assertion would still pass.
func assertDiskIsMountedAt(oc *exutil.CLI, setup diskSetup, mountPath string) {
	o.Expect(setup.format).To(o.Equal("xfs"),
		fmt.Sprintf("MachineConfig %q should format its data disk as xfs.", setup.mcName))
	o.Expect(setup.mountOptions).To(o.ContainElement(prjquotaMountOption),
		fmt.Sprintf("MachineConfig %q should request the %s mount option.", setup.mcName, prjquotaMountOption))

	// The installer derives the unit name from the mount path, so a mismatch here means the
	// partition would be created but never mounted.
	expectedUnit := strings.ReplaceAll(strings.Trim(mountPath, "/"), "/", "-") + ".mount"
	o.Expect(setup.unitName).To(o.Equal(expectedUnit),
		fmt.Sprintf("MachineConfig %q should enable the systemd mount unit for %s.", setup.mcName, mountPath))

	for _, node := range getNodesWithRole(oc, setup.role) {
		nodeName := node.Name
		framework.Logf("Checking %s on node %s", mountPath, nodeName)

		o.Eventually(func() error {
			state, err := collectNodeDiskState(oc, nodeName, setup.partitionDevice(), mountPath, setup.backingDevice)
			if err != nil {
				return err
			}

			if !strings.HasPrefix(state.resolvedPartition, "/dev/") {
				return fmt.Errorf("partition %q should resolve to a block device, got %q",
					setup.partitionDevice(), state.resolvedPartition)
			}
			if state.partitionFSType != "xfs" {
				return fmt.Errorf("partition %q should hold an xfs filesystem, got %q",
					setup.partitionDevice(), state.partitionFSType)
			}
			if state.mountSource != state.resolvedPartition {
				return fmt.Errorf("%s should be backed by the %q data disk at %q, got %q",
					mountPath, setup.partLabel, state.resolvedPartition, state.mountSource)
			}
			if state.mountSource == state.rootSource {
				return fmt.Errorf("%s should not share a device with the root filesystem, both are %q",
					mountPath, state.rootSource)
			}
			// Tie the mounted partition back to the data disk the MachineConfig targeted.
			// Without this the partition label alone could be satisfied by a partition on
			// any disk, including the one holding the root filesystem.
			if state.partitionParent != state.resolvedBackingDevice {
				return fmt.Errorf("%s should be backed by a partition of %q (%q), but its partition %q lives on %q",
					mountPath, setup.backingDevice, state.resolvedBackingDevice, state.resolvedPartition, state.partitionParent)
			}
			if state.resolvedBackingDevice == state.rootSource {
				return fmt.Errorf("%s should not be backed by the root device %q", mountPath, state.rootSource)
			}
			if state.mountFSType != "xfs" {
				return fmt.Errorf("%s should be an xfs filesystem, got %q", mountPath, state.mountFSType)
			}
			if !strings.Contains(","+state.mountOptions+",", ","+prjquotaMountOption+",") {
				return fmt.Errorf("%s should be mounted with %s, got %q", mountPath, prjquotaMountOption, state.mountOptions)
			}
			return nil
		}, diskStateTimeout, diskStateInterval).Should(o.Succeed(),
			fmt.Sprintf("Data disk %q was not set up at %s on node %q.", setup.partLabel, mountPath, nodeName))
	}
}

// azureDataDisksByNode returns the data disks declared in the Machine provider spec of every
// machine with the given role, keyed by node name.
func azureDataDisksByNode(ctx context.Context, oc *exutil.CLI, client *machineclient.Clientset, role string) map[string][]machinev1beta1.DataDisk {
	byNode := map[string][]machinev1beta1.DataDisk{}

	for _, node := range getNodesWithRole(oc, role) {
		annotation, ok := node.Annotations[machineAnnotation]
		o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("Node %q should carry the %s annotation.", node.Name, machineAnnotation))

		namespace, name, found := strings.Cut(annotation, "/")
		o.Expect(found).To(o.BeTrue(),
			fmt.Sprintf("The %s annotation on node %q should be namespace/name, got %q.", machineAnnotation, node.Name, annotation))

		machine, err := client.MachineV1beta1().Machines(namespace).Get(ctx, name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting Machine %q for node %q.", annotation, node.Name))
		o.Expect(machine.Spec.ProviderSpec.Value).NotTo(o.BeNil(),
			fmt.Sprintf("Machine %q should have a provider spec.", annotation))

		providerSpec := machinev1beta1.AzureMachineProviderSpec{}
		err = json.Unmarshal(machine.Spec.ProviderSpec.Value.Raw, &providerSpec)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error decoding the Azure provider spec of Machine %q.", annotation))

		byNode[node.Name] = providerSpec.DataDisks
	}

	return byNode
}

// These tests assert that the data disks described by a machine pool's diskSetup stanza were
// attached, partitioned, formatted and mounted on the nodes, and that the Azure-specific
// knobs in the install-config reached the Machine provider spec.
//
// They are read-only: they inspect cluster and node state without mutating either, so they
// are safe to run in the parallel conformance suite. They require a cluster installed with
// the layout in expectedDisks, and fail rather than skip when it is absent.
var _ = g.Describe("[sig-mco][OCPFeatureGate:AzureMultiDisk][OCPFeatureGate:MultiDiskSetup] Azure machine pool disk setup", func() {
	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLIWithoutNamespace("machine-config")
		mcClientSet   *machineconfigclient.Clientset
		machineClient *machineclient.Clientset
		setups        map[string]diskSetup
	)

	g.BeforeEach(func(ctx context.Context) {
		if platform := checkPlatform(oc); platform != "azure" {
			e2eskipper.Skipf("Skipping these tests since the cluster platform is %q, not azure.", platform)
		}

		var err error
		mcClientSet, err = machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating the machineconfiguration client.")

		machineClient, err = machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating the machine client.")

		decoded := getDiskSetups(ctx, mcClientSet)
		framework.Logf("Found %d disk-setup MachineConfig(s) in the cluster.", len(decoded))

		// Disk setup is opt-in, and several Azure TechPreview jobs configure a layout other
		// than the one asserted here: the machine-config-operator multi-disk job, for
		// example, declares a single etcd disk on the control plane. Those clusters are not
		// what this suite describes, so run only when the full expected layout is present
		// and skip otherwise rather than failing somebody else's job.
		//
		// The cost of skipping instead of failing is that a drift between this table and the
		// install-config used by the job is silent here. It is not silent overall: the tests
		// stop reporting runs, which is visible in the job's results.
		setups = indexDiskSetups(decoded)
		missing := []string{}
		for _, expected := range expectedDisks {
			if _, ok := setups[expected.role+" "+expected.mountPath]; !ok {
				missing = append(missing, expected.role+":"+expected.mountPath)
			}
		}
		if len(missing) > 0 {
			configured := make([]string, 0, len(setups))
			for key := range setups {
				configured = append(configured, key)
			}
			sort.Strings(configured)
			e2eskipper.Skipf("Skipping these tests since the cluster was not installed with the disk layout they describe. Missing: %v. Configured: %v.",
				missing, configured)
		}
	})

	g.It("should provision, partition and mount an etcd data disk on control plane nodes", func() {
		assertDiskIsMountedAt(oc, diskSetupFor(setups, "master", "/var/lib/etcd"), "/var/lib/etcd")
	})

	g.It("should provision, partition and mount a user-defined data disk on control plane nodes", func() {
		assertDiskIsMountedAt(oc, diskSetupFor(setups, "master", "/var/lib/containers"), "/var/lib/containers")
	})

	g.It("should provision, partition and mount a user-defined data disk on compute nodes", func() {
		assertDiskIsMountedAt(oc, diskSetupFor(setups, "worker", "/var/lib/containers"), "/var/lib/containers")
	})

	g.It("should provision, partition and mount a second user-defined data disk on compute nodes", func() {
		assertDiskIsMountedAt(oc, diskSetupFor(setups, "worker", "/var/lib/kubelet"), "/var/lib/kubelet")
	})

	g.It("should attach every data disk with the configured storage account type, size and LUN", func(ctx context.Context) {
		// Machine lookups are per role, so do them once rather than once per expected disk.
		dataDisksByRole := map[string]map[string][]machinev1beta1.DataDisk{}
		for _, expected := range expectedDisks {
			if _, done := dataDisksByRole[expected.role]; !done {
				dataDisksByRole[expected.role] = azureDataDisksByNode(ctx, oc, machineClient, expected.role)
			}
		}

		for _, expected := range expectedDisks {
			setup := diskSetupFor(setups, expected.role, expected.mountPath)

			lun, err := setup.lun()
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("MachineConfig %q should partition an Azure LUN device.", setup.mcName))
			o.Expect(lun).To(o.Equal(expected.lun),
				fmt.Sprintf("MachineConfig %q should partition the LUN the install-config requested for %s.", setup.mcName, expected.mountPath))

			// Pair the disk setup with its data disk by LUN. The partition label cannot be
			// used for this: the installer labels etcd and swap partitions with the disk
			// setup's type and strips non-alphanumeric characters, so it does not generally
			// equal the data disk's nameSuffix.
			for node, dataDisks := range dataDisksByRole[expected.role] {
				var matched *machinev1beta1.DataDisk
				for i := range dataDisks {
					if dataDisks[i].Lun == lun {
						matched = &dataDisks[i]
						break
					}
				}
				o.Expect(matched).NotTo(o.BeNil(),
					fmt.Sprintf("Machine for node %q should declare a data disk at LUN %d, got %v.", node, lun, dataDisks))

				o.Expect(matched.DiskSizeGB).To(o.Equal(expected.sizeGB),
					fmt.Sprintf("Data disk at LUN %d on node %q should be %d GB.", lun, node, expected.sizeGB))
				o.Expect(string(matched.ManagedDisk.StorageAccountType)).To(o.Equal(expected.storageAccountType),
					fmt.Sprintf("Data disk at LUN %d on node %q should use storage account type %q.", lun, node, expected.storageAccountType))
			}

			// The size requested in the install-config should also be what the kernel sees,
			// which proves the disk Azure attached is the one that was asked for.
			minBytes := int64(float64(int64(expected.sizeGB)*gibiByte) * diskSizeTolerance)
			for _, node := range getNodesWithRole(oc, expected.role) {
				out, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, diskSetupDebugNamespace,
					"lsblk", "--bytes", "--nodeps", "--noheadings", "--output", "SIZE", setup.backingDevice)
				o.Expect(err).NotTo(o.HaveOccurred(),
					fmt.Sprintf("Error reading the size of %q on node %q.", setup.backingDevice, node.Name))

				observed, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
				o.Expect(err).NotTo(o.HaveOccurred(),
					fmt.Sprintf("lsblk should report a byte count for %q on node %q, got %q.", setup.backingDevice, node.Name, out))
				o.Expect(observed).To(o.BeNumerically(">=", minBytes),
					fmt.Sprintf("Device %q on node %q should be at least %d bytes for a %d GB data disk, got %d.",
						setup.backingDevice, node.Name, minBytes, expected.sizeGB, observed))
			}
		}
	})
})

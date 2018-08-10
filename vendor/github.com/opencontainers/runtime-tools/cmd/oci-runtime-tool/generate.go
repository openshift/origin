package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/generate/seccomp"
	"github.com/urfave/cli"
)

var generateFlags = []cli.Flag{
	cli.StringSliceFlag{Name: "args", Usage: "command to run in the container"},
	cli.StringSliceFlag{Name: "env", Usage: "add environment variable e.g. key=value"},
	cli.StringSliceFlag{Name: "env-file", Usage: "read in a file of environment variables"},
	cli.StringSliceFlag{Name: "hooks-poststart-add", Usage: "set command to run in poststart hooks"},
	cli.BoolFlag{Name: "hooks-poststart-remove-all", Usage: "remove all poststart hooks"},
	cli.StringSliceFlag{Name: "hooks-poststop-add", Usage: "set command to run in poststop hooks"},
	cli.BoolFlag{Name: "hooks-poststop-remove-all", Usage: "remove all poststop hooks"},
	cli.StringSliceFlag{Name: "hooks-prestart-add", Usage: "set command to run in prestart hooks"},
	cli.BoolFlag{Name: "hooks-prestart-remove-all", Usage: "remove all prestart hooks"},
	cli.StringFlag{Name: "hostname", Usage: "hostname value for the container"},
	cli.StringSliceFlag{Name: "label", Usage: "add annotations to the configuration e.g. key=value"},
	cli.StringFlag{Name: "linux-apparmor", Usage: "specifies the the apparmor profile for the container"},
	cli.IntFlag{Name: "linux-blkio-leaf-weight", Usage: "Block IO (relative leaf weight), the range is from 10 to 1000"},
	cli.StringSliceFlag{Name: "linux-blkio-leaf-weight-device", Usage: "Block IO (relative device leaf weight), e.g. major:minor:leaf-weight"},
	cli.StringSliceFlag{Name: "linux-blkio-read-bps-device", Usage: "Limit read rate (bytes per second) from a device"},
	cli.StringSliceFlag{Name: "linux-blkio-read-iops-device", Usage: "Limit read rate (IO per second) from a device"},
	cli.IntFlag{Name: "linux-blkio-weight", Usage: "Block IO (relative weight), the range is from 10 to 1000"},
	cli.StringSliceFlag{Name: "linux-blkio-weight-device", Usage: "Block IO (relative device weight), e.g. major:minor:weight"},
	cli.StringSliceFlag{Name: "linux-blkio-write-bps-device", Usage: "Limit write rate (bytes per second) to a device"},
	cli.StringSliceFlag{Name: "linux-blkio-write-iops-device", Usage: "Limit write rate (IO per second) to a device"},
	cli.StringFlag{Name: "linux-cgroups-path", Usage: "specify the path to the cgroups"},
	cli.Uint64Flag{Name: "linux-cpu-period", Usage: "the CPU period to be used for hardcapping (in usecs)"},
	cli.Uint64Flag{Name: "linux-cpu-quota", Usage: "the allowed CPU time in a given period (in usecs)"},
	cli.StringFlag{Name: "linux-cpus", Usage: "CPUs to use within the cpuset (default is to use any CPU available)"},
	cli.Uint64Flag{Name: "linux-cpu-shares", Usage: "the relative share of CPU time available to the tasks in a cgroup"},
	cli.StringSliceFlag{Name: "linux-device-add", Usage: "add a device which must be made available in the container"},
	cli.StringSliceFlag{Name: "linux-device-remove", Usage: "remove a device which must be made available in the container"},
	cli.BoolFlag{Name: "linux-device-remove-all", Usage: "remove all devices which must be made available in the container"},
	cli.StringSliceFlag{Name: "linux-device-cgroup-add", Usage: "add a device access rule"},
	cli.StringSliceFlag{Name: "linux-device-cgroup-remove", Usage: "remove a device access rule"},
	cli.BoolFlag{Name: "linux-disable-oom-kill", Usage: "disable OOM Killer"},
	cli.StringSliceFlag{Name: "linux-gidmappings", Usage: "add GIDMappings e.g HostID:ContainerID:Size"},
	cli.StringSliceFlag{Name: "linux-hugepage-limits-add", Usage: "add hugepage resource limits"},
	cli.StringSliceFlag{Name: "linux-hugepage-limits-drop", Usage: "drop hugepage resource limits"},
	cli.StringFlag{Name: "linux-intelRdt-l3CacheSchema", Usage: "specifies the schema for L3 cache id and capacity bitmask"},
	cli.StringSliceFlag{Name: "linux-masked-paths", Usage: "specifies paths can not be read inside container"},
	cli.Uint64Flag{Name: "linux-mem-kernel-limit", Usage: "kernel memory limit (in bytes)"},
	cli.Uint64Flag{Name: "linux-mem-kernel-tcp", Usage: "kernel memory limit for tcp (in bytes)"},
	cli.Uint64Flag{Name: "linux-mem-limit", Usage: "memory limit (in bytes)"},
	cli.Uint64Flag{Name: "linux-mem-reservation", Usage: "memory reservation or soft limit (in bytes)"},
	cli.StringFlag{Name: "linux-mems", Usage: "list of memory nodes in the cpuset (default is to use any available memory node)"},
	cli.Uint64Flag{Name: "linux-mem-swap", Usage: "total memory limit (memory + swap) (in bytes)"},
	cli.Uint64Flag{Name: "linux-mem-swappiness", Usage: "how aggressive the kernel will swap memory pages (Range from 0 to 100)"},
	cli.StringFlag{Name: "linux-mount-label", Usage: "selinux mount context label"},
	cli.StringSliceFlag{Name: "linux-namespace-add", Usage: "adds a namespace to the set of namespaces to create or join of the form 'ns[:path]'"},
	cli.StringSliceFlag{Name: "linux-namespace-remove", Usage: "removes a namespace from the set of namespaces to create or join of the form 'ns'"},
	cli.BoolFlag{Name: "linux-namespace-remove-all", Usage: "removes all namespaces from the set of namespaces created or joined"},
	cli.IntFlag{Name: "linux-network-classid", Usage: "specifies class identifier tagged by container's network packets"},
	cli.StringSliceFlag{Name: "linux-network-priorities", Usage: "specifies priorities of network traffic"},
	cli.IntFlag{Name: "linux-oom-score-adj", Usage: "oom_score_adj for the container"},
	cli.Int64Flag{Name: "linux-pids-limit", Usage: "maximum number of PIDs"},
	cli.StringSliceFlag{Name: "linux-readonly-paths", Usage: "specifies paths readonly inside container"},
	cli.Int64Flag{Name: "linux-realtime-period", Usage: "CPU period to be used for realtime scheduling (in usecs)"},
	cli.Int64Flag{Name: "linux-realtime-runtime", Usage: "the time realtime scheduling may use (in usecs)"},
	cli.StringFlag{Name: "linux-rootfs-propagation", Usage: "mount propagation for rootfs"},
	cli.StringFlag{Name: "linux-seccomp-allow", Usage: "specifies syscalls to respond with allow"},
	cli.StringFlag{Name: "linux-seccomp-arch", Usage: "specifies additional architectures permitted to be used for system calls"},
	cli.StringFlag{Name: "linux-seccomp-default", Usage: "specifies default action to be used for system calls and removes existing rules with specified action"},
	cli.StringFlag{Name: "linux-seccomp-default-force", Usage: "same as seccomp-default but does not remove existing rules with specified action"},
	cli.StringFlag{Name: "linux-seccomp-errno", Usage: "specifies syscalls to respond with errno"},
	cli.StringFlag{Name: "linux-seccomp-kill", Usage: "specifies syscalls to respond with kill"},
	cli.BoolFlag{Name: "linux-seccomp-only", Usage: "specifies to export just a seccomp configuration file"},
	cli.StringFlag{Name: "linux-seccomp-remove", Usage: "specifies syscalls to remove seccomp rules for"},
	cli.BoolFlag{Name: "linux-seccomp-remove-all", Usage: "removes all syscall rules from seccomp configuration"},
	cli.StringFlag{Name: "linux-seccomp-trace", Usage: "specifies syscalls to respond with trace"},
	cli.StringFlag{Name: "linux-seccomp-trap", Usage: "specifies syscalls to respond with trap"},
	cli.StringFlag{Name: "linux-selinux-label", Usage: "process selinux label"},
	cli.StringSliceFlag{Name: "linux-sysctl", Usage: "add sysctl settings e.g net.ipv4.forward=1"},
	cli.StringSliceFlag{Name: "linux-uidmappings", Usage: "add UIDMappings e.g HostID:ContainerID:Size"},
	cli.StringSliceFlag{Name: "mounts-add", Usage: "configures additional mounts inside container"},
	cli.StringSliceFlag{Name: "mounts-remove", Usage: "remove destination mountpoints from inside container"},
	cli.BoolFlag{Name: "mounts-remove-all", Usage: "remove all mounts inside container"},
	cli.StringFlag{Name: "os", Value: runtime.GOOS, Usage: "operating system the container is created for"},
	cli.StringFlag{Name: "output", Usage: "output file (defaults to stdout)"},
	cli.BoolFlag{Name: "privileged", Usage: "enable privileged container settings"},
	cli.StringSliceFlag{Name: "process-cap-add-ambient", Usage: "add Linux ambient capabilities"},
	cli.StringSliceFlag{Name: "process-cap-add-bounding", Usage: "add Linux bounding capabilities"},
	cli.StringSliceFlag{Name: "process-cap-add-effective", Usage: "add Linux effective capabilities"},
	cli.StringSliceFlag{Name: "process-cap-add-inheritable", Usage: "add Linux inheritable capabilities"},
	cli.StringSliceFlag{Name: "process-cap-add-permitted", Usage: "add Linux permitted capabilities"},
	cli.BoolFlag{Name: "process-cap-drop-all", Usage: "drop all Linux capabilities"},
	cli.StringSliceFlag{Name: "process-cap-drop-ambient", Usage: "drop Linux ambient capabilities"},
	cli.StringSliceFlag{Name: "process-cap-drop-bounding", Usage: "drop Linux bounding capabilities"},
	cli.StringSliceFlag{Name: "process-cap-drop-effective", Usage: "drop Linux effective capabilities"},
	cli.StringSliceFlag{Name: "process-cap-drop-inheritable", Usage: "drop Linux inheritable capabilities"},
	cli.StringSliceFlag{Name: "process-cap-drop-permitted", Usage: "drop Linux permitted capabilities"},
	cli.StringFlag{Name: "process-consolesize", Usage: "specifies the console size in characters (width:height)"},
	cli.StringFlag{Name: "process-cwd", Value: "/", Usage: "current working directory for the process"},
	cli.IntFlag{Name: "process-gid", Usage: "gid for the process"},
	cli.StringSliceFlag{Name: "process-groups", Usage: "supplementary groups for the process"},
	cli.BoolFlag{Name: "process-no-new-privileges", Usage: "set no new privileges bit for the container process"},
	cli.StringSliceFlag{Name: "process-rlimits-add", Usage: "specifies resource limits for processes inside the container. "},
	cli.StringSliceFlag{Name: "process-rlimits-remove", Usage: "remove specified resource limits for processes inside the container. "},
	cli.BoolFlag{Name: "process-rlimits-remove-all", Usage: "remove all resource limits for processes inside the container. "},
	cli.BoolFlag{Name: "process-terminal", Usage: "specifies whether a terminal is attached to the process"},
	cli.IntFlag{Name: "process-uid", Usage: "uid for the process"},
	cli.StringFlag{Name: "process-username", Usage: "username for the process"},
	cli.StringFlag{Name: "rootfs-path", Value: "rootfs", Usage: "path to the root filesystem"},
	cli.BoolFlag{Name: "rootfs-readonly", Usage: "make the container's rootfs readonly"},
	cli.StringSliceFlag{Name: "solaris-anet", Usage: "set up networking for Solaris application containers"},
	cli.StringFlag{Name: "solaris-capped-cpu-ncpus", Usage: "Specifies the percentage of CPU usage"},
	cli.StringFlag{Name: "solaris-capped-memory-physical", Usage: "Specifies the physical caps on the memory"},
	cli.StringFlag{Name: "solaris-capped-memory-swap", Usage: "Specifies the swap caps on the memory"},
	cli.StringFlag{Name: "solaris-limitpriv", Usage: "privilege limit"},
	cli.StringFlag{Name: "solaris-max-shm-memory", Usage: "Specifies the maximum amount of shared memory"},
	cli.StringFlag{Name: "solaris-milestone", Usage: "Specifies the SMF FMRI"},
	cli.StringFlag{Name: "template", Usage: "base template to use for creating the configuration"},
	cli.StringFlag{Name: "windows-hyperv-utilityVMPath", Usage: "specifies the path to the image used for the utility VM"},
	cli.BoolFlag{Name: "windows-ignore-flushes-during-boot", Usage: "ignore flushes during boot"},
	cli.StringSliceFlag{Name: "windows-layer-folders", Usage: "specifies a list of layer folders the container image relies on"},
	cli.StringFlag{Name: "windows-network", Usage: "specifies network for container"},
	cli.StringFlag{Name: "windows-resources-cpu", Usage: "specifies CPU for container"},
	cli.Uint64Flag{Name: "windows-resources-memory-limit", Usage: "specifies limit of memory"},
	cli.StringFlag{Name: "windows-resources-storage", Usage: "specifies storage for container"},
	cli.BoolFlag{Name: "windows-servicing", Usage: "servicing operations"},
}

var generateCommand = cli.Command{
	Name:   "generate",
	Usage:  "generate an OCI spec file",
	Flags:  generateFlags,
	Before: before,
	Action: func(context *cli.Context) error {
		// Start from the default template.
		specgen, err := generate.New(context.String("os"))
		if err != nil {
			return err
		}

		var template string
		if context.IsSet("template") {
			template = context.String("template")
		}
		if template != "" {
			specgen, err = generate.NewFromFile(template)
			if err != nil {
				return err
			}
		}

		err = setupSpec(&specgen, context)
		if err != nil {
			return err
		}

		var exportOpts generate.ExportOptions
		exportOpts.Seccomp = context.Bool("linux-seccomp-only")

		if context.IsSet("output") {
			err = specgen.SaveToFile(context.String("output"), exportOpts)
		} else {
			err = specgen.Save(os.Stdout, exportOpts)
		}
		if err != nil {
			return err
		}
		return nil
	},
}

func setupSpec(g *generate.Generator, context *cli.Context) error {
	if context.GlobalBool("host-specific") {
		g.HostSpecific = true
	}

	if len(g.Config.Version) == 0 {
		g.SetVersion(rspec.Version)
	}

	if context.IsSet("hostname") {
		g.SetHostname(context.String("hostname"))
	}

	if context.IsSet("label") {
		annotations := context.StringSlice("label")
		for _, s := range annotations {
			pair := strings.SplitN(s, "=", 2)
			if len(pair) != 2 || pair[0] == "" {
				return fmt.Errorf("incorrectly specified annotation: %s", s)
			}
			g.AddAnnotation(pair[0], pair[1])
		}
	}

	g.SetRootPath(context.String("rootfs-path"))

	if context.IsSet("rootfs-readonly") {
		g.SetRootReadonly(context.Bool("rootfs-readonly"))
	}

	if context.IsSet("process-uid") {
		g.SetProcessUID(uint32(context.Int("process-uid")))
	}

	if context.IsSet("process-username") {
		g.SetProcessUsername(context.String("process-username"))
	}

	if context.IsSet("process-gid") {
		g.SetProcessGID(uint32(context.Int("process-gid")))
	}

	if context.IsSet("linux-selinux-label") {
		g.SetProcessSelinuxLabel(context.String("linux-selinux-label"))
	}

	g.SetProcessCwd(context.String("process-cwd"))

	if context.IsSet("linux-apparmor") {
		g.SetProcessApparmorProfile(context.String("linux-apparmor"))
	}

	if context.IsSet("process-no-new-privileges") {
		g.SetProcessNoNewPrivileges(context.Bool("process-no-new-privileges"))
	}

	if context.IsSet("process-terminal") {
		g.SetProcessTerminal(context.Bool("process-terminal"))
	}

	if context.IsSet("args") {
		g.SetProcessArgs(context.StringSlice("args"))
	}

	{
		envs, err := readKVStrings(context.StringSlice("env-file"), context.StringSlice("env"))
		if err != nil {
			return err
		}

		for _, env := range envs {
			name, value, err := parseEnv(env)
			if err != nil {
				return err
			}
			g.AddProcessEnv(name, value)
		}
	}

	if context.IsSet("process-groups") {
		groups := context.StringSlice("process-groups")
		for _, group := range groups {
			groupID, err := strconv.Atoi(group)
			if err != nil {
				return err
			}
			g.AddProcessAdditionalGid(uint32(groupID))
		}
	}

	if context.IsSet("linux-cgroups-path") {
		g.SetLinuxCgroupsPath(context.String("linux-cgroups-path"))
	}

	if context.IsSet("linux-masked-paths") {
		paths := context.StringSlice("linux-masked-paths")
		for _, path := range paths {
			g.AddLinuxMaskedPaths(path)
		}
	}

	if context.IsSet("linux-device-cgroup-add") {
		devices := context.StringSlice("linux-device-cgroup-add")
		for _, device := range devices {
			dev, err := parseLinuxResourcesDeviceAccess(device, g)
			if err != nil {
				return err
			}
			g.AddLinuxResourcesDevice(dev.Allow, dev.Type, dev.Major, dev.Minor, dev.Access)
		}
	}

	if context.IsSet("linux-device-cgroup-remove") {
		devices := context.StringSlice("linux-device-cgroup-remove")
		for _, device := range devices {
			dev, err := parseLinuxResourcesDeviceAccess(device, g)
			if err != nil {
				return err
			}
			g.RemoveLinuxResourcesDevice(dev.Allow, dev.Type, dev.Major, dev.Minor, dev.Access)
		}
	}

	if context.IsSet("linux-readonly-paths") {
		paths := context.StringSlice("linux-readonly-paths")
		for _, path := range paths {
			g.AddLinuxReadonlyPaths(path)
		}
	}

	if context.IsSet("linux-mount-label") {
		g.SetLinuxMountLabel(context.String("linux-mount-label"))
	}

	if context.IsSet("linux-sysctl") {
		sysctls := context.StringSlice("linux-sysctl")
		for _, s := range sysctls {
			pair := strings.Split(s, "=")
			if len(pair) != 2 {
				return fmt.Errorf("incorrectly specified sysctl: %s", s)
			}
			g.AddLinuxSysctl(pair[0], pair[1])
		}
	}

	g.SetupPrivileged(context.Bool("privileged"))

	if context.Bool("process-cap-drop-all") {
		g.ClearProcessCapabilities()
	}

	if context.IsSet("process-cap-add-ambient") {
		addCaps := context.StringSlice("process-cap-add-ambient")
		for _, cap := range addCaps {
			if err := g.AddProcessCapabilityAmbient(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-add-bounding") {
		addCaps := context.StringSlice("process-cap-add-bounding")
		for _, cap := range addCaps {
			if err := g.AddProcessCapabilityBounding(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-add-effective") {
		addCaps := context.StringSlice("process-cap-add-effective")
		for _, cap := range addCaps {
			if err := g.AddProcessCapabilityEffective(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-add-inheritable") {
		addCaps := context.StringSlice("process-cap-add-inheritable")
		for _, cap := range addCaps {
			if err := g.AddProcessCapabilityInheritable(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-add-permitted") {
		addCaps := context.StringSlice("process-cap-add-permitted")
		for _, cap := range addCaps {
			if err := g.AddProcessCapabilityPermitted(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-drop-ambient") {
		dropCaps := context.StringSlice("process-cap-drop-ambient")
		for _, cap := range dropCaps {
			if err := g.DropProcessCapabilityAmbient(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-drop-bounding") {
		dropCaps := context.StringSlice("process-cap-drop-bounding")
		for _, cap := range dropCaps {
			if err := g.DropProcessCapabilityBounding(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-drop-effective") {
		dropCaps := context.StringSlice("process-cap-drop-effective")
		for _, cap := range dropCaps {
			if err := g.DropProcessCapabilityEffective(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-drop-inheritable") {
		dropCaps := context.StringSlice("process-cap-drop-inheritable")
		for _, cap := range dropCaps {
			if err := g.DropProcessCapabilityInheritable(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-cap-drop-permitted") {
		dropCaps := context.StringSlice("process-cap-drop-permitted")
		for _, cap := range dropCaps {
			if err := g.DropProcessCapabilityPermitted(cap); err != nil {
				return err
			}
		}
	}

	if context.IsSet("process-consolesize") {
		consoleSize := context.String("process-consolesize")
		width, height, err := parseConsoleSize(consoleSize)
		if err != nil {
			return err
		}
		g.SetProcessConsoleSize(width, height)
	}

	var uidMaps, gidMaps []string

	if context.IsSet("linux-uidmappings") {
		uidMaps = context.StringSlice("linux-uidmappings")
	}

	if context.IsSet("linux-gidmappings") {
		gidMaps = context.StringSlice("linux-gidmappings")
	}

	// Add default user namespace.
	if len(uidMaps) > 0 || len(gidMaps) > 0 {
		g.AddOrReplaceLinuxNamespace("user", "")
	}

	if context.IsSet("mounts-remove-all") {
		g.ClearMounts()
	}

	if context.IsSet("mounts-remove") {
		mounts := context.StringSlice("mounts-remove")
		for _, mount := range mounts {
			g.RemoveMount(mount)
		}
	}

	if context.IsSet("mounts-add") {
		mounts := context.StringSlice("mounts-add")
		for _, mount := range mounts {
			mnt := rspec.Mount{}
			if err := json.Unmarshal([]byte(mount), &mnt); err != nil {
				return err
			}
			g.AddMount(mnt)
		}
	}

	if context.IsSet("hooks-poststart-remove-all") {
		g.ClearPostStartHooks()
	}

	if context.IsSet("hooks-poststart-add") {
		postStartHooks := context.StringSlice("hooks-poststart-add")
		for _, hook := range postStartHooks {
			tmpHook := rspec.Hook{}
			if err := json.Unmarshal([]byte(hook), &tmpHook); err != nil {
				return err
			}
			if err := g.AddPostStartHook(tmpHook); err != nil {
				return err
			}
		}
	}

	if context.IsSet("hooks-poststop-remove-all") {
		g.ClearPostStopHooks()
	}

	if context.IsSet("hooks-poststop-add") {
		postStopHooks := context.StringSlice("hooks-poststop-add")
		for _, hook := range postStopHooks {
			tmpHook := rspec.Hook{}
			if err := json.Unmarshal([]byte(hook), &tmpHook); err != nil {
				return err
			}
			if err := g.AddPostStopHook(tmpHook); err != nil {
				return err
			}
		}
	}

	if context.IsSet("hooks-prestart-remove-all") {
		g.ClearPreStartHooks()
	}

	if context.IsSet("hooks-prestart-add") {
		preStartHooks := context.StringSlice("hooks-prestart-add")
		for _, hook := range preStartHooks {
			tmpHook := rspec.Hook{}
			if err := json.Unmarshal([]byte(hook), &tmpHook); err != nil {
				return err
			}
			if err := g.AddPreStartHook(tmpHook); err != nil {
				return err
			}
		}
	}

	if context.IsSet("linux-rootfs-propagation") {
		rp := context.String("linux-rootfs-propagation")
		if err := g.SetLinuxRootPropagation(rp); err != nil {
			return err
		}
	}

	for _, uidMap := range uidMaps {
		hid, cid, size, err := parseIDMapping(uidMap)
		if err != nil {
			return err
		}

		g.AddLinuxUIDMapping(hid, cid, size)
	}

	for _, gidMap := range gidMaps {
		hid, cid, size, err := parseIDMapping(gidMap)
		if err != nil {
			return err
		}

		g.AddLinuxGIDMapping(hid, cid, size)
	}

	if context.IsSet("linux-disable-oom-kill") {
		g.SetLinuxResourcesMemoryDisableOOMKiller(context.Bool("linux-disable-oom-kill"))
	}

	if context.IsSet("linux-oom-score-adj") {
		g.SetProcessOOMScoreAdj(context.Int("linux-oom-score-adj"))
	}

	if context.IsSet("linux-blkio-leaf-weight") {
		g.SetLinuxResourcesBlockIOLeafWeight(uint16(context.Uint64("linux-blkio-leaf-weight")))
	}

	if context.IsSet("linux-blkio-leaf-weight-device") {
		devLeafWeight := context.StringSlice("linux-blkio-leaf-weight-device")
		for _, v := range devLeafWeight {
			major, minor, leafWeight, err := parseDeviceWeight(v)
			if err != nil {
				return err
			}
			if leafWeight == -1 {
				g.DropLinuxResourcesBlockIOLeafWeightDevice(major, minor)
			} else {
				g.AddLinuxResourcesBlockIOLeafWeightDevice(major, minor, uint16(leafWeight))
			}
		}
	}

	if context.IsSet("linux-blkio-read-bps-device") {
		throttleDevices := context.StringSlice("linux-blkio-read-bps-device")
		for _, v := range throttleDevices {
			major, minor, rate, err := parseThrottleDevice(v)
			if err != nil {
				return err
			}
			if rate == -1 {
				g.DropLinuxResourcesBlockIOThrottleReadBpsDevice(major, minor)
			} else {
				g.AddLinuxResourcesBlockIOThrottleReadBpsDevice(major, minor, uint64(rate))
			}
		}
	}

	if context.IsSet("linux-blkio-read-iops-device") {
		throttleDevices := context.StringSlice("linux-blkio-read-iops-device")
		for _, v := range throttleDevices {
			major, minor, rate, err := parseThrottleDevice(v)
			if err != nil {
				return err
			}
			if rate == -1 {
				g.DropLinuxResourcesBlockIOThrottleReadIOPSDevice(major, minor)
			} else {
				g.AddLinuxResourcesBlockIOThrottleReadIOPSDevice(major, minor, uint64(rate))
			}
		}
	}

	if context.IsSet("linux-blkio-weight") {
		g.SetLinuxResourcesBlockIOWeight(uint16(context.Uint64("linux-blkio-weight")))
	}

	if context.IsSet("linux-blkio-weight-device") {
		devWeight := context.StringSlice("linux-blkio-weight-device")
		for _, v := range devWeight {
			major, minor, weight, err := parseDeviceWeight(v)
			if err != nil {
				return err
			}
			if weight == -1 {
				g.DropLinuxResourcesBlockIOWeightDevice(major, minor)
			} else {
				g.AddLinuxResourcesBlockIOWeightDevice(major, minor, uint16(weight))
			}
		}
	}

	if context.IsSet("linux-blkio-write-bps-device") {
		throttleDevices := context.StringSlice("linux-blkio-write-bps-device")
		for _, v := range throttleDevices {
			major, minor, rate, err := parseThrottleDevice(v)
			if err != nil {
				return err
			}
			if rate == -1 {
				g.DropLinuxResourcesBlockIOThrottleWriteBpsDevice(major, minor)
			} else {
				g.AddLinuxResourcesBlockIOThrottleWriteBpsDevice(major, minor, uint64(rate))
			}
		}
	}

	if context.IsSet("linux-blkio-write-iops-device") {
		throttleDevices := context.StringSlice("linux-blkio-write-iops-device")
		for _, v := range throttleDevices {
			major, minor, rate, err := parseThrottleDevice(v)
			if err != nil {
				return err
			}
			if rate == -1 {
				g.DropLinuxResourcesBlockIOThrottleWriteIOPSDevice(major, minor)
			} else {
				g.AddLinuxResourcesBlockIOThrottleWriteIOPSDevice(major, minor, uint64(rate))
			}
		}
	}

	if context.IsSet("linux-cpu-shares") {
		g.SetLinuxResourcesCPUShares(context.Uint64("linux-cpu-shares"))
	}

	if context.IsSet("linux-cpu-period") {
		g.SetLinuxResourcesCPUPeriod(context.Uint64("linux-cpu-period"))
	}

	if context.IsSet("linux-cpu-quota") {
		g.SetLinuxResourcesCPUQuota(context.Int64("linux-cpu-quota"))
	}

	if context.IsSet("linux-realtime-runtime") {
		g.SetLinuxResourcesCPURealtimeRuntime(context.Int64("linux-realtime-runtime"))
	}

	if context.IsSet("linux-pids-limit") {
		g.SetLinuxResourcesPidsLimit(context.Int64("linux-pids-limit"))
	}

	if context.IsSet("linux-realtime-period") {
		g.SetLinuxResourcesCPURealtimePeriod(context.Uint64("linux-realtime-period"))
	}

	if context.IsSet("linux-cpus") {
		g.SetLinuxResourcesCPUCpus(context.String("linux-cpus"))
	}

	if context.IsSet("linux-hugepage-limits-add") {
		pageList := context.StringSlice("linux-hugepage-limits-add")
		for _, v := range pageList {
			pagesize, limit, err := parseHugepageLimit(v)
			if err != nil {
				return err
			}
			g.AddLinuxResourcesHugepageLimit(pagesize, limit)
		}
	}

	if context.IsSet("linux-hugepage-limits-drop") {
		pageList := context.StringSlice("linux-hugepage-limits-drop")
		for _, v := range pageList {
			g.DropLinuxResourcesHugepageLimit(v)
		}
	}

	if context.IsSet("linux-intelRdt-l3CacheSchema") {
		g.SetLinuxIntelRdtL3CacheSchema(context.String("linux-intelRdt-l3CacheSchema"))
	}

	if context.IsSet("linux-mems") {
		g.SetLinuxResourcesCPUMems(context.String("linux-mems"))
	}

	if context.IsSet("linux-mem-limit") {
		g.SetLinuxResourcesMemoryLimit(context.Int64("linux-mem-limit"))
	}

	if context.IsSet("linux-mem-reservation") {
		g.SetLinuxResourcesMemoryReservation(context.Int64("linux-mem-reservation"))
	}

	if context.IsSet("linux-mem-swap") {
		g.SetLinuxResourcesMemorySwap(context.Int64("linux-mem-swap"))
	}

	if context.IsSet("linux-mem-kernel-limit") {
		g.SetLinuxResourcesMemoryKernel(context.Int64("linux-mem-kernel-limit"))
	}

	if context.IsSet("linux-mem-kernel-tcp") {
		g.SetLinuxResourcesMemoryKernelTCP(context.Int64("linux-mem-kernel-tcp"))
	}

	if context.IsSet("linux-mem-swappiness") {
		g.SetLinuxResourcesMemorySwappiness(context.Uint64("linux-mem-swappiness"))
	}

	if context.IsSet("linux-network-classid") {
		g.SetLinuxResourcesNetworkClassID(uint32(context.Int("linux-network-classid")))
	}

	if context.IsSet("linux-network-priorities") {
		priorities := context.StringSlice("linux-network-priorities")
		for _, p := range priorities {
			name, priority, err := parseNetworkPriority(p)
			if err != nil {
				return err
			}
			if priority == -1 {
				g.DropLinuxResourcesNetworkPriorities(name)
			} else {
				g.AddLinuxResourcesNetworkPriorities(name, uint32(priority))
			}
		}
	}

	if context.Bool("linux-namespace-remove-all") {
		g.ClearLinuxNamespaces()
	}

	if context.IsSet("linux-namespace-add") {
		namespaces := context.StringSlice("linux-namespace-add")
		for _, ns := range namespaces {
			name, path, err := parseNamespace(ns)
			if err != nil {
				return err
			}
			if err := g.AddOrReplaceLinuxNamespace(name, path); err != nil {
				return err
			}
		}
	}

	if context.IsSet("linux-namespace-remove") {
		namespaces := context.StringSlice("linux-namespace-remove")
		for _, name := range namespaces {
			if err := g.RemoveLinuxNamespace(name); err != nil {
				return err
			}
		}
	}

	if context.Bool("process-rlimits-remove-all") {
		g.ClearProcessRlimits()
	}

	if context.IsSet("process-rlimits-add") {
		rlimits := context.StringSlice("process-rlimits-add")
		for _, rlimit := range rlimits {
			rType, rHard, rSoft, err := parseRlimit(rlimit)
			if err != nil {
				return err
			}
			g.AddProcessRlimits(rType, rHard, rSoft)
		}
	}

	if context.IsSet("process-rlimits-remove") {
		rlimits := context.StringSlice("process-rlimits-remove")
		for _, rlimit := range rlimits {
			g.RemoveProcessRlimits(rlimit)
		}
	}

	if context.Bool("linux-device-remove-all") {
		g.ClearLinuxDevices()
	}

	if context.IsSet("linux-device-add") {
		devices := context.StringSlice("linux-device-add")
		for _, deviceArg := range devices {
			dev, err := parseDevice(deviceArg, g)
			if err != nil {
				return err
			}
			g.AddDevice(dev)
		}
	}

	if context.IsSet("linux-device-remove") {
		devices := context.StringSlice("linux-device-remove")
		for _, device := range devices {
			g.RemoveDevice(device)
		}
	}

	if context.IsSet("solaris-anet") {
		anets := context.StringSlice("solaris-anet")
		for _, anet := range anets {
			tmpAnet := rspec.SolarisAnet{}
			if err := json.Unmarshal([]byte(anet), &tmpAnet); err != nil {
				return err
			}

			g.AddSolarisAnet(tmpAnet)
		}
	}

	if context.IsSet("solaris-capped-cpu-ncpus") {
		g.SetSolarisCappedCPUNcpus(context.String("solaris-capped-cpu-ncpus"))
	}

	if context.IsSet("solaris-capped-memory-physical") {
		g.SetSolarisCappedMemoryPhysical(context.String("solaris-capped-memory-physical"))
	}

	if context.IsSet("solaris-capped-memory-swap") {
		g.SetSolarisCappedMemorySwap(context.String("solaris-capped-memory-swap"))
	}

	if context.IsSet("solaris-limitpriv") {
		g.SetSolarisLimitPriv(context.String("solaris-limitpriv"))
	}

	if context.IsSet("solaris-max-shm-memory") {
		g.SetSolarisMaxShmMemory(context.String("solaris-max-shm-memory"))
	}

	if context.IsSet("solaris-milestone") {
		g.SetSolarisMilestone(context.String("solaris-milestone"))
	}

	if context.IsSet("windows-hyperv-utilityVMPath") {
		g.SetWindowsHypervUntilityVMPath(context.String("windows-hyperv-utilityVMPath"))
	}

	if context.IsSet("windows-ignore-flushes-during-boot") {
		g.SetWinodwsIgnoreFlushesDuringBoot(context.Bool("windows-ignore-flushes-during-boot"))
	}

	if context.IsSet("windows-layer-folders") {
		folders := context.StringSlice("windows-layer-folders")
		for _, folder := range folders {
			g.AddWindowsLayerFolders(folder)
		}
	}

	if context.IsSet("windows-network") {
		network := context.String("windows-network")
		tmpNetwork := rspec.WindowsNetwork{}
		if err := json.Unmarshal([]byte(network), &tmpNetwork); err != nil {
			return err
		}
		g.SetWindowsNetwork(tmpNetwork)
	}

	if context.IsSet("windows-resources-cpu") {
		cpu := context.String("windows-resources-cpu")
		tmpCPU := rspec.WindowsCPUResources{}
		if err := json.Unmarshal([]byte(cpu), &tmpCPU); err != nil {
			return err
		}
		g.SetWindowsResourcesCPU(tmpCPU)
	}

	if context.IsSet("windows-resources-memory-limit") {
		limit := context.Uint64("windows-resources-memory-limit")
		g.SetWindowsResourcesMemoryLimit(limit)
	}

	if context.IsSet("windows-resources-storage") {
		storage := context.String("windows-resources-storage")
		tmpStorage := rspec.WindowsStorageResources{}
		if err := json.Unmarshal([]byte(storage), &tmpStorage); err != nil {
			return err
		}
		g.SetWindowsResourcesStorage(tmpStorage)
	}

	if context.IsSet("windows-servicing") {
		g.SetWinodwsServicing(context.Bool("windows-servicing"))
	}

	err := addSeccomp(context, g)
	return err
}

func parseConsoleSize(consoleSize string) (uint, uint, error) {
	size := strings.Split(consoleSize, ":")
	if len(size) != 2 {
		return 0, 0, fmt.Errorf("invalid consolesize value: %s", consoleSize)
	}

	width, err := strconv.Atoi(size[0])
	if err != nil {
		return 0, 0, err
	}

	height, err := strconv.Atoi(size[1])
	if err != nil {
		return 0, 0, err
	}

	return uint(width), uint(height), nil
}

func parseIDMapping(idms string) (uint32, uint32, uint32, error) {
	idm := strings.Split(idms, ":")
	if len(idm) != 3 {
		return 0, 0, 0, fmt.Errorf("idmappings error: %s", idms)
	}

	hid, err := strconv.Atoi(idm[0])
	if err != nil {
		return 0, 0, 0, err
	}

	cid, err := strconv.Atoi(idm[1])
	if err != nil {
		return 0, 0, 0, err
	}

	size, err := strconv.Atoi(idm[2])
	if err != nil {
		return 0, 0, 0, err
	}

	return uint32(hid), uint32(cid), uint32(size), nil
}

func parseHugepageLimit(pageLimit string) (string, uint64, error) {
	pl := strings.Split(pageLimit, ":")
	if len(pl) != 2 {
		return "", 0, fmt.Errorf("invalid format: %s", pageLimit)
	}

	limit, err := strconv.Atoi(pl[1])
	if err != nil {
		return "", 0, err
	}

	return pl[0], uint64(limit), nil
}

func parseNetworkPriority(np string) (string, int32, error) {
	var err error

	parts := strings.Split(np, ":")
	if len(parts) != 2 || parts[0] == "" {
		return "", 0, fmt.Errorf("invalid value %v for --linux-network-priorities", np)
	}
	priority, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, err
	}

	return parts[0], int32(priority), nil
}

func parseRlimit(rlimit string) (string, uint64, uint64, error) {
	parts := strings.Split(rlimit, ":")
	if len(parts) != 3 || parts[0] == "" {
		return "", 0, 0, fmt.Errorf("invalid rlimits value: %s", rlimit)
	}

	hard, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, err
	}

	soft, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, err
	}

	return parts[0], uint64(hard), uint64(soft), nil
}

func parseNamespace(ns string) (string, string, error) {
	parts := strings.SplitN(ns, ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid namespace value: %s", ns)
	}

	nsType := parts[0]
	nsPath := ""

	if len(parts) == 2 {
		nsPath = parts[1]
	}

	return nsType, nsPath, nil
}

var deviceType = map[string]bool{
	"b": true, // a block (buffered) special file
	"c": true, // a character special file
	"u": true, // a character (unbuffered) special file
	"p": true, // a FIFO
}

// parseDevice takes the raw string passed with the --device-add flag
func parseDevice(device string, g *generate.Generator) (rspec.LinuxDevice, error) {
	dev := rspec.LinuxDevice{}

	// The required part and optional part are separated by ":"
	argsParts := strings.Split(device, ":")
	if len(argsParts) < 4 {
		return dev, fmt.Errorf("Incomplete device arguments: %s", device)
	}
	requiredPart := argsParts[0:4]
	optionalPart := argsParts[4:]

	// The required part must contain type, major, minor, and path
	dev.Type = requiredPart[0]
	if !deviceType[dev.Type] {
		return dev, fmt.Errorf("Invalid device type: %s", dev.Type)
	}

	i, err := strconv.ParseInt(requiredPart[1], 10, 64)
	if err != nil {
		return dev, err
	}
	dev.Major = i

	i, err = strconv.ParseInt(requiredPart[2], 10, 64)
	if err != nil {
		return dev, err
	}
	dev.Minor = i
	dev.Path = requiredPart[3]

	// The optional part include all optional property
	for _, s := range optionalPart {
		parts := strings.SplitN(s, "=", 2)

		if len(parts) != 2 {
			return dev, fmt.Errorf("Incomplete device arguments: %s", s)
		}

		name, value := parts[0], parts[1]

		switch name {
		case "fileMode":
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return dev, err
			}
			mode := os.FileMode(i)
			dev.FileMode = &mode
		case "uid":
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return dev, err
			}
			uid := uint32(i)
			dev.UID = &uid

		case "gid":
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return dev, err
			}
			gid := uint32(i)
			dev.GID = &gid
		default:
			return dev, fmt.Errorf("'%s' is not supported by device section", name)
		}
	}

	return dev, nil
}

var cgroupDeviceType = map[string]bool{
	"a": true, // all
	"b": true, // block device
	"c": true, // character device
}
var cgroupDeviceAccess = map[string]bool{
	"r": true, //read
	"w": true, //write
	"m": true, //mknod
}

// parseLinuxResourcesDeviceAccess parses the raw string passed with the --device-access-add flag
func parseLinuxResourcesDeviceAccess(device string, g *generate.Generator) (rspec.LinuxDeviceCgroup, error) {
	var allow bool
	var devType, access string
	var major, minor *int64

	argsParts := strings.Split(device, ",")

	switch argsParts[0] {
	case "allow":
		allow = true
	case "deny":
		allow = false
	default:
		return rspec.LinuxDeviceCgroup{},
			fmt.Errorf("Only 'allow' and 'deny' are allowed in the first field of device-access-add: %s", device)
	}

	for _, s := range argsParts[1:] {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return rspec.LinuxDeviceCgroup{}, fmt.Errorf("Incomplete device-access-add arguments: %s", s)
		}
		name, value := parts[0], parts[1]

		switch name {
		case "type":
			if !cgroupDeviceType[value] {
				return rspec.LinuxDeviceCgroup{}, fmt.Errorf("Invalid device type in device-access-add: %s", value)
			}
			devType = value
		case "major":
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return rspec.LinuxDeviceCgroup{}, err
			}
			major = &i
		case "minor":
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return rspec.LinuxDeviceCgroup{}, err
			}
			minor = &i
		case "access":
			for _, c := range strings.Split(value, "") {
				if !cgroupDeviceAccess[c] {
					return rspec.LinuxDeviceCgroup{}, fmt.Errorf("Invalid device access in device-access-add: %s", c)
				}
			}
			access = value
		}
	}
	return rspec.LinuxDeviceCgroup{
		Allow:  allow,
		Type:   devType,
		Major:  major,
		Minor:  minor,
		Access: access,
	}, nil
}

func addSeccomp(context *cli.Context, g *generate.Generator) error {
	if context.Bool("linux-seccomp-remove-all") {
		err := g.RemoveAllSeccompRules()
		if err != nil {
			return err
		}
	}

	// Set the DefaultAction of seccomp
	if context.IsSet("linux-seccomp-default") {
		seccompDefault := context.String("linux-seccomp-default")
		err := g.SetDefaultSeccompAction(seccompDefault)
		if err != nil {
			return err
		}
	} else if context.IsSet("linux-seccomp-default-force") {
		seccompDefaultForced := context.String("linux-seccomp-default-force")
		err := g.SetDefaultSeccompActionForce(seccompDefaultForced)
		if err != nil {
			return err
		}
	}

	// Add the additional architectures permitted to be used for system calls
	if context.IsSet("linux-seccomp-arch") {
		seccompArch := context.String("linux-seccomp-arch")
		architectureArgs := strings.Split(seccompArch, ",")
		for _, arg := range architectureArgs {
			err := g.SetSeccompArchitecture(arg)
			if err != nil {
				return err
			}
		}
	}

	if context.IsSet("linux-seccomp-errno") {
		err := seccompSet(context, "errno", g)
		if err != nil {
			return err
		}
	}

	if context.IsSet("linux-seccomp-kill") {
		err := seccompSet(context, "kill", g)
		if err != nil {
			return err
		}
	}

	if context.IsSet("linux-seccomp-trace") {
		err := seccompSet(context, "trace", g)
		if err != nil {
			return err
		}
	}

	if context.IsSet("linux-seccomp-trap") {
		err := seccompSet(context, "trap", g)
		if err != nil {
			return err
		}
	}

	if context.IsSet("linux-seccomp-allow") {
		err := seccompSet(context, "allow", g)
		if err != nil {
			return err
		}
	}

	if context.IsSet("linux-seccomp-remove") {
		seccompRemove := context.String("linux-seccomp-remove")
		err := g.RemoveSeccompRule(seccompRemove)
		if err != nil {
			return err
		}
	}

	return nil
}

func seccompSet(context *cli.Context, seccompFlag string, g *generate.Generator) error {
	flagInput := context.String("linux-seccomp-" + seccompFlag)
	flagArgs := strings.Split(flagInput, ",")
	setSyscallArgsSlice := []seccomp.SyscallOpts{}
	for _, flagArg := range flagArgs {
		comparisonArgs := strings.Split(flagArg, ":")
		if len(comparisonArgs) == 5 {
			setSyscallArgs := seccomp.SyscallOpts{
				Action:   seccompFlag,
				Syscall:  comparisonArgs[0],
				Index:    comparisonArgs[1],
				Value:    comparisonArgs[2],
				ValueTwo: comparisonArgs[3],
				Operator: comparisonArgs[4],
			}
			setSyscallArgsSlice = append(setSyscallArgsSlice, setSyscallArgs)
		} else if len(comparisonArgs) == 1 {
			setSyscallArgs := seccomp.SyscallOpts{
				Action:  seccompFlag,
				Syscall: comparisonArgs[0],
			}
			setSyscallArgsSlice = append(setSyscallArgsSlice, setSyscallArgs)
		} else {
			return fmt.Errorf("invalid syscall argument formatting %v", comparisonArgs)
		}

		for _, r := range setSyscallArgsSlice {
			err := g.SetSyscallAction(r)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// readKVStrings reads a file of line terminated key=value pairs, and overrides any keys
// present in the file with additional pairs specified in the override parameter
//
// This function is copied from github.com/docker/docker/runconfig/opts/parse.go
func readKVStrings(files []string, override []string) ([]string, error) {
	envVariables := []string{}
	for _, ef := range files {
		parsedVars, err := parseEnvFile(ef)
		if err != nil {
			return nil, err
		}
		envVariables = append(envVariables, parsedVars...)
	}
	// parse the '-e' and '--env' after, to allow override
	envVariables = append(envVariables, override...)

	return envVariables, nil
}

// parseEnv splits a given environment variable (of the form name=value) into
// (name, value). An error is returned if there is no "=" in the line or if the
// name is empty.
func parseEnv(env string) (string, string, error) {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("environment variable must contain '=': %s", env)
	}

	name, value := parts[0], parts[1]
	if name == "" {
		return "", "", fmt.Errorf("environment variable must have non-empty name: %s", env)
	}
	return name, value, nil
}

// parseEnvFile reads a file with environment variables enumerated by lines
//
// ``Environment variable names used by the utilities in the Shell and
// Utilities volume of IEEE Std 1003.1-2001 consist solely of uppercase
// letters, digits, and the '_' (underscore) from the characters defined in
// Portable Character Set and do not begin with a digit. *But*, other
// characters may be permitted by an implementation; applications shall
// tolerate the presence of such names.''
// -- http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html
//
// As of #16585, it's up to application inside docker to validate or not
// environment variables, that's why we just strip leading whitespace and
// nothing more.
//
// This function is copied from github.com/docker/docker/runconfig/opts/envfile.go
func parseEnvFile(filename string) ([]string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return []string{}, err
	}
	defer fh.Close()

	lines := []string{}
	scanner := bufio.NewScanner(fh)
	currentLine := 0
	utf8bom := []byte{0xEF, 0xBB, 0xBF}
	for scanner.Scan() {
		scannedBytes := scanner.Bytes()
		if !utf8.Valid(scannedBytes) {
			return []string{}, fmt.Errorf("env file %s contains invalid utf8 bytes at line %d: %v", filename, currentLine+1, scannedBytes)
		}
		// We trim UTF8 BOM
		if currentLine == 0 {
			scannedBytes = bytes.TrimPrefix(scannedBytes, utf8bom)
		}
		// trim the line from all leading whitespace first
		line := strings.TrimLeftFunc(string(scannedBytes), unicode.IsSpace)
		currentLine++
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			data := strings.SplitN(line, "=", 2)

			// trim the front of a variable, but nothing else
			variable := strings.TrimLeft(data[0], whiteSpaces)
			if strings.ContainsAny(variable, whiteSpaces) {
				return []string{}, ErrBadEnvVariable{fmt.Sprintf("variable '%s' has white spaces", variable)}
			}

			if len(data) > 1 {

				// pass the value through, no trimming
				lines = append(lines, fmt.Sprintf("%s=%s", variable, data[1]))
			} else {
				// if only a pass-through variable is given, clean it up.
				lines = append(lines, fmt.Sprintf("%s=%s", strings.TrimSpace(line), os.Getenv(line)))
			}
		}
	}
	return lines, scanner.Err()
}

var whiteSpaces = " \t"

// ErrBadEnvVariable typed error for bad environment variable
type ErrBadEnvVariable struct {
	msg string
}

func (e ErrBadEnvVariable) Error() string {
	return fmt.Sprintf("poorly formatted environment: %s", e.msg)
}

func parseDeviceWeight(weightDevice string) (int64, int64, int16, error) {
	list := strings.Split(weightDevice, ":")
	if len(list) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid format: %s", weightDevice)
	}

	major, err := strconv.Atoi(list[0])
	if err != nil {
		return 0, 0, 0, err
	}

	minor, err := strconv.Atoi(list[1])
	if err != nil {
		return 0, 0, 0, err
	}

	weight, err := strconv.Atoi(list[2])
	if err != nil {
		return 0, 0, 0, err
	}

	return int64(major), int64(minor), int16(weight), nil
}

func parseThrottleDevice(throttleDevice string) (int64, int64, int64, error) {
	list := strings.Split(throttleDevice, ":")
	if len(list) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid format: %s", throttleDevice)
	}

	major, err := strconv.Atoi(list[0])
	if err != nil {
		return 0, 0, 0, err
	}

	minor, err := strconv.Atoi(list[1])
	if err != nil {
		return 0, 0, 0, err
	}

	rate, err := strconv.Atoi(list[2])
	if err != nil {
		return 0, 0, 0, err
	}

	return int64(major), int64(minor), int64(rate), nil
}

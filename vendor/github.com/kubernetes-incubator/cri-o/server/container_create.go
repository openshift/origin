package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	dockermounts "github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/symlink"
	"github.com/kubernetes-incubator/cri-o/lib"
	"github.com/kubernetes-incubator/cri-o/lib/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/kubernetes-incubator/cri-o/pkg/annotations"
	"github.com/kubernetes-incubator/cri-o/pkg/storage"
	"github.com/kubernetes-incubator/cri-o/server/apparmor"
	"github.com/kubernetes-incubator/cri-o/server/seccomp"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/user"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	seccompUnconfined      = "unconfined"
	seccompRuntimeDefault  = "runtime/default"
	seccompDockerDefault   = "docker/default"
	seccompLocalhostPrefix = "localhost/"

	scopePrefix           = "crio"
	defaultCgroupfsParent = "/crio"
	defaultSystemdParent  = "system.slice"
)

type orderedMounts []rspec.Mount

// Len returns the number of mounts. Used in sorting.
func (m orderedMounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m orderedMounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m orderedMounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m orderedMounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

func addOCIBindMounts(mountLabel string, containerConfig *pb.ContainerConfig, specgen *generate.Generator) ([]oci.ContainerVolume, []rspec.Mount, error) {
	volumes := []oci.ContainerVolume{}
	ociMounts := []rspec.Mount{}
	mounts := containerConfig.GetMounts()
	for _, mount := range mounts {
		dest := mount.ContainerPath
		if dest == "" {
			return nil, nil, fmt.Errorf("Mount.ContainerPath is empty")
		}

		src := mount.HostPath
		if src == "" {
			return nil, nil, fmt.Errorf("Mount.HostPath is empty")
		}

		if _, err := os.Stat(src); err != nil && os.IsNotExist(err) {
			if err1 := os.MkdirAll(src, 0644); err1 != nil {
				return nil, nil, fmt.Errorf("Failed to mkdir %s: %s", src, err)
			}
		}

		src, err := resolveSymbolicLink(src)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve symlink %q: %v", src, err)
		}

		options := []string{"rw"}
		if mount.Readonly {
			options = []string{"ro"}
		}
		options = append(options, "rbind")

		// mount propagation
		mountInfos, err := dockermounts.GetMounts()
		if err != nil {
			return nil, nil, err
		}
		switch mount.GetPropagation() {
		case pb.MountPropagation_PROPAGATION_PRIVATE:
			options = append(options, "rprivate")
			// Since default root propagation in runc is rprivate ignore
			// setting the root propagation
		case pb.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			if err := ensureShared(src, mountInfos); err != nil {
				return nil, nil, err
			}
			options = append(options, "rshared")
			specgen.SetLinuxRootPropagation("rshared")
		case pb.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			if err := ensureSharedOrSlave(src, mountInfos); err != nil {
				return nil, nil, err
			}
			options = append(options, "rslave")
			if specgen.Spec().Linux.RootfsPropagation != "rshared" &&
				specgen.Spec().Linux.RootfsPropagation != "rslave" {
				specgen.SetLinuxRootPropagation("rslave")
			}
		default:
			logrus.Warnf("Unknown propagation mode for hostPath %q", mount.HostPath)
			options = append(options, "rprivate")
		}

		if mount.SelinuxRelabel {
			// Need a way in kubernetes to determine if the volume is shared or private
			if err := label.Relabel(src, mountLabel, true); err != nil && err != unix.ENOTSUP {
				return nil, nil, fmt.Errorf("relabel failed %s: %v", src, err)
			}
		}

		volumes = append(volumes, oci.ContainerVolume{
			ContainerPath: dest,
			HostPath:      src,
			Readonly:      mount.Readonly,
		})

		ociMounts = append(ociMounts, rspec.Mount{
			Source:      src,
			Destination: dest,
			Options:     options,
		})
	}

	return volumes, ociMounts, nil
}

// Ensure mount point on which path is mounted, is shared.
func ensureShared(path string, mountInfos []*dockermounts.Info) error {
	sourceMount, optionalOpts, err := getSourceMount(path, mountInfos)
	if err != nil {
		return err
	}

	// Make sure source mount point is shared.
	optsSplit := strings.Split(optionalOpts, " ")
	for _, opt := range optsSplit {
		if strings.HasPrefix(opt, "shared:") {
			return nil
		}
	}

	return fmt.Errorf("path %q is mounted on %q but it is not a shared mount", path, sourceMount)
}

// Ensure mount point on which path is mounted, is either shared or slave.
func ensureSharedOrSlave(path string, mountInfos []*dockermounts.Info) error {
	sourceMount, optionalOpts, err := getSourceMount(path, mountInfos)
	if err != nil {
		return err
	}
	// Make sure source mount point is shared.
	optsSplit := strings.Split(optionalOpts, " ")
	for _, opt := range optsSplit {
		if strings.HasPrefix(opt, "shared:") {
			return nil
		} else if strings.HasPrefix(opt, "master:") {
			return nil
		}
	}
	return fmt.Errorf("path %q is mounted on %q but it is not a shared or slave mount", path, sourceMount)
}

func getMountInfo(mountInfos []*dockermounts.Info, dir string) *dockermounts.Info {
	for _, m := range mountInfos {
		if m.Mountpoint == dir {
			return m
		}
	}
	return nil
}

func getSourceMount(source string, mountInfos []*dockermounts.Info) (string, string, error) {
	mountinfo := getMountInfo(mountInfos, source)
	if mountinfo != nil {
		return source, mountinfo.Optional, nil
	}

	path := source
	for {
		path = filepath.Dir(path)
		mountinfo = getMountInfo(mountInfos, path)
		if mountinfo != nil {
			return path, mountinfo.Optional, nil
		}

		if path == "/" {
			break
		}
	}

	// If we are here, we did not find parent mount. Something is wrong.
	return "", "", fmt.Errorf("Could not find source mount of %s", source)
}

func addImageVolumes(rootfs string, s *Server, containerInfo *storage.ContainerInfo, specgen *generate.Generator, mountLabel string) ([]rspec.Mount, error) {
	mounts := []rspec.Mount{}
	for dest := range containerInfo.Config.Config.Volumes {
		fp, err := symlink.FollowSymlinkInScope(filepath.Join(rootfs, dest), rootfs)
		if err != nil {
			return nil, err
		}
		switch s.config.ImageVolumes {
		case lib.ImageVolumesMkdir:
			if err1 := os.MkdirAll(fp, 0644); err1 != nil {
				return nil, err1
			}
		case lib.ImageVolumesBind:
			volumeDirName := stringid.GenerateNonCryptoID()
			src := filepath.Join(containerInfo.RunDir, "mounts", volumeDirName)
			if err1 := os.MkdirAll(src, 0644); err1 != nil {
				return nil, err1
			}
			// Label the source with the sandbox selinux mount label
			if mountLabel != "" {
				if err1 := label.Relabel(src, mountLabel, true); err1 != nil && err1 != unix.ENOTSUP {
					return nil, fmt.Errorf("relabel failed %s: %v", src, err1)
				}
			}

			logrus.Debugf("Adding bind mounted volume: %s to %s", src, dest)
			mounts = append(mounts, rspec.Mount{
				Source:      src,
				Destination: dest,
				Options:     []string{"rw"},
			})

		case lib.ImageVolumesIgnore:
			logrus.Debugf("Ignoring volume %v", dest)
		default:
			logrus.Fatalf("Unrecognized image volumes setting")
		}
	}
	return mounts, nil
}

// resolveSymbolicLink resolves a possbile symlink path. If the path is a symlink, returns resolved
// path; if not, returns the original path.
func resolveSymbolicLink(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return path, nil
	}
	return filepath.EvalSymlinks(path)
}

func addDevices(sb *sandbox.Sandbox, containerConfig *pb.ContainerConfig, specgen *generate.Generator) error {
	sp := specgen.Spec()
	if containerConfig.GetLinux().GetSecurityContext().GetPrivileged() {
		hostDevices, err := devices.HostDevices()
		if err != nil {
			return err
		}
		for _, hostDevice := range hostDevices {
			rd := rspec.LinuxDevice{
				Path:  hostDevice.Path,
				Type:  string(hostDevice.Type),
				Major: hostDevice.Major,
				Minor: hostDevice.Minor,
				UID:   &hostDevice.Uid,
				GID:   &hostDevice.Gid,
			}
			if hostDevice.Major == 0 && hostDevice.Minor == 0 {
				// Invalid device, most likely a symbolic link, skip it.
				continue
			}
			specgen.AddDevice(rd)
		}
		sp.Linux.Resources.Devices = []rspec.LinuxDeviceCgroup{
			{
				Allow:  true,
				Access: "rwm",
			},
		}
		return nil
	}
	for _, device := range containerConfig.GetDevices() {
		path, err := resolveSymbolicLink(device.HostPath)
		if err != nil {
			return err
		}
		dev, err := devices.DeviceFromPath(path, device.Permissions)
		// if there was no error, return the device
		if err == nil {
			rd := rspec.LinuxDevice{
				Path:  device.ContainerPath,
				Type:  string(dev.Type),
				Major: dev.Major,
				Minor: dev.Minor,
				UID:   &dev.Uid,
				GID:   &dev.Gid,
			}
			specgen.AddDevice(rd)
			sp.Linux.Resources.Devices = append(sp.Linux.Resources.Devices, rspec.LinuxDeviceCgroup{
				Allow:  true,
				Type:   string(dev.Type),
				Major:  &dev.Major,
				Minor:  &dev.Minor,
				Access: dev.Permissions,
			})
			continue
		}
		// if the device is not a device node
		// try to see if it's a directory holding many devices
		if err == devices.ErrNotADevice {

			// check if it is a directory
			if src, e := os.Stat(path); e == nil && src.IsDir() {

				// mount the internal devices recursively
				filepath.Walk(path, func(dpath string, f os.FileInfo, e error) error {
					childDevice, e := devices.DeviceFromPath(dpath, device.Permissions)
					if e != nil {
						// ignore the device
						return nil
					}
					cPath := strings.Replace(dpath, path, device.ContainerPath, 1)
					rd := rspec.LinuxDevice{
						Path:  cPath,
						Type:  string(childDevice.Type),
						Major: childDevice.Major,
						Minor: childDevice.Minor,
						UID:   &childDevice.Uid,
						GID:   &childDevice.Gid,
					}
					specgen.AddDevice(rd)
					sp.Linux.Resources.Devices = append(sp.Linux.Resources.Devices, rspec.LinuxDeviceCgroup{
						Allow:  true,
						Type:   string(childDevice.Type),
						Major:  &childDevice.Major,
						Minor:  &childDevice.Minor,
						Access: childDevice.Permissions,
					})

					return nil
				})
			}
		}
	}
	return nil
}

// buildOCIProcessArgs build an OCI compatible process arguments slice.
func buildOCIProcessArgs(containerKubeConfig *pb.ContainerConfig, imageOCIConfig *v1.Image) ([]string, error) {
	//# Start the nginx container using the default command, but use custom
	//arguments (arg1 .. argN) for that command.
	//kubectl run nginx --image=nginx -- <arg1> <arg2> ... <argN>

	//# Start the nginx container using a different command and custom arguments.
	//kubectl run nginx --image=nginx --command -- <cmd> <arg1> ... <argN>

	kubeCommands := containerKubeConfig.Command
	kubeArgs := containerKubeConfig.Args

	// merge image config and kube config
	// same as docker does today...
	if imageOCIConfig != nil {
		if len(kubeCommands) == 0 {
			if len(kubeArgs) == 0 {
				kubeArgs = imageOCIConfig.Config.Cmd
			}
			if kubeCommands == nil {
				kubeCommands = imageOCIConfig.Config.Entrypoint
			}
		}
	}

	if len(kubeCommands) == 0 && len(kubeArgs) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	// create entrypoint and args
	var entrypoint string
	var args []string
	if len(kubeCommands) != 0 {
		entrypoint = kubeCommands[0]
		args = append(kubeCommands[1:], kubeArgs...)
	} else {
		entrypoint = kubeArgs[0]
		args = kubeArgs[1:]
	}

	processArgs := append([]string{entrypoint}, args...)

	logrus.Debugf("OCI process args %v", processArgs)

	return processArgs, nil
}

// addOCIHook look for hooks programs installed in hooksDirPath and add them to spec
func addOCIHook(specgen *generate.Generator, hook lib.HookParams) error {
	logrus.Debugf("AddOCIHook", hook)
	for _, stage := range hook.Stage {
		h := rspec.Hook{
			Path: hook.Hook,
			Args: append([]string{hook.Hook}, hook.Arguments...),
			Env:  []string{fmt.Sprintf("stage=%s", stage)},
		}
		switch stage {
		case "prestart":
			specgen.AddPreStartHook(h)
		case "poststart":
			specgen.AddPostStartHook(h)
		case "poststop":
			specgen.AddPostStopHook(h)
		}
	}
	return nil
}

// setupContainerUser sets the UID, GID and supplemental groups in OCI runtime config
func setupContainerUser(specgen *generate.Generator, rootfs string, sc *pb.LinuxContainerSecurityContext, imageConfig *v1.Image) error {
	if sc != nil {
		containerUser := ""
		// Case 1: run as user is set by kubelet
		if sc.GetRunAsUser() != nil {
			containerUser = strconv.FormatInt(sc.GetRunAsUser().GetValue(), 10)
		} else {
			// Case 2: run as username is set by kubelet
			userName := sc.GetRunAsUsername()
			if userName != "" {
				containerUser = userName
			} else {
				// Case 3: get user from image config
				if imageConfig != nil {
					imageUser := imageConfig.Config.User
					if imageUser != "" {
						containerUser = imageUser
					}
				}
			}
		}

		logrus.Debugf("CONTAINER USER: %+v", containerUser)

		// Add uid, gid and groups from user
		uid, gid, addGroups, err1 := getUserInfo(rootfs, containerUser)
		if err1 != nil {
			return err1
		}

		logrus.Debugf("UID: %v, GID: %v, Groups: %+v", uid, gid, addGroups)
		specgen.SetProcessUID(uid)
		specgen.SetProcessGID(gid)
		for _, group := range addGroups {
			specgen.AddProcessAdditionalGid(group)
		}

		// Add groups from CRI
		groups := sc.GetSupplementalGroups()
		for _, group := range groups {
			specgen.AddProcessAdditionalGid(uint32(group))
		}
	}
	return nil
}

// setupCapabilities sets process.capabilities in the OCI runtime config.
func setupCapabilities(specgen *generate.Generator, capabilities *pb.Capability) error {
	if capabilities == nil {
		return nil
	}

	toCAPPrefixed := func(cap string) string {
		if !strings.HasPrefix(strings.ToLower(cap), "cap_") {
			return "CAP_" + strings.ToUpper(cap)
		}
		return cap
	}

	// Add/drop all capabilities if "all" is specified, so that
	// following individual add/drop could still work. E.g.
	// AddCapabilities: []string{"ALL"}, DropCapabilities: []string{"CHOWN"}
	// will be all capabilities without `CAP_CHOWN`.
	// see https://github.com/kubernetes/kubernetes/issues/51980
	if inStringSlice(capabilities.GetAddCapabilities(), "ALL") {
		for _, c := range getOCICapabilitiesList() {
			if err := specgen.AddProcessCapabilityAmbient(c); err != nil {
				return err
			}
			if err := specgen.AddProcessCapabilityBounding(c); err != nil {
				return err
			}
			if err := specgen.AddProcessCapabilityEffective(c); err != nil {
				return err
			}
			if err := specgen.AddProcessCapabilityInheritable(c); err != nil {
				return err
			}
			if err := specgen.AddProcessCapabilityPermitted(c); err != nil {
				return err
			}
		}
	}
	if inStringSlice(capabilities.GetDropCapabilities(), "ALL") {
		for _, c := range getOCICapabilitiesList() {
			if err := specgen.DropProcessCapabilityAmbient(c); err != nil {
				return err
			}
			if err := specgen.DropProcessCapabilityBounding(c); err != nil {
				return err
			}
			if err := specgen.DropProcessCapabilityEffective(c); err != nil {
				return err
			}
			if err := specgen.DropProcessCapabilityInheritable(c); err != nil {
				return err
			}
			if err := specgen.DropProcessCapabilityPermitted(c); err != nil {
				return err
			}
		}
	}

	for _, cap := range capabilities.GetAddCapabilities() {
		if strings.ToUpper(cap) == "ALL" {
			continue
		}
		capPrefixed := toCAPPrefixed(cap)
		if err := specgen.AddProcessCapabilityAmbient(capPrefixed); err != nil {
			return err
		}
		if err := specgen.AddProcessCapabilityBounding(capPrefixed); err != nil {
			return err
		}
		if err := specgen.AddProcessCapabilityEffective(capPrefixed); err != nil {
			return err
		}
		if err := specgen.AddProcessCapabilityInheritable(capPrefixed); err != nil {
			return err
		}
		if err := specgen.AddProcessCapabilityPermitted(capPrefixed); err != nil {
			return err
		}
	}

	for _, cap := range capabilities.GetDropCapabilities() {
		if strings.ToUpper(cap) == "ALL" {
			continue
		}
		capPrefixed := toCAPPrefixed(cap)
		if err := specgen.DropProcessCapabilityAmbient(capPrefixed); err != nil {
			return fmt.Errorf("failed to drop cap %s %v", capPrefixed, err)
		}
		if err := specgen.DropProcessCapabilityBounding(capPrefixed); err != nil {
			return fmt.Errorf("failed to drop cap %s %v", capPrefixed, err)
		}
		if err := specgen.DropProcessCapabilityEffective(capPrefixed); err != nil {
			return fmt.Errorf("failed to drop cap %s %v", capPrefixed, err)
		}
		if err := specgen.DropProcessCapabilityInheritable(capPrefixed); err != nil {
			return fmt.Errorf("failed to drop cap %s %v", capPrefixed, err)
		}
		if err := specgen.DropProcessCapabilityPermitted(capPrefixed); err != nil {
			return fmt.Errorf("failed to drop cap %s %v", capPrefixed, err)
		}
	}

	return nil
}

func hostNetwork(containerConfig *pb.ContainerConfig) bool {
	securityContext := containerConfig.GetLinux().GetSecurityContext()
	if securityContext == nil || securityContext.GetNamespaceOptions() == nil {
		return false
	}

	return securityContext.GetNamespaceOptions().GetNetwork() == pb.NamespaceMode_NODE
}

// ensureSaneLogPath is a hack to fix https://issues.k8s.io/44043 which causes
// logPath to be a broken symlink to some magical Docker path. Ideally we
// wouldn't have to deal with this, but until that issue is fixed we have to
// remove the path if it's a broken symlink.
func ensureSaneLogPath(logPath string) error {
	// If the path exists but the resolved path does not, then we have a broken
	// symlink and we need to remove it.
	fi, err := os.Lstat(logPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		// Non-existent files and non-symlinks aren't our problem.
		return nil
	}

	_, err = os.Stat(logPath)
	if os.IsNotExist(err) {
		err = os.RemoveAll(logPath)
		if err != nil {
			return fmt.Errorf("ensureSaneLogPath remove bad logPath: %s", err)
		}
	}
	return nil
}

// addSecretsBindMounts mounts user defined secrets to the container
func addSecretsBindMounts(mountLabel, ctrRunDir string, defaultMounts []string, specgen generate.Generator) ([]rspec.Mount, error) {
	containerMounts := specgen.Spec().Mounts
	mounts, err := secretMounts(defaultMounts, mountLabel, ctrRunDir, containerMounts)
	if err != nil {
		return nil, err
	}
	return mounts, nil
}

// CreateContainer creates a new container in specified PodSandbox
func (s *Server) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (res *pb.CreateContainerResponse, err error) {
	const operation = "create_container"
	defer func() {
		recordOperation(operation, time.Now())
		recordError(operation, err)
	}()
	logrus.Debugf("CreateContainerRequest %+v", req)

	s.updateLock.RLock()
	defer s.updateLock.RUnlock()

	sbID := req.PodSandboxId
	if sbID == "" {
		return nil, fmt.Errorf("PodSandboxId should not be empty")
	}

	sandboxID, err := s.PodIDIndex().Get(sbID)
	if err != nil {
		return nil, fmt.Errorf("PodSandbox with ID starting with %s not found: %v", sbID, err)
	}

	sb := s.getSandbox(sandboxID)
	if sb == nil {
		return nil, fmt.Errorf("specified sandbox not found: %s", sandboxID)
	}

	// The config of the container
	containerConfig := req.GetConfig()
	if containerConfig == nil {
		return nil, fmt.Errorf("CreateContainerRequest.ContainerConfig is nil")
	}

	if containerConfig.GetMetadata() == nil {
		return nil, fmt.Errorf("CreateContainerRequest.ContainerConfig.Metadata is nil")
	}

	name := containerConfig.GetMetadata().GetName()
	if name == "" {
		return nil, fmt.Errorf("CreateContainerRequest.ContainerConfig.Name is empty")
	}

	containerID, containerName, err := s.generateContainerIDandName(sb.Metadata(), containerConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			s.ReleaseContainerName(containerName)
		}
	}()

	container, err := s.createSandboxContainer(ctx, containerID, containerName, sb, req.GetSandboxConfig(), containerConfig)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err2 := s.StorageRuntimeServer().DeleteContainer(containerID)
			if err2 != nil {
				logrus.Warnf("Failed to cleanup container directory: %v", err2)
			}
		}
	}()

	if err = s.Runtime().CreateContainer(container, sb.CgroupParent()); err != nil {
		return nil, err
	}

	s.addContainer(container)

	if err = s.CtrIDIndex().Add(containerID); err != nil {
		s.removeContainer(container)
		return nil, err
	}

	s.ContainerStateToDisk(container)

	resp := &pb.CreateContainerResponse{
		ContainerId: containerID,
	}

	logrus.Debugf("CreateContainerResponse: %+v", resp)
	return resp, nil
}

func (s *Server) setupOCIHooks(specgen *generate.Generator, sb *sandbox.Sandbox, containerConfig *pb.ContainerConfig, command string) error {
	mounts := containerConfig.GetMounts()
	addedHooks := map[string]struct{}{}
	addHook := func(hook lib.HookParams) error {
		// Only add a hook once
		if _, ok := addedHooks[hook.Hook]; !ok {
			if err := addOCIHook(specgen, hook); err != nil {
				return err
			}
			addedHooks[hook.Hook] = struct{}{}
		}
		return nil
	}
	for _, hook := range s.Hooks() {
		logrus.Debugf("SetupOCIHooks", hook)
		if hook.HasBindMounts && len(mounts) > 0 {
			if err := addHook(hook); err != nil {
				return err
			}
			continue
		}
		for _, cmd := range hook.Cmds {
			match, err := regexp.MatchString(cmd, command)
			if err != nil {
				logrus.Errorf("Invalid regex %q:%q", cmd, err)
				continue
			}
			if match {
				if err := addHook(hook); err != nil {
					return err
				}
			}
		}
		for _, annotationRegex := range hook.Annotations {
			for _, annotation := range containerConfig.GetAnnotations() {
				match, err := regexp.MatchString(annotationRegex, annotation)
				if err != nil {
					logrus.Errorf("Invalid regex %q:%q", annotationRegex, err)
					continue
				}
				if match {
					if err := addHook(hook); err != nil {
						return err
					}
				}
			}
			for _, annotation := range sb.Annotations() {
				match, err := regexp.MatchString(annotationRegex, annotation)
				if err != nil {
					logrus.Errorf("Invalid regex %q:%q", annotationRegex, err)
					continue
				}
				if match {
					if err := addHook(hook); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
func (s *Server) createSandboxContainer(ctx context.Context, containerID string, containerName string, sb *sandbox.Sandbox, sandboxConfig *pb.PodSandboxConfig, containerConfig *pb.ContainerConfig) (*oci.Container, error) {
	if sb == nil {
		return nil, errors.New("createSandboxContainer needs a sandbox")
	}

	// TODO: simplify this function (cyclomatic complexity here is high)
	// TODO: factor generating/updating the spec into something other projects can vendor

	// creates a spec Generator with the default spec.
	specgen := generate.New()
	specgen.HostSpecific = true
	specgen.ClearProcessRlimits()

	var readOnlyRootfs bool
	var privileged bool
	if containerConfig.GetLinux().GetSecurityContext() != nil {
		if containerConfig.GetLinux().GetSecurityContext().Privileged {
			privileged = true
		}

		if containerConfig.GetLinux().GetSecurityContext().ReadonlyRootfs {
			readOnlyRootfs = true
			specgen.SetRootReadonly(true)
		}
	}

	mountLabel := sb.MountLabel()
	processLabel := sb.ProcessLabel()
	selinuxConfig := containerConfig.GetLinux().GetSecurityContext().GetSelinuxOptions()
	if selinuxConfig != nil {
		var err error
		processLabel, mountLabel, err = getSELinuxLabels(selinuxConfig, privileged)
		if err != nil {
			return nil, err
		}
	}

	containerVolumes, ociMounts, err := addOCIBindMounts(mountLabel, containerConfig, &specgen)
	if err != nil {
		return nil, err
	}

	volumesJSON, err := json.Marshal(containerVolumes)
	if err != nil {
		return nil, err
	}
	specgen.AddAnnotation(annotations.Volumes, string(volumesJSON))

	mnt := rspec.Mount{
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "relatime", "ro"},
	}
	// Add cgroup mount so container process can introspect its own limits
	specgen.AddMount(mnt)

	if err := addDevices(sb, containerConfig, &specgen); err != nil {
		return nil, err
	}

	labels := containerConfig.GetLabels()

	if err := validateLabels(labels); err != nil {
		return nil, err
	}

	metadata := containerConfig.GetMetadata()

	kubeAnnotations := containerConfig.GetAnnotations()
	if kubeAnnotations != nil {
		for k, v := range kubeAnnotations {
			specgen.AddAnnotation(k, v)
		}
	}
	if labels != nil {
		for k, v := range labels {
			specgen.AddAnnotation(k, v)
		}
	}

	// set this container's apparmor profile if it is set by sandbox
	if s.appArmorEnabled && !privileged {

		appArmorProfileName := s.getAppArmorProfileName(containerConfig.GetLinux().GetSecurityContext().GetApparmorProfile())
		if appArmorProfileName != "" {
			// reload default apparmor profile if it is unloaded.
			if s.appArmorProfile == apparmor.DefaultApparmorProfile {
				if err := apparmor.EnsureDefaultApparmorProfile(); err != nil {
					return nil, err
				}
			}

			specgen.SetProcessApparmorProfile(appArmorProfileName)
		}

	}

	logPath := containerConfig.GetLogPath()
	sboxLogDir := sandboxConfig.GetLogDirectory()
	if sboxLogDir == "" {
		sboxLogDir = sb.LogDir()
	}
	if logPath == "" {
		logPath = filepath.Join(sboxLogDir, containerID+".log")
	}
	if !filepath.IsAbs(logPath) {
		// XXX: It's not really clear what this should be versus the sbox logDirectory.
		logrus.Warnf("requested logPath for ctr id %s is a relative path: %s", containerID, logPath)
		logPath = filepath.Join(sboxLogDir, logPath)
		logrus.Warnf("logPath from relative path is now absolute: %s", logPath)
	}

	// Handle https://issues.k8s.io/44043
	if err := ensureSaneLogPath(logPath); err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"sbox.logdir": sboxLogDir,
		"ctr.logfile": containerConfig.GetLogPath(),
		"log_path":    logPath,
	}).Debugf("setting container's log_path")

	specgen.SetProcessTerminal(containerConfig.Tty)
	if containerConfig.Tty {
		specgen.AddProcessEnv("TERM", "xterm")
	}

	linux := containerConfig.GetLinux()
	if linux != nil {
		resources := linux.GetResources()
		if resources != nil {
			specgen.SetLinuxResourcesCPUPeriod(uint64(resources.GetCpuPeriod()))
			specgen.SetLinuxResourcesCPUQuota(resources.GetCpuQuota())
			specgen.SetLinuxResourcesCPUShares(uint64(resources.GetCpuShares()))
			specgen.SetLinuxResourcesMemoryLimit(resources.GetMemoryLimitInBytes())
			specgen.SetProcessOOMScoreAdj(int(resources.GetOomScoreAdj()))
			specgen.SetLinuxResourcesCPUCpus(resources.GetCpusetCpus())
			specgen.SetLinuxResourcesCPUMems(resources.GetCpusetMems())
		}

		var cgPath string
		parent := defaultCgroupfsParent
		useSystemd := s.config.CgroupManager == oci.SystemdCgroupsManager
		if useSystemd {
			parent = defaultSystemdParent
		}
		if sb.CgroupParent() != "" {
			parent = sb.CgroupParent()
		}
		if useSystemd {
			cgPath = parent + ":" + scopePrefix + ":" + containerID
		} else {
			cgPath = filepath.Join(parent, scopePrefix+"-"+containerID)
		}
		specgen.SetLinuxCgroupsPath(cgPath)

		if privileged {
			specgen.SetupPrivileged(true)
			setOCIBindMountsPrivileged(&specgen)
		} else {
			err = setupCapabilities(&specgen, linux.GetSecurityContext().GetCapabilities())
			if err != nil {
				return nil, err
			}
		}
		specgen.SetProcessSelinuxLabel(processLabel)
		specgen.SetLinuxMountLabel(mountLabel)
		specgen.SetProcessNoNewPrivileges(linux.GetSecurityContext().GetNoNewPrivs())

		if containerConfig.GetLinux().GetSecurityContext() != nil &&
			!containerConfig.GetLinux().GetSecurityContext().Privileged {
			for _, mp := range []string{
				"/proc/kcore",
				"/proc/latency_stats",
				"/proc/timer_list",
				"/proc/timer_stats",
				"/proc/sched_debug",
				"/proc/scsi",
				"/sys/firmware",
			} {
				specgen.AddLinuxMaskedPaths(mp)
			}

			for _, rp := range []string{
				"/proc/asound",
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			} {
				specgen.AddLinuxReadonlyPaths(rp)
			}
		}
	}
	// Join the namespace paths for the pod sandbox container.
	podInfraState := s.Runtime().ContainerStatus(sb.InfraContainer())

	logrus.Debugf("pod container state %+v", podInfraState)

	ipcNsPath := fmt.Sprintf("/proc/%d/ns/ipc", podInfraState.Pid)
	if err := specgen.AddOrReplaceLinuxNamespace(string(rspec.IPCNamespace), ipcNsPath); err != nil {
		return nil, err
	}

	utsNsPath := fmt.Sprintf("/proc/%d/ns/uts", podInfraState.Pid)
	if err := specgen.AddOrReplaceLinuxNamespace(string(rspec.UTSNamespace), utsNsPath); err != nil {
		return nil, err
	}

	if containerConfig.GetLinux().GetSecurityContext().GetNamespaceOptions().GetPid() == pb.NamespaceMode_NODE {
		// kubernetes PodSpec specify to use Host PID namespace
		specgen.RemoveLinuxNamespace(string(rspec.PIDNamespace))
	} else if s.config.EnableSharedPIDNamespace {
		// share Pod PID namespace
		pidNsPath := fmt.Sprintf("/proc/%d/ns/pid", podInfraState.Pid)
		if err := specgen.AddOrReplaceLinuxNamespace(string(rspec.PIDNamespace), pidNsPath); err != nil {
			return nil, err
		}
	}

	netNsPath := sb.NetNsPath()
	if netNsPath == "" {
		// The sandbox does not have a permanent namespace,
		// it's on the host one.
		netNsPath = fmt.Sprintf("/proc/%d/ns/net", podInfraState.Pid)
	}

	if err := specgen.AddOrReplaceLinuxNamespace(string(rspec.NetworkNamespace), netNsPath); err != nil {
		return nil, err
	}

	imageSpec := containerConfig.GetImage()
	if imageSpec == nil {
		return nil, fmt.Errorf("CreateContainerRequest.ContainerConfig.Image is nil")
	}

	image := imageSpec.Image
	if image == "" {
		return nil, fmt.Errorf("CreateContainerRequest.ContainerConfig.Image.Image is empty")
	}
	images, err := s.StorageImageServer().ResolveNames(image)
	if err != nil {
		if err == storage.ErrCannotParseImageID {
			images = append(images, image)
		} else {
			return nil, err
		}
	}

	// Get imageName and imageRef that are later requested in container status
	status, err := s.StorageImageServer().ImageStatus(s.ImageContext(), images[0])
	if err != nil {
		return nil, err
	}
	imageName := status.Name
	imageRef := status.ID
	if len(status.RepoDigests) > 0 {
		imageRef = status.RepoDigests[0]
	}

	specgen.AddAnnotation(annotations.Image, image)
	specgen.AddAnnotation(annotations.ImageName, imageName)
	specgen.AddAnnotation(annotations.ImageRef, imageRef)
	specgen.AddAnnotation(annotations.IP, sb.IP())

	mnt = rspec.Mount{
		Type:        "bind",
		Source:      sb.ShmPath(),
		Destination: "/etc/shm",
		Options:     []string{"rw", "bind"},
	}
	// bind mount the pod shm
	specgen.AddMount(mnt)

	options := []string{"rw"}
	if readOnlyRootfs {
		options = []string{"ro"}
	}
	if sb.ResolvPath() != "" {
		if err := label.Relabel(sb.ResolvPath(), mountLabel, true); err != nil && err != unix.ENOTSUP {
			return nil, err
		}

		mnt = rspec.Mount{
			Type:        "bind",
			Source:      sb.ResolvPath(),
			Destination: "/etc/resolv.conf",
			Options:     append(options, "bind"),
		}
		// bind mount the pod resolver file
		specgen.AddMount(mnt)
	}

	if sb.HostnamePath() != "" {
		if err := label.Relabel(sb.HostnamePath(), mountLabel, true); err != nil && err != unix.ENOTSUP {
			return nil, err
		}

		mnt = rspec.Mount{
			Type:        "bind",
			Source:      sb.HostnamePath(),
			Destination: "/etc/hostname",
			Options:     append(options, "bind"),
		}
		specgen.AddMount(mnt)
	}

	isInCRIMounts := func(dst string, mounts []*pb.Mount) bool {
		for _, m := range mounts {
			if m.ContainerPath == dst {
				return true
			}
		}
		return false
	}

	if !isInCRIMounts("/etc/hosts", containerConfig.GetMounts()) && hostNetwork(containerConfig) {
		// Only bind mount for host netns and when CRI does not give us any hosts file
		mnt = rspec.Mount{
			Type:        "bind",
			Source:      "/etc/hosts",
			Destination: "/etc/hosts",
			Options:     append(options, "bind"),
		}
		specgen.AddMount(mnt)
	}

	// Set hostname and add env for hostname
	specgen.SetHostname(sb.Hostname())
	specgen.AddProcessEnv("HOSTNAME", sb.Hostname())

	specgen.AddAnnotation(annotations.Name, containerName)
	specgen.AddAnnotation(annotations.ContainerID, containerID)
	specgen.AddAnnotation(annotations.SandboxID, sb.ID())
	specgen.AddAnnotation(annotations.SandboxName, sb.InfraContainer().Name())
	specgen.AddAnnotation(annotations.ContainerType, annotations.ContainerTypeContainer)
	specgen.AddAnnotation(annotations.LogPath, logPath)
	specgen.AddAnnotation(annotations.TTY, fmt.Sprintf("%v", containerConfig.Tty))
	specgen.AddAnnotation(annotations.Stdin, fmt.Sprintf("%v", containerConfig.Stdin))
	specgen.AddAnnotation(annotations.StdinOnce, fmt.Sprintf("%v", containerConfig.StdinOnce))
	specgen.AddAnnotation(annotations.ResolvPath, sb.InfraContainer().CrioAnnotations()[annotations.ResolvPath])

	created := time.Now()
	specgen.AddAnnotation(annotations.Created, created.Format(time.RFC3339Nano))

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	specgen.AddAnnotation(annotations.Metadata, string(metadataJSON))

	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return nil, err
	}
	specgen.AddAnnotation(annotations.Labels, string(labelsJSON))

	kubeAnnotationsJSON, err := json.Marshal(kubeAnnotations)
	if err != nil {
		return nil, err
	}
	specgen.AddAnnotation(annotations.Annotations, string(kubeAnnotationsJSON))

	spp := containerConfig.GetLinux().GetSecurityContext().GetSeccompProfilePath()
	if !privileged {
		if err = s.setupSeccomp(&specgen, spp); err != nil {
			return nil, err
		}
	}
	specgen.AddAnnotation(annotations.SeccompProfilePath, spp)

	metaname := metadata.Name
	attempt := metadata.Attempt
	containerInfo, err := s.StorageRuntimeServer().CreateContainer(s.ImageContext(),
		sb.Name(), sb.ID(),
		image, status.ID,
		containerName, containerID,
		metaname,
		attempt,
		mountLabel,
		nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err2 := s.StorageRuntimeServer().DeleteContainer(containerInfo.ID)
			if err2 != nil {
				logrus.Warnf("Failed to cleanup container directory: %v", err2)
			}
		}
	}()

	mountPoint, err := s.StorageRuntimeServer().StartContainer(containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to mount container %s(%s): %v", containerName, containerID, err)
	}
	specgen.AddAnnotation(annotations.MountPoint, mountPoint)

	containerImageConfig := containerInfo.Config
	if containerImageConfig == nil {
		err = fmt.Errorf("empty image config for %s", image)
		return nil, err
	}

	if containerImageConfig.Config.StopSignal != "" {
		// this key is defined in image-spec conversion document at https://github.com/opencontainers/image-spec/pull/492/files#diff-8aafbe2c3690162540381b8cdb157112R57
		specgen.AddAnnotation("org.opencontainers.image.stopSignal", containerImageConfig.Config.StopSignal)
	}

	// Add image volumes
	volumeMounts, err := addImageVolumes(mountPoint, s, &containerInfo, &specgen, mountLabel)
	if err != nil {
		return nil, err
	}

	processArgs, err := buildOCIProcessArgs(containerConfig, containerImageConfig)
	if err != nil {
		return nil, err
	}
	specgen.SetProcessArgs(processArgs)

	envs := mergeEnvs(containerImageConfig, containerConfig.GetEnvs())
	for _, e := range envs {
		parts := strings.SplitN(e, "=", 2)
		specgen.AddProcessEnv(parts[0], parts[1])
	}

	// Set working directory
	// Pick it up from image config first and override if specified in CRI
	containerCwd := "/"
	if containerImageConfig != nil {
		imageCwd := containerImageConfig.Config.WorkingDir
		if imageCwd != "" {
			containerCwd = imageCwd
		}
	}
	runtimeCwd := containerConfig.WorkingDir
	if runtimeCwd != "" {
		containerCwd = runtimeCwd
	}
	specgen.SetProcessCwd(containerCwd)
	if err := setupWorkingDirectory(mountPoint, mountLabel, containerCwd); err != nil {
		if err1 := s.StorageRuntimeServer().StopContainer(containerID); err1 != nil {
			return nil, fmt.Errorf("can't umount container after cwd error %v: %v", err, err1)
		}
		return nil, err
	}

	var secretMounts []rspec.Mount
	if len(s.config.DefaultMounts) > 0 {
		var err error
		secretMounts, err = addSecretsBindMounts(mountLabel, containerInfo.RunDir, s.config.DefaultMounts, specgen)
		if err != nil {
			return nil, fmt.Errorf("failed to mount secrets: %v", err)
		}
	}

	mounts := []rspec.Mount{}
	mounts = append(mounts, ociMounts...)
	mounts = append(mounts, volumeMounts...)
	mounts = append(mounts, secretMounts...)

	sort.Sort(orderedMounts(mounts))

	for _, m := range mounts {
		mnt = rspec.Mount{
			Type:        "bind",
			Source:      m.Source,
			Destination: m.Destination,
			Options:     append(m.Options, "bind"),
		}
		specgen.AddMount(mnt)
	}

	if err := s.setupOCIHooks(&specgen, sb, containerConfig, processArgs[0]); err != nil {
		return nil, err
	}

	// Setup user and groups
	if linux != nil {
		if err = setupContainerUser(&specgen, mountPoint, linux.GetSecurityContext(), containerImageConfig); err != nil {
			return nil, err
		}
	}

	// Set up pids limit if pids cgroup is mounted
	_, err = cgroups.FindCgroupMountpoint("pids")
	if err == nil {
		specgen.SetLinuxResourcesPidsLimit(s.config.PidsLimit)
	}

	// by default, the root path is an empty string. set it now.
	specgen.SetRootPath(mountPoint)

	saveOptions := generate.ExportOptions{}
	if err = specgen.SaveToFile(filepath.Join(containerInfo.Dir, "config.json"), saveOptions); err != nil {
		return nil, err
	}
	if err = specgen.SaveToFile(filepath.Join(containerInfo.RunDir, "config.json"), saveOptions); err != nil {
		return nil, err
	}

	crioAnnotations := specgen.Spec().Annotations

	container, err := oci.NewContainer(containerID, containerName, containerInfo.RunDir, logPath, sb.NetNs(), labels, crioAnnotations, kubeAnnotations, image, imageName, imageRef, metadata, sb.ID(), containerConfig.Tty, containerConfig.Stdin, containerConfig.StdinOnce, sb.Privileged(), sb.Trusted(), containerInfo.Dir, created, containerImageConfig.Config.StopSignal)
	if err != nil {
		return nil, err
	}
	container.SetSpec(specgen.Spec())
	container.SetMountPoint(mountPoint)
	container.SetSeccompProfilePath(spp)

	for _, cv := range containerVolumes {
		container.AddVolume(cv)
	}

	return container, nil
}

func (s *Server) setupSeccomp(specgen *generate.Generator, profile string) error {
	if profile == "" {
		// running w/o seccomp, aka unconfined
		specgen.Spec().Linux.Seccomp = nil
		return nil
	}
	if !s.seccompEnabled {
		if profile != seccompUnconfined {
			return fmt.Errorf("seccomp is not enabled in your kernel, cannot run with a profile")
		}
		logrus.Warn("seccomp is not enabled in your kernel, running container without profile")
	}
	if profile == seccompUnconfined {
		// running w/o seccomp, aka unconfined
		specgen.Spec().Linux.Seccomp = nil
		return nil
	}
	if profile == seccompRuntimeDefault || profile == seccompDockerDefault {
		return seccomp.LoadProfileFromStruct(s.seccompProfile, specgen)
	}
	if !strings.HasPrefix(profile, seccompLocalhostPrefix) {
		return fmt.Errorf("unknown seccomp profile option: %q", profile)
	}
	fname := strings.TrimPrefix(profile, "localhost/")
	file, err := ioutil.ReadFile(filepath.FromSlash(fname))
	if err != nil {
		return fmt.Errorf("cannot load seccomp profile %q: %v", fname, err)
	}
	return seccomp.LoadProfileFromBytes(file, specgen)
}

// getAppArmorProfileName gets the profile name for the given container.
func (s *Server) getAppArmorProfileName(profile string) string {
	if profile == "" {
		return ""
	}

	if profile == apparmor.ProfileRuntimeDefault {
		// If the value is runtime/default, then return default profile.
		return s.appArmorProfile
	}

	return strings.TrimPrefix(profile, apparmor.ProfileNamePrefix)
}

// openContainerFile opens a file inside a container rootfs safely
func openContainerFile(rootfs string, path string) (io.ReadCloser, error) {
	fp, err := symlink.FollowSymlinkInScope(filepath.Join(rootfs, path), rootfs)
	if err != nil {
		return nil, err
	}
	return os.Open(fp)
}

// getUserInfo returns UID, GID and additional groups for specified user
// by looking them up in /etc/passwd and /etc/group
func getUserInfo(rootfs string, userName string) (uint32, uint32, []uint32, error) {
	// We don't care if we can't open the file because
	// not all images will have these files
	passwdFile, err := openContainerFile(rootfs, "/etc/passwd")
	if err != nil {
		logrus.Warnf("Failed to open /etc/passwd: %v", err)
	} else {
		defer passwdFile.Close()
	}

	groupFile, err := openContainerFile(rootfs, "/etc/group")
	if err != nil {
		logrus.Warnf("Failed to open /etc/group: %v", err)
	} else {
		defer groupFile.Close()
	}

	execUser, err := user.GetExecUser(userName, nil, passwdFile, groupFile)
	if err != nil {
		return 0, 0, nil, err
	}

	uid := uint32(execUser.Uid)
	gid := uint32(execUser.Gid)
	var additionalGids []uint32
	for _, g := range execUser.Sgids {
		additionalGids = append(additionalGids, uint32(g))
	}

	return uid, gid, additionalGids, nil
}

func setOCIBindMountsPrivileged(g *generate.Generator) {
	spec := g.Spec()
	// clear readonly for /sys and cgroup
	for i, m := range spec.Mounts {
		if spec.Mounts[i].Destination == "/sys" && !spec.Root.Readonly {
			clearReadOnly(&spec.Mounts[i])
		}
		if m.Type == "cgroup" {
			clearReadOnly(&spec.Mounts[i])
		}
	}
	spec.Linux.ReadonlyPaths = nil
	spec.Linux.MaskedPaths = nil
}

func clearReadOnly(m *rspec.Mount) {
	var opt []string
	for _, o := range m.Options {
		if o != "ro" {
			opt = append(opt, o)
		}
	}
	m.Options = opt
}

func setupWorkingDirectory(rootfs, mountLabel, containerCwd string) error {
	fp, err := symlink.FollowSymlinkInScope(filepath.Join(rootfs, containerCwd), rootfs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(fp, 0755); err != nil {
		return err
	}
	if mountLabel != "" {
		if err1 := label.Relabel(fp, mountLabel, true); err1 != nil && err1 != unix.ENOTSUP {
			return fmt.Errorf("relabel failed %s: %v", fp, err1)
		}
	}
	return nil
}

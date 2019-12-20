package oci

import (
	"errors"
	"strconv"
	"strings"

	runhcsopts "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	// AnnotationContainerMemorySizeInMB overrides the container memory size set
	// via the OCI spec.
	//
	// Note: This annotation is in MB. OCI is in Bytes. When using this override
	// the caller MUST use MB or sizing will be wrong.
	//
	// Note: This is only present because CRI does not (currently) have a
	// `WindowsPodSandboxConfig` for setting this correctly. It should not be
	// used via OCI runtimes and rather use
	// `spec.Windows.Resources.Memory.Limit`.
	AnnotationContainerMemorySizeInMB = "io.microsoft.container.memory.sizeinmb"
	// AnnotationContainerProcessorCount overrides the container processor count
	// set via the OCI spec.
	//
	// Note: For Windows Process Containers CPU Count/Limit/Weight are mutually
	// exclusive and the caller MUST only set one of the values.
	//
	// Note: This is only present because CRI does not (currently) have a
	// `WindowsPodSandboxConfig` for setting this correctly. It should not be
	// used via OCI runtimes and rather use `spec.Windows.Resources.CPU.Count`.
	AnnotationContainerProcessorCount = "io.microsoft.container.processor.count"
	// AnnotationContainerProcessorLimit overrides the container processor limit
	// set via the OCI spec.
	//
	// Limit allows values 1 - 10,000 where 10,000 means 100% CPU. (And is the
	// default if omitted)
	//
	// Note: For Windows Process Containers CPU Count/Limit/Weight are mutually
	// exclusive and the caller MUST only set one of the values.
	//
	// Note: This is only present because CRI does not (currently) have a
	// `WindowsPodSandboxConfig` for setting this correctly. It should not be
	// used via OCI runtimes and rather use
	// `spec.Windows.Resources.CPU.Maximum`.
	AnnotationContainerProcessorLimit = "io.microsoft.container.processor.limit"
	// AnnotationContainerProcessorWeight overrides the container processor
	// weight set via the OCI spec.
	//
	// Weight allows values 0 - 10,000. (100 is the default)
	//
	// Note: For Windows Process Containers CPU Count/Limit/Weight are mutually
	// exclusive and the caller MUST only set one of the values.
	//
	// Note: This is only present because CRI does not (currently) have a
	// `WindowsPodSandboxConfig` for setting this correctly. It should not be
	// used via OCI runtimes and rather use `spec.Windows.Resources.CPU.Shares`.
	AnnotationContainerProcessorWeight = "io.microsoft.container.processor.weight"
	// AnnotationContainerStorageQoSBandwidthMaximum overrides the container
	// storage bandwidth per second set via the OCI spec.
	//
	// Note: This is only present because CRI does not (currently) have a
	// `WindowsPodSandboxConfig` for setting this correctly. It should not be
	// used via OCI runtimes and rather use
	// `spec.Windows.Resources.Storage.Bps`.
	AnnotationContainerStorageQoSBandwidthMaximum = "io.microsoft.container.storage.qos.bandwidthmaximum"
	// AnnotationContainerStorageQoSIopsMaximum overrides the container storage
	// maximum iops set via the OCI spec.
	//
	// Note: This is only present because CRI does not (currently) have a
	// `WindowsPodSandboxConfig` for setting this correctly. It should not be
	// used via OCI runtimes and rather use
	// `spec.Windows.Resources.Storage.Iops`.
	AnnotationContainerStorageQoSIopsMaximum = "io.microsoft.container.storage.qos.iopsmaximum"
	annotationAllowOvercommit                = "io.microsoft.virtualmachine.computetopology.memory.allowovercommit"
	annotationEnableDeferredCommit           = "io.microsoft.virtualmachine.computetopology.memory.enabledeferredcommit"
	// annotationMemorySizeInMB overrides the container memory size set via the
	// OCI spec.
	//
	// Note: This annotation is in MB. OCI is in Bytes. When using this override
	// the caller MUST use MB or sizing will be wrong.
	annotationMemorySizeInMB = "io.microsoft.virtualmachine.computetopology.memory.sizeinmb"
	// annotationProcessorCount overrides the hypervisor isolated vCPU count set
	// via the OCI spec.
	//
	// Note: Unlike Windows process isolated container QoS Count/Limt/Weight on
	// the UVM are not mutually exclusive and can be set together.
	annotationProcessorCount = "io.microsoft.virtualmachine.computetopology.processor.count"
	// annotationProcessorLimit overrides the hypervisor isolated vCPU limit set
	// via the OCI spec.
	//
	// Limit allows values 1 - 10,000 where 10,000 means 100% CPU. (And is the
	// default if omitted)
	//
	// Note: Unlike Windows process isolated container QoS Count/Limt/Weight on
	// the UVM are not mutually exclusive and can be set together.
	annotationProcessorLimit = "io.microsoft.virtualmachine.computetopology.processor.limit"
	// annotationProcessorWeight overrides the hypervisor isolated vCPU weight set
	// via the OCI spec.
	//
	// Weight allows values 0 - 10,000. (100 is the default if omitted)
	//
	// Note: Unlike Windows process isolated container QoS Count/Limt/Weight on
	// the UVM are not mutually exclusive and can be set together.
	annotationProcessorWeight            = "io.microsoft.virtualmachine.computetopology.processor.weight"
	annotationVPMemCount                 = "io.microsoft.virtualmachine.devices.virtualpmem.maximumcount"
	annotationVPMemSize                  = "io.microsoft.virtualmachine.devices.virtualpmem.maximumsizebytes"
	annotationPreferredRootFSType        = "io.microsoft.virtualmachine.lcow.preferredrootfstype"
	annotationBootFilesRootPath          = "io.microsoft.virtualmachine.lcow.bootfilesrootpath"
	annotationStorageQoSBandwidthMaximum = "io.microsoft.virtualmachine.storageqos.bandwidthmaximum"
	annotationStorageQoSIopsMaximum      = "io.microsoft.virtualmachine.storageqos.iopsmaximum"
)

// parseAnnotationsBool searches `a` for `key` and if found verifies that the
// value is `true` or `false` in any case. If `key` is not found returns `def`.
func parseAnnotationsBool(a map[string]string, key string, def bool) bool {
	if v, ok := a[key]; ok {
		switch strings.ToLower(v) {
		case "true":
			return true
		case "false":
			return false
		default:
			logrus.WithFields(logrus.Fields{
				logfields.OCIAnnotation: key,
				logfields.Value:         v,
				logfields.ExpectedType:  logfields.Bool,
			}).Warning("annotation could not be parsed")
		}
	}
	return def
}

// ParseAnnotationsCPUCount searches `s.Annotations` for the CPU annotation. If
// not found searches `s` for the Windows CPU section. If neither are found
// returns `def`.
func ParseAnnotationsCPUCount(s *specs.Spec, annotation string, def int32) int32 {
	if m := parseAnnotationsUint64(s.Annotations, annotation, 0); m != 0 {
		return int32(m)
	}
	if s.Windows != nil &&
		s.Windows.Resources != nil &&
		s.Windows.Resources.CPU != nil &&
		s.Windows.Resources.CPU.Count != nil &&
		*s.Windows.Resources.CPU.Count > 0 {
		return int32(*s.Windows.Resources.CPU.Count)
	}
	return def
}

// ParseAnnotationsCPULimit searches `s.Annotations` for the CPU annotation. If
// not found searches `s` for the Windows CPU section. If neither are found
// returns `def`.
func ParseAnnotationsCPULimit(s *specs.Spec, annotation string, def int32) int32 {
	if m := parseAnnotationsUint64(s.Annotations, annotation, 0); m != 0 {
		return int32(m)
	}
	if s.Windows != nil &&
		s.Windows.Resources != nil &&
		s.Windows.Resources.CPU != nil &&
		s.Windows.Resources.CPU.Maximum != nil &&
		*s.Windows.Resources.CPU.Maximum > 0 {
		return int32(*s.Windows.Resources.CPU.Maximum)
	}
	return def
}

// ParseAnnotationsCPUWeight searches `s.Annotations` for the CPU annotation. If
// not found searches `s` for the Windows CPU section. If neither are found
// returns `def`.
func ParseAnnotationsCPUWeight(s *specs.Spec, annotation string, def int32) int32 {
	if m := parseAnnotationsUint64(s.Annotations, annotation, 0); m != 0 {
		return int32(m)
	}
	if s.Windows != nil &&
		s.Windows.Resources != nil &&
		s.Windows.Resources.CPU != nil &&
		s.Windows.Resources.CPU.Shares != nil &&
		*s.Windows.Resources.CPU.Shares > 0 {
		return int32(*s.Windows.Resources.CPU.Shares)
	}
	return def
}

// ParseAnnotationsStorageIops searches `s.Annotations` for the `Iops`
// annotation. If not found searches `s` for the Windows Storage section. If
// neither are found returns `def`.
func ParseAnnotationsStorageIops(s *specs.Spec, annotation string, def int32) int32 {
	if m := parseAnnotationsUint64(s.Annotations, annotation, 0); m != 0 {
		return int32(m)
	}
	if s.Windows != nil &&
		s.Windows.Resources != nil &&
		s.Windows.Resources.Storage != nil &&
		s.Windows.Resources.Storage.Iops != nil &&
		*s.Windows.Resources.Storage.Iops > 0 {
		return int32(*s.Windows.Resources.Storage.Iops)
	}
	return def
}

// ParseAnnotationsStorageBps searches `s.Annotations` for the `Bps` annotation.
// If not found searches `s` for the Windows Storage section. If neither are
// found returns `def`.
func ParseAnnotationsStorageBps(s *specs.Spec, annotation string, def int32) int32 {
	if m := parseAnnotationsUint64(s.Annotations, annotation, 0); m != 0 {
		return int32(m)
	}
	if s.Windows != nil &&
		s.Windows.Resources != nil &&
		s.Windows.Resources.Storage != nil &&
		s.Windows.Resources.Storage.Bps != nil &&
		*s.Windows.Resources.Storage.Bps > 0 {
		return int32(*s.Windows.Resources.Storage.Bps)
	}
	return def
}

// ParseAnnotationsMemory searches `s.Annotations` for the memory annotation. If
// not found searches `s` for the Windows memory section. If neither are found
// returns `def`.
//
// Note: The returned value is in `MB`.
func ParseAnnotationsMemory(s *specs.Spec, annotation string, def int32) int32 {
	if m := parseAnnotationsUint64(s.Annotations, annotation, 0); m != 0 {
		return int32(m)
	}
	if s.Windows != nil &&
		s.Windows.Resources != nil &&
		s.Windows.Resources.Memory != nil &&
		s.Windows.Resources.Memory.Limit != nil &&
		*s.Windows.Resources.Memory.Limit > 0 {
		return int32(*s.Windows.Resources.Memory.Limit / 1024 / 1024)
	}
	return def
}

// parseAnnotationsPreferredRootFSType searches `a` for `key` and verifies that the
// value is in the set of allowed values. If `key` is not found returns `def`.
func parseAnnotationsPreferredRootFSType(a map[string]string, key string, def uvm.PreferredRootFSType) uvm.PreferredRootFSType {
	if v, ok := a[key]; ok {
		switch v {
		case "initrd":
			return uvm.PreferredRootFSTypeInitRd
		case "vhd":
			return uvm.PreferredRootFSTypeVHD
		default:
			logrus.Warningf("annotation: '%s', with value: '%s' must be 'initrd' or 'vhd'", key, v)
		}
	}
	return def
}

// parseAnnotationsUint32 searches `a` for `key` and if found verifies that the
// value is a 32 bit unsigned integer. If `key` is not found returns `def`.
func parseAnnotationsUint32(a map[string]string, key string, def uint32) uint32 {
	if v, ok := a[key]; ok {
		countu, err := strconv.ParseUint(v, 10, 32)
		if err == nil {
			v := uint32(countu)
			return v
		}
		logrus.WithFields(logrus.Fields{
			logfields.OCIAnnotation: key,
			logfields.Value:         v,
			logfields.ExpectedType:  logfields.Uint32,
			logrus.ErrorKey:         err,
		}).Warning("annotation could not be parsed")
	}
	return def
}

// parseAnnotationsUint64 searches `a` for `key` and if found verifies that the
// value is a 64 bit unsigned integer. If `key` is not found returns `def`.
func parseAnnotationsUint64(a map[string]string, key string, def uint64) uint64 {
	if v, ok := a[key]; ok {
		countu, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			return countu
		}
		logrus.WithFields(logrus.Fields{
			logfields.OCIAnnotation: key,
			logfields.Value:         v,
			logfields.ExpectedType:  logfields.Uint64,
			logrus.ErrorKey:         err,
		}).Warning("annotation could not be parsed")
	}
	return def
}

// parseAnnotationsString searches `a` for `key`. If `key` is not found returns `def`.
func parseAnnotationsString(a map[string]string, key string, def string) string {
	if v, ok := a[key]; ok {
		return v
	}
	return def
}

// SpecToUVMCreateOpts parses `s` and returns either `*uvm.OptionsLCOW` or
// `*uvm.OptionsWCOW`.
func SpecToUVMCreateOpts(s *specs.Spec, id, owner string) (interface{}, error) {
	if !IsIsolated(s) {
		return nil, errors.New("cannot create UVM opts for non-isolated spec")
	}
	if IsLCOW(s) {
		lopts := uvm.NewDefaultOptionsLCOW(id, owner)
		lopts.MemorySizeInMB = ParseAnnotationsMemory(s, annotationMemorySizeInMB, lopts.MemorySizeInMB)
		lopts.AllowOvercommit = parseAnnotationsBool(s.Annotations, annotationAllowOvercommit, lopts.AllowOvercommit)
		lopts.EnableDeferredCommit = parseAnnotationsBool(s.Annotations, annotationEnableDeferredCommit, lopts.EnableDeferredCommit)
		lopts.ProcessorCount = ParseAnnotationsCPUCount(s, annotationProcessorCount, lopts.ProcessorCount)
		lopts.ProcessorLimit = ParseAnnotationsCPULimit(s, annotationProcessorLimit, lopts.ProcessorLimit)
		lopts.ProcessorWeight = ParseAnnotationsCPUWeight(s, annotationProcessorWeight, lopts.ProcessorWeight)
		lopts.VPMemDeviceCount = parseAnnotationsUint32(s.Annotations, annotationVPMemCount, lopts.VPMemDeviceCount)
		lopts.VPMemSizeBytes = parseAnnotationsUint64(s.Annotations, annotationVPMemSize, lopts.VPMemSizeBytes)
		lopts.StorageQoSBandwidthMaximum = ParseAnnotationsStorageBps(s, annotationStorageQoSBandwidthMaximum, lopts.StorageQoSBandwidthMaximum)
		lopts.StorageQoSIopsMaximum = ParseAnnotationsStorageIops(s, annotationStorageQoSIopsMaximum, lopts.StorageQoSIopsMaximum)
		lopts.PreferredRootFSType = parseAnnotationsPreferredRootFSType(s.Annotations, annotationPreferredRootFSType, lopts.PreferredRootFSType)
		switch lopts.PreferredRootFSType {
		case uvm.PreferredRootFSTypeInitRd:
			lopts.RootFSFile = uvm.InitrdFile
		case uvm.PreferredRootFSTypeVHD:
			lopts.RootFSFile = uvm.VhdFile
		}
		lopts.BootFilesPath = parseAnnotationsString(s.Annotations, annotationBootFilesRootPath, lopts.BootFilesPath)
		return lopts, nil
	} else if IsWCOW(s) {
		wopts := uvm.NewDefaultOptionsWCOW(id, owner)
		wopts.MemorySizeInMB = ParseAnnotationsMemory(s, annotationMemorySizeInMB, wopts.MemorySizeInMB)
		wopts.AllowOvercommit = parseAnnotationsBool(s.Annotations, annotationAllowOvercommit, wopts.AllowOvercommit)
		wopts.EnableDeferredCommit = parseAnnotationsBool(s.Annotations, annotationEnableDeferredCommit, wopts.EnableDeferredCommit)
		wopts.ProcessorCount = ParseAnnotationsCPUCount(s, annotationProcessorCount, wopts.ProcessorCount)
		wopts.ProcessorLimit = ParseAnnotationsCPULimit(s, annotationProcessorLimit, wopts.ProcessorLimit)
		wopts.ProcessorWeight = ParseAnnotationsCPUWeight(s, annotationProcessorWeight, wopts.ProcessorWeight)
		wopts.StorageQoSBandwidthMaximum = ParseAnnotationsStorageBps(s, annotationStorageQoSBandwidthMaximum, wopts.StorageQoSBandwidthMaximum)
		wopts.StorageQoSIopsMaximum = ParseAnnotationsStorageIops(s, annotationStorageQoSIopsMaximum, wopts.StorageQoSIopsMaximum)
		return wopts, nil
	}
	return nil, errors.New("cannot create UVM opts spec is not LCOW or WCOW")
}

// UpdateSpecFromOptions sets extra annotations on the OCI spec based on the
// `opts` struct.
func UpdateSpecFromOptions(s specs.Spec, opts *runhcsopts.Options) specs.Spec {
	if opts != nil && opts.BootFilesRootPath != "" {
		s.Annotations[annotationBootFilesRootPath] = opts.BootFilesRootPath
	}

	return s
}

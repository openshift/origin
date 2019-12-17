package uvm

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/sirupsen/logrus"
)

// Options are the set of options passed to Create() to create a utility vm.
type Options struct {
	ID                      string // Identifier for the uvm. Defaults to generated GUID.
	Owner                   string // Specifies the owner. Defaults to executable name.
	AdditionHCSDocumentJSON string // Optional additional JSON to merge into the HCS document prior

	// MemorySizeInMB sets the UVM memory. If `0` will default to platform
	// default.
	MemorySizeInMB int32

	// Memory for UVM. Defaults to true. For physical backed memory, set to
	// false.
	AllowOvercommit bool

	// Memory for UVM. Defaults to false. For virtual memory with deferred
	// commit, set to true.
	EnableDeferredCommit bool

	// ProcessorCount sets the number of vCPU's. If `0` will default to platform
	// default.
	ProcessorCount int32

	// ProcessorLimit sets the maximum percentage of each vCPU's the UVM can
	// consume. If `0` will default to platform default.
	ProcessorLimit int32

	// ProcessorWeight sets the relative weight of these vCPU's vs another UVM's
	// when scheduling. If `0` will default to platform default.
	ProcessorWeight int32

	// StorageQoSIopsMaximum sets the maximum number of Iops. If `0` will
	// default to the platform default.
	StorageQoSIopsMaximum int32

	// StorageQoSIopsMaximum sets the maximum number of bytes per second. If `0`
	// will default to the platform default.
	StorageQoSBandwidthMaximum int32
}

// newDefaultOptions returns the default base options for WCOW and LCOW.
//
// If `id` is empty it will be generated.
//
// If `owner` is empty it will be set to the calling executables name.
func newDefaultOptions(id, owner string) *Options {
	opts := &Options{
		ID:                   id,
		Owner:                owner,
		MemorySizeInMB:       1024,
		AllowOvercommit:      true,
		EnableDeferredCommit: false,
		ProcessorCount:       defaultProcessorCount(),
	}

	if opts.ID == "" {
		opts.ID = guid.New().String()
	}
	if opts.Owner == "" {
		opts.Owner = filepath.Base(os.Args[0])
	}

	return opts
}

// ID returns the ID of the VM's compute system.
func (uvm *UtilityVM) ID() string {
	return uvm.hcsSystem.ID()
}

// OS returns the operating system of the utility VM.
func (uvm *UtilityVM) OS() string {
	return uvm.operatingSystem
}

// Close terminates and releases resources associated with the utility VM.
func (uvm *UtilityVM) Close() (err error) {
	op := "uvm::Close"
	log := logrus.WithFields(logrus.Fields{
		logfields.UVMID: uvm.id,
	})
	log.Debug(op + " - Begin Operation")
	defer func() {
		if err != nil {
			log.Data[logrus.ErrorKey] = err
			log.Error(op + " - End Operation - Error")
		} else {
			log.Debug(op + " - End Operation - Success")
		}
	}()

	if err := uvm.hcsSystem.Terminate(); hcs.IsPending(err) {
		uvm.Wait()
	}

	// outputListener will only be nil for a Create -> Stop without a Start. In
	// this case we have no goroutine processing output so its safe to close the
	// channel here.
	if uvm.outputListener != nil {
		close(uvm.outputProcessingDone)
		uvm.outputListener.Close()
		uvm.outputListener = nil
	}
	return uvm.hcsSystem.Close()
}

func defaultProcessorCount() int32 {
	if runtime.NumCPU() == 1 {
		return 1
	}
	return 2
}

// normalizeProcessorCount sets `uvm.processorCount` to `Min(requested,
// runtime.NumCPU())`.
func (uvm *UtilityVM) normalizeProcessorCount(requested int32) {
	hostCount := int32(runtime.NumCPU())
	if requested > hostCount {
		logrus.WithFields(logrus.Fields{
			logfields.UVMID: uvm.id,
		}).Warningf("Changing user requested CPUCount: %d to current number of processors: %d",
			requested,
			hostCount)
		uvm.processorCount = hostCount
	} else {
		uvm.processorCount = requested
	}
}

// ProcessorCount returns the number of processors actually assigned to the UVM.
func (uvm *UtilityVM) ProcessorCount() int32 {
	return uvm.processorCount
}

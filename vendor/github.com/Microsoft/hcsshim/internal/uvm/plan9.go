package uvm

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Microsoft/hcsshim/internal/guestrequest"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/requesttype"
	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
)

type Plan9Share struct {
	name, uvmPath string
}

const plan9Port = 564

// AddPlan9 adds a Plan9 share to a utility VM.
func (uvm *UtilityVM) AddPlan9(hostPath string, uvmPath string, readOnly bool, restrict bool, allowedNames []string) (_ *Plan9Share, err error) {
	op := "uvm::AddPlan9"
	log := logrus.WithFields(logrus.Fields{
		logfields.UVMID: uvm.id,
		"host-path":     hostPath,
		"uvm-path":      uvmPath,
		"readOnly":      readOnly,
		"restrict":      restrict,
		"allowedNames":  allowedNames,
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

	if uvm.operatingSystem != "linux" {
		return nil, errNotSupported
	}
	if restrict && osversion.Get().Build < 18328 {
		return nil, errors.New("single-file mappings are not supported on this build of Windows")
	}
	if uvmPath == "" {
		return nil, fmt.Errorf("uvmPath must be passed to AddPlan9")
	}

	// TODO: JTERRY75 - These are marked private in the schema. For now use them
	// but when there are public variants we need to switch to them.
	const (
		shareFlagsReadOnly           int32 = 0x00000001
		shareFlagsLinuxMetadata      int32 = 0x00000004
		shareFlagsCaseSensitive      int32 = 0x00000008
		shareFlagsRestrictFileAccess int32 = 0x00000080
	)

	// TODO: JTERRY75 - `shareFlagsCaseSensitive` only works if the Windows
	// `hostPath` supports case sensitivity. We need to detect this case before
	// forwarding this flag in all cases.
	flags := shareFlagsLinuxMetadata // | shareFlagsCaseSensitive
	if readOnly {
		flags |= shareFlagsReadOnly
	}
	if restrict {
		flags |= shareFlagsRestrictFileAccess
	}

	uvm.m.Lock()
	index := uvm.plan9Counter
	uvm.plan9Counter++
	uvm.m.Unlock()
	name := strconv.FormatUint(index, 10)

	modification := &hcsschema.ModifySettingRequest{
		RequestType: requesttype.Add,
		Settings: hcsschema.Plan9Share{
			Name:         name,
			AccessName:   name,
			Path:         hostPath,
			Port:         plan9Port,
			Flags:        flags,
			AllowedFiles: allowedNames,
		},
		ResourcePath: fmt.Sprintf("VirtualMachine/Devices/Plan9/Shares"),
		GuestRequest: guestrequest.GuestRequest{
			ResourceType: guestrequest.ResourceTypeMappedDirectory,
			RequestType:  requesttype.Add,
			Settings: guestrequest.LCOWMappedDirectory{
				MountPath: uvmPath,
				ShareName: name,
				Port:      plan9Port,
				ReadOnly:  readOnly,
			},
		},
	}

	if err := uvm.Modify(modification); err != nil {
		return nil, err
	}

	share := &Plan9Share{name: name, uvmPath: uvmPath}
	return share, nil
}

// RemovePlan9 removes a Plan9 share from a utility VM. Each Plan9 share is ref-counted
// and only actually removed when the ref-count drops to zero.
func (uvm *UtilityVM) RemovePlan9(share *Plan9Share) (err error) {
	op := "uvm::RemovePlan9"
	log := logrus.WithFields(logrus.Fields{
		logfields.UVMID: uvm.id,
		"name":          share.name,
		"uvm-path":      share.uvmPath,
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

	if uvm.operatingSystem != "linux" {
		return errNotSupported
	}

	modification := &hcsschema.ModifySettingRequest{
		RequestType: requesttype.Remove,
		Settings: hcsschema.Plan9Share{
			Name:       share.name,
			AccessName: share.name,
			Port:       plan9Port,
		},
		ResourcePath: fmt.Sprintf("VirtualMachine/Devices/Plan9/Shares"),
		GuestRequest: guestrequest.GuestRequest{
			ResourceType: guestrequest.ResourceTypeMappedDirectory,
			RequestType:  requesttype.Remove,
			Settings: guestrequest.LCOWMappedDirectory{
				MountPath: share.uvmPath,
				ShareName: share.name,
				Port:      plan9Port,
			},
		},
	}
	if err := uvm.Modify(modification); err != nil {
		return fmt.Errorf("failed to remove plan9 share %s from %s: %+v: %s", share.name, uvm.id, modification, err)
	}
	return nil
}

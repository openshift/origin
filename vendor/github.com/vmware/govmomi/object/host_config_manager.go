// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

type HostConfigManager struct {
	Common
}

func NewHostConfigManager(c *vim25.Client, ref types.ManagedObjectReference) *HostConfigManager {
	return &HostConfigManager{
		Common: NewCommon(c, ref),
	}
}

// reference returns the ManagedObjectReference for the given HostConfigManager property name.
// An error is returned if the field is nil, of type ErrNotSupported if versioned is true.
func (m HostConfigManager) reference(ctx context.Context, name string, versioned ...bool) (types.ManagedObjectReference, error) {
	prop := "configManager." + name
	var content []types.ObjectContent

	err := m.Properties(ctx, m.Reference(), []string{prop}, &content)
	if err != nil {
		return types.ManagedObjectReference{}, err
	}

	for _, c := range content {
		for _, p := range c.PropSet {
			if p.Name != prop {
				continue
			}
			if ref, ok := p.Val.(types.ManagedObjectReference); ok {
				return ref, nil
			}
		}
	}

	err = fmt.Errorf("%s %s is nil", m.Reference(), prop)
	if len(versioned) == 1 && versioned[0] {
		err = ErrNotSupported
	}
	return types.ManagedObjectReference{}, err
}

func (m HostConfigManager) DatastoreSystem(ctx context.Context) (*HostDatastoreSystem, error) {
	ref, err := m.reference(ctx, "datastoreSystem")
	if err != nil {
		return nil, err
	}
	return NewHostDatastoreSystem(m.c, ref), nil
}

func (m HostConfigManager) NetworkSystem(ctx context.Context) (*HostNetworkSystem, error) {
	ref, err := m.reference(ctx, "networkSystem")
	if err != nil {
		return nil, err
	}
	return NewHostNetworkSystem(m.c, ref), nil
}

func (m HostConfigManager) FirewallSystem(ctx context.Context) (*HostFirewallSystem, error) {
	ref, err := m.reference(ctx, "firewallSystem")
	if err != nil {
		return nil, err
	}

	return NewHostFirewallSystem(m.c, ref), nil
}

func (m HostConfigManager) StorageSystem(ctx context.Context) (*HostStorageSystem, error) {
	ref, err := m.reference(ctx, "storageSystem")
	if err != nil {
		return nil, err
	}
	return NewHostStorageSystem(m.c, ref), nil
}

func (m HostConfigManager) VirtualNicManager(ctx context.Context) (*HostVirtualNicManager, error) {
	ref, err := m.reference(ctx, "virtualNicManager")
	if err != nil {
		return nil, err
	}
	return NewHostVirtualNicManager(m.c, ref, m.Reference()), nil
}

func (m HostConfigManager) VsanSystem(ctx context.Context) (*HostVsanSystem, error) {
	ref, err := m.reference(ctx, "vsanSystem", true) // Added in 5.5
	if err != nil {
		return nil, err
	}
	return NewHostVsanSystem(m.c, ref), nil
}

func (m HostConfigManager) VsanInternalSystem(ctx context.Context) (*HostVsanInternalSystem, error) {
	ref, err := m.reference(ctx, "vsanInternalSystem", true) // Added in 5.5
	if err != nil {
		return nil, err
	}
	return NewHostVsanInternalSystem(m.c, ref), nil
}

func (m HostConfigManager) AccountManager(ctx context.Context) (*HostAccountManager, error) {
	ref, err := m.reference(ctx, "accountManager", true) // Added in 5.5
	if err != nil {
		if err == ErrNotSupported {
			// Versions < 5.5 can use the ServiceContent ref,
			// but only when connected directly to ESX.
			if m.c.ServiceContent.AccountManager == nil {
				return nil, err
			}
			ref = *m.c.ServiceContent.AccountManager
		} else {
			return nil, err
		}
	}

	return NewHostAccountManager(m.c, ref), nil
}

func (m HostConfigManager) OptionManager(ctx context.Context) (*OptionManager, error) {
	ref, err := m.reference(ctx, "advancedOption")
	if err != nil {
		return nil, err
	}
	return NewOptionManager(m.c, ref), nil
}

func (m HostConfigManager) ServiceSystem(ctx context.Context) (*HostServiceSystem, error) {
	ref, err := m.reference(ctx, "serviceSystem")
	if err != nil {
		return nil, err
	}
	return NewHostServiceSystem(m.c, ref), nil
}

func (m HostConfigManager) CertificateManager(ctx context.Context) (*HostCertificateManager, error) {
	ref, err := m.reference(ctx, "certificateManager", true) // Added in 6.0
	if err != nil {
		return nil, err
	}
	return NewHostCertificateManager(m.c, ref, m.Reference()), nil
}

func (m HostConfigManager) DateTimeSystem(ctx context.Context) (*HostDateTimeSystem, error) {
	ref, err := m.reference(ctx, "dateTimeSystem")
	if err != nil {
		return nil, err
	}
	return NewHostDateTimeSystem(m.c, ref), nil
}

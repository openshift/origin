// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type CustomizationSpecManager struct {
	Common
}

func NewCustomizationSpecManager(c *vim25.Client) *CustomizationSpecManager {
	cs := CustomizationSpecManager{
		Common: NewCommon(c, *c.ServiceContent.CustomizationSpecManager),
	}

	return &cs
}

func (cs CustomizationSpecManager) Info(ctx context.Context) ([]types.CustomizationSpecInfo, error) {
	var m mo.CustomizationSpecManager
	err := cs.Properties(ctx, cs.Reference(), []string{"info"}, &m)
	return m.Info, err
}

func (cs CustomizationSpecManager) DoesCustomizationSpecExist(ctx context.Context, name string) (bool, error) {
	req := types.DoesCustomizationSpecExist{
		This: cs.Reference(),
		Name: name,
	}

	res, err := methods.DoesCustomizationSpecExist(ctx, cs.c, &req)

	if err != nil {
		return false, err
	}

	return res.Returnval, nil
}

func (cs CustomizationSpecManager) GetCustomizationSpec(ctx context.Context, name string) (*types.CustomizationSpecItem, error) {
	req := types.GetCustomizationSpec{
		This: cs.Reference(),
		Name: name,
	}

	res, err := methods.GetCustomizationSpec(ctx, cs.c, &req)

	if err != nil {
		return nil, err
	}

	return &res.Returnval, nil
}

func (cs CustomizationSpecManager) CreateCustomizationSpec(ctx context.Context, item types.CustomizationSpecItem) error {
	req := types.CreateCustomizationSpec{
		This: cs.Reference(),
		Item: item,
	}

	_, err := methods.CreateCustomizationSpec(ctx, cs.c, &req)
	if err != nil {
		return err
	}

	return nil
}

func (cs CustomizationSpecManager) OverwriteCustomizationSpec(ctx context.Context, item types.CustomizationSpecItem) error {
	req := types.OverwriteCustomizationSpec{
		This: cs.Reference(),
		Item: item,
	}

	_, err := methods.OverwriteCustomizationSpec(ctx, cs.c, &req)
	if err != nil {
		return err
	}

	return nil
}

func (cs CustomizationSpecManager) DeleteCustomizationSpec(ctx context.Context, name string) error {
	req := types.DeleteCustomizationSpec{
		This: cs.Reference(),
		Name: name,
	}

	_, err := methods.DeleteCustomizationSpec(ctx, cs.c, &req)
	if err != nil {
		return err
	}

	return nil
}

func (cs CustomizationSpecManager) DuplicateCustomizationSpec(ctx context.Context, name string, newName string) error {
	req := types.DuplicateCustomizationSpec{
		This:    cs.Reference(),
		Name:    name,
		NewName: newName,
	}

	_, err := methods.DuplicateCustomizationSpec(ctx, cs.c, &req)
	if err != nil {
		return err
	}

	return nil
}

func (cs CustomizationSpecManager) RenameCustomizationSpec(ctx context.Context, name string, newName string) error {
	req := types.RenameCustomizationSpec{
		This:    cs.Reference(),
		Name:    name,
		NewName: newName,
	}

	_, err := methods.RenameCustomizationSpec(ctx, cs.c, &req)
	if err != nil {
		return err
	}

	return nil
}

func (cs CustomizationSpecManager) CustomizationSpecItemToXml(ctx context.Context, item types.CustomizationSpecItem) (string, error) {
	req := types.CustomizationSpecItemToXml{
		This: cs.Reference(),
		Item: item,
	}

	res, err := methods.CustomizationSpecItemToXml(ctx, cs.c, &req)
	if err != nil {
		return "", err
	}

	return res.Returnval, nil
}

func (cs CustomizationSpecManager) XmlToCustomizationSpecItem(ctx context.Context, xml string) (*types.CustomizationSpecItem, error) {
	req := types.XmlToCustomizationSpecItem{
		This:        cs.Reference(),
		SpecItemXml: xml,
	}

	res, err := methods.XmlToCustomizationSpecItem(ctx, cs.c, &req)
	if err != nil {
		return nil, err
	}
	return &res.Returnval, nil
}

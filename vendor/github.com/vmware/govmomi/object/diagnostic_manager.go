// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type DiagnosticManager struct {
	Common
}

func NewDiagnosticManager(c *vim25.Client) *DiagnosticManager {
	m := DiagnosticManager{
		Common: NewCommon(c, *c.ServiceContent.DiagnosticManager),
	}

	return &m
}

func (m DiagnosticManager) Log(ctx context.Context, host *HostSystem, key string) *DiagnosticLog {
	return &DiagnosticLog{
		m:    m,
		Key:  key,
		Host: host,
	}
}

func (m DiagnosticManager) BrowseLog(ctx context.Context, host *HostSystem, key string, start, lines int32) (*types.DiagnosticManagerLogHeader, error) {
	req := types.BrowseDiagnosticLog{
		This:  m.Reference(),
		Key:   key,
		Start: start,
		Lines: lines,
	}

	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	res, err := methods.BrowseDiagnosticLog(ctx, m.Client(), &req)
	if err != nil {
		return nil, err
	}

	return &res.Returnval, nil
}

func (m DiagnosticManager) GenerateLogBundles(ctx context.Context, includeDefault bool, host []*HostSystem) (*Task, error) {
	req := types.GenerateLogBundles_Task{
		This:           m.Reference(),
		IncludeDefault: includeDefault,
	}

	for _, h := range host {
		req.Host = append(req.Host, h.Reference())
	}

	res, err := methods.GenerateLogBundles_Task(ctx, m.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(m.c, res.Returnval), nil
}

func (m DiagnosticManager) QueryDescriptions(ctx context.Context, host *HostSystem) ([]types.DiagnosticManagerLogDescriptor, error) {
	req := types.QueryDescriptions{
		This: m.Reference(),
	}

	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	res, err := methods.QueryDescriptions(ctx, m.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

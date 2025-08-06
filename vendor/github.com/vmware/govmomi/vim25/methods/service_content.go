// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package methods

import (
	"context"
	"time"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// copy of vim25.ServiceInstance to avoid import cycle
var serviceInstance = types.ManagedObjectReference{
	Type:  "ServiceInstance",
	Value: "ServiceInstance",
}

func GetServiceContent(ctx context.Context, r soap.RoundTripper) (types.ServiceContent, error) {
	req := types.RetrieveServiceContent{
		This: serviceInstance,
	}

	res, err := RetrieveServiceContent(ctx, r, &req)
	if err != nil {
		return types.ServiceContent{}, err
	}

	return res.Returnval, nil
}

func GetCurrentTime(ctx context.Context, r soap.RoundTripper) (*time.Time, error) {
	req := types.CurrentTime{
		This: serviceInstance,
	}

	res, err := CurrentTime(ctx, r, &req)
	if err != nil {
		return nil, err
	}

	return &res.Returnval, nil
}

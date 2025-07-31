// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package methods

import (
	"context"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

type PlaceVmsXClusterBody struct {
	Req    *types.PlaceVmsXCluster         `xml:"urn:vim25 PlaceVmsXCluster,omitempty"`
	Res    *types.PlaceVmsXClusterResponse `xml:"PlaceVmsXClusterResponse,omitempty"`
	Fault_ *soap.Fault                     `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault,omitempty"`
}

func (b *PlaceVmsXClusterBody) Fault() *soap.Fault { return b.Fault_ }

func PlaceVmsXCluster(ctx context.Context, r soap.RoundTripper, req *types.PlaceVmsXCluster) (*types.PlaceVmsXClusterResponse, error) {
	var reqBody, resBody PlaceVmsXClusterBody

	reqBody.Req = req

	if err := r.RoundTrip(ctx, &reqBody, &resBody); err != nil {
		return nil, err
	}

	return resBody.Res, nil
}

// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

type DisabledMethodRequest struct {
	Method string `xml:"method"`
	Reason string `xml:"reasonId"`
}

type disableMethodsRequest struct {
	This   types.ManagedObjectReference   `xml:"_this"`
	Entity []types.ManagedObjectReference `xml:"entity"`
	Method []DisabledMethodRequest        `xml:"method"`
	Source string                         `xml:"sourceId"`
	Scope  bool                           `xml:"sessionScope,omitempty"`
}

type disableMethodsBody struct {
	Req *disableMethodsRequest `xml:"urn:internalvim25 DisableMethods,omitempty"`
	Res any                    `xml:"urn:vim25 DisableMethodsResponse,omitempty"`
	Err *soap.Fault            `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault,omitempty"`
}

func (b *disableMethodsBody) Fault() *soap.Fault { return b.Err }

func (m AuthorizationManager) DisableMethods(ctx context.Context, entity []types.ManagedObjectReference, method []DisabledMethodRequest, source string) error {
	var reqBody, resBody disableMethodsBody

	reqBody.Req = &disableMethodsRequest{
		This:   m.Reference(),
		Entity: entity,
		Method: method,
		Source: source,
	}

	return m.Client().RoundTrip(ctx, &reqBody, &resBody)
}

type enableMethodsRequest struct {
	This   types.ManagedObjectReference   `xml:"_this"`
	Entity []types.ManagedObjectReference `xml:"entity"`
	Method []string                       `xml:"method"`
	Source string                         `xml:"sourceId"`
}

type enableMethodsBody struct {
	Req *enableMethodsRequest `xml:"urn:internalvim25 EnableMethods,omitempty"`
	Res any                   `xml:"urn:vim25 EnableMethodsResponse,omitempty"`
	Err *soap.Fault           `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault,omitempty"`
}

func (b *enableMethodsBody) Fault() *soap.Fault { return b.Err }

func (m AuthorizationManager) EnableMethods(ctx context.Context, entity []types.ManagedObjectReference, method []string, source string) error {
	var reqBody, resBody enableMethodsBody

	reqBody.Req = &enableMethodsRequest{
		This:   m.Reference(),
		Entity: entity,
		Method: method,
		Source: source,
	}

	return m.Client().RoundTrip(ctx, &reqBody, &resBody)
}

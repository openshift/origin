// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"time"

	"github.com/vmware/govmomi/session/keepalive"
	"github.com/vmware/govmomi/vim25/soap"
)

// KeepAlive is a backward compatible wrapper around KeepAliveHandler.
func KeepAlive(roundTripper soap.RoundTripper, idleTime time.Duration) soap.RoundTripper {
	return KeepAliveHandler(roundTripper, idleTime, nil)
}

// KeepAliveHandler is a backward compatible wrapper around keepalive.NewHandlerSOAP.
func KeepAliveHandler(roundTripper soap.RoundTripper, idleTime time.Duration, handler func(soap.RoundTripper) error) soap.RoundTripper {
	var f func() error
	if handler != nil {
		f = func() error {
			return handler(roundTripper)
		}
	}
	return keepalive.NewHandlerSOAP(roundTripper, idleTime, f)
}

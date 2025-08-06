package bmc

import (
	"context"

	"github.com/gebn/bmc/pkg/ipmi"
)

// SessionlessCommands contains high-level methods to issue commands that do not
// require a session, but can be sent using one. This is IPMI version agnostic,
// so RMCP+ session setup payload types do not appear, only commands that would
// come under the IPMI message standard payload type.
type SessionlessCommands interface {

	// GetSystemGUID issues the Get System GUID command to the BMC, specified in
	// section 18.13 of IPMI v1.5 and 22.14 of v2.0. You may wish to use a
	// library like google/uuid to manipulate and display the GUID.
	GetSystemGUID(context.Context) ([16]byte, error)

	// GetChannelAuthenticationCapabilities issues the Get Channel
	// Authentication Capabilities command to the BMC, specified in 18.12 of
	// IPMI v1.5 and 22.13 of v2.0. This is mainly useful for this library when
	// deciding how to connect to a BMC (e.g. whether to use v1.5 or v2.0) and
	// as a keepalive, however could be useful to scan an estate for
	// compatibility.
	GetChannelAuthenticationCapabilities(context.Context, *ipmi.GetChannelAuthenticationCapabilitiesReq) (*ipmi.GetChannelAuthenticationCapabilitiesRsp, error)
}

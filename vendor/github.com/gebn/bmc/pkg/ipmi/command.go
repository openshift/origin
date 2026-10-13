package ipmi

import (
	"github.com/google/gopacket"
)

// Command represents the request and response parts (if any) of executing a
// function against a managed system. Implementations of this interface have the
// -Cmd suffix.
//
// Allocating the command should allocate both its request and response layers -
// if we allocate a request, the chances are we will be sending it and so will
// need the response, so this reduces load on GC. It is recommended to implement
// this interface using a struct with up to two value layers.
//
// Note that this interface only applies to activity below the message layer,
// i.e. with a NetFn and command number. RMCP+ session setup payloads
// (RAKP1/2/3/4, RMCP+ Open Session Req/Rsp), while they could be considered
// commands, do not fall into this category. This turns out not to be so bad, as
// they are not on the hot path, so it is nice to be able to treat them
// differently.
type Command interface {

	// Name returns the name of the command, without request/response suffix
	// e.g. "Get Device ID". This is used for metrics.
	Name() string

	// Operation returns the operation parameters for the request. This should
	// avoid allocation, referencing a value in static memory. Technically, this
	// should be a member of a Request interface that embeds
	// gopacket.SerializableLayer, however it is here to allow Request() to
	// return nil for commands not requiring a request payload, which would
	// otherwise need to have a no-op layer created.
	Operation() *Operation

	// RemoteLUN is set in the Message layer. This is usually LUNBMC, however
	// some sensors may come under OEM LUNs, requiring a Get Sensor Reading
	// invocation to use these.
	RemoteLUN() LUN

	// Request returns the possibly-nil request layer that we send to the
	// managed system. This should not allocate any additional memory.
	Request() gopacket.SerializableLayer

	// Response returns the possibly-nil response layer that we expect back from
	// the managed system following our request. This should not allocate any
	// additional memory.
	Response() gopacket.DecodingLayer
}

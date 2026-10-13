package ipmi

import (
	"github.com/google/gopacket"
)

// Payload is implemented by outgoing IPMI v2.0 RMCP+ session setup
// interactions. Implementations have a -Payload suffix.
//
// Note that although "IPMI Message" is payload, those interactions do not use
// this interface, as there is an additional Message layer that cannot be
// handled neatly and efficiently. This interface is for layers that are sent
// and received directly below the V2Session layer, outside of a session.
//
// It is convention for structs implementing this interface to have Req and Rsp
// value fields for the Request() and Response() respectively.
type Payload interface {

	// Descriptor returns the PayloadDescriptor for the request layer. This
	// should avoid allocation, returning a pointer to static memory.
	// Technically this should be a member of the Request(), however we put it
	// here for consistency with Command. Note that unlike in Command, Request()
	// and Response() here cannot return nil.
	Descriptor() *PayloadDescriptor

	// Request returns the request layer that we send to the managed system,
	// immediately after the null V2Session layer. This should not allocate any
	// additional memory, and must not return nil.
	Request() gopacket.SerializableLayer

	// Response returns the response layer that we expect back from the managed
	// system following our request. This should not allocate any additional
	// memory, and must not return nil.
	Response() gopacket.DecodingLayer
}

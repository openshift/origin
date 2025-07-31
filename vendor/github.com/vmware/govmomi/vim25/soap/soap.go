// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package soap

import (
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vim25/xml"
)

// HeaderElement allows changing the default XMLName (e.g. Cookie's default of vcSessionCookie)
type HeaderElement struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// Header includes optional soap Header fields.
type Header struct {
	Action   string         `xml:"-"`                         // Action is the 'SOAPAction' HTTP header value. Defaults to "Client.Namespace/Client.Version".
	Cookie   *HeaderElement `xml:"vcSessionCookie,omitempty"` // Cookie is a vCenter session cookie that can be used with other SDK endpoints (e.g. pbm, vslm).
	ID       string         `xml:"operationID,omitempty"`     // ID is the operationID used by ESX/vCenter logging for correlation.
	Security any            `xml:",omitempty"`                // Security is used for SAML token authentication and request signing.
}

type Envelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Header  *Header  `xml:"http://schemas.xmlsoap.org/soap/envelope/ Header,omitempty"`
	Body    any
}

type Fault struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	Code    string   `xml:"faultcode"`
	String  string   `xml:"faultstring"`
	Detail  struct {
		Fault types.AnyType `xml:",any,typeattr"`
	} `xml:"detail"`
}

func (f *Fault) VimFault() types.AnyType {
	return f.Detail.Fault
}

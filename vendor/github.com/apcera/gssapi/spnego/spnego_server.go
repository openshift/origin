// Copyright 2015 Apcera Inc. All rights reserved.

// This is intended to give an interface for Kerberized servers to negotiate
// with clients using SPNEGO. A reference implementation is provided below.
package spnego

import (
	"errors"
	"net/http"

	"github.com/apcera/gssapi"
)

// A ServerNegotiator is an interface that defines minimal functionality for
// SPNEGO and credential issuance using GSSAPI from the server side.
type ServerNegotiator interface {
	// AcquireCred acquires a credential from the server's environment.
	AcquireCred(string) (*gssapi.CredId, error)

	// Negotiate handles the negotiation with the client.
	Negotiate(*gssapi.CredId, http.Header, http.Header) (string, int, error)
}

// A KerberizedServer allows a server to negotiate authentication over SPNEGO
// with a client.
type KerberizedServer struct {
	*gssapi.Lib
	UseProxyAuthentication bool
}

var _ ServerNegotiator = KerberizedServer{}

// AcquireCred acquires a Kerberos credential (keytab) from environment. The
// CredId MUST be released by the caller.
func (k KerberizedServer) AcquireCred(serviceName string) (*gssapi.CredId, error) {
	nameBuf, err := k.MakeBufferString(serviceName)
	if err != nil {
		return nil, err
	}
	defer nameBuf.Release()

	name, err := nameBuf.Name(k.GSS_KRB5_NT_PRINCIPAL_NAME)
	if err != nil {
		return nil, err
	}
	defer name.Release()

	cred, actualMechs, _, err := k.Lib.AcquireCred(name,
		gssapi.GSS_C_INDEFINITE, k.GSS_C_NO_OID_SET, gssapi.GSS_C_ACCEPT)
	if err != nil {
		return nil, err
	}
	defer actualMechs.Release()

	return cred, nil
}

// Negotiate handles the SPNEGO client-server negotiation. Negotiate will likely
// be invoked multiple times; a 200 or 400 response code are terminating
// conditions, whereas a 401 or 407 means that the client should respond to the
// challenge that we send.
func (k KerberizedServer) Negotiate(cred *gssapi.CredId, inHeader, outHeader http.Header) (string, int, error) {
	var challengeHeader, authHeader string
	var challengeStatus int
	if k.UseProxyAuthentication {
		challengeHeader = "Proxy-Authenticate"
		challengeStatus = http.StatusProxyAuthRequired
		authHeader = "Proxy-Authorization"
	} else {
		challengeHeader = "WWW-Authenticate"
		challengeStatus = http.StatusUnauthorized
		authHeader = "Authorization"
	}

	negotiate, inputToken := CheckSPNEGONegotiate(k.Lib, inHeader, authHeader)
	defer inputToken.Release()

	// Here, challenge the client to initiate the security context. The first
	// request a client has made will often be unauthenticated, so we return a
	// 401, which the client handles.
	if !negotiate || inputToken.Length() == 0 {
		AddSPNEGONegotiate(outHeader, challengeHeader, inputToken)
		return "", challengeStatus, errors.New("SPNEGO: unauthorized")
	}

	// FIXME: GSS_S_CONTINUED_NEEDED handling?
	ctx, srcName, _, outputToken, _, _, delegatedCredHandle, err :=
		k.AcceptSecContext(k.GSS_C_NO_CONTEXT,
			cred, inputToken, k.GSS_C_NO_CHANNEL_BINDINGS)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	delegatedCredHandle.Release()
	ctx.DeleteSecContext()
	outputToken.Release()
	defer srcName.Release()

	return srcName.String(), http.StatusOK, nil
}

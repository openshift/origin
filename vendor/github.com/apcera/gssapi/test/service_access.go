// Copyright 2013-2015 Apcera Inc. All rights reserved.

package test

import (
	"net/http"

	"github.com/apcera/gssapi"
	"github.com/apcera/gssapi/spnego"
)

func HandleAccess(c *Context, w http.ResponseWriter, r *http.Request) (code int, message string) {
	ctx, code, message := allowed(c, w, r)
	if ctx == nil {
		return code, message
	}

	w.Write([]byte("OK"))
	return http.StatusOK, "OK"
}

// allowed implements the SPNEGO protocol. When the request is to be passed
// through, it returns http.StatusOK and a valid gssapi CtxId object.
// Otherwise, it sets the WWW-Authorization header as applicable, and returns
// http.StatusUnathorized.
func allowed(c *Context, w http.ResponseWriter, r *http.Request) (
	ctx *gssapi.CtxId, code int, message string) {

	// returning a 401 with a challenge, but no token will make the client
	// initiate security context and re-submit with a non-empty Authorization
	negotiate, inputToken := spnego.CheckSPNEGONegotiate(c.Lib, r.Header, "Authorization")
	if !negotiate || inputToken.Length() == 0 {
		spnego.AddSPNEGONegotiate(w.Header(), "WWW-Authenticate", nil)
		return nil, http.StatusUnauthorized, "no input token provided"
	}

	ctx, srcName, _, outputToken, _, _, delegatedCredHandle, err :=
		c.AcceptSecContext(c.GSS_C_NO_CONTEXT,
			c.credential, inputToken, c.GSS_C_NO_CHANNEL_BINDINGS)

	//TODO: special case handling of GSS_S_CONTINUE_NEEDED
	// but it doesn't change the logic, still fail
	if err != nil {
		//TODO: differentiate invalid tokens here and return a 403
		//TODO: add a test for a bad and maybe an expired auth tokens
		return nil, http.StatusInternalServerError, err.Error()
	}

	srcName.Release()
	delegatedCredHandle.Release()

	spnego.AddSPNEGONegotiate(w.Header(), "WWW-Authenticate", outputToken)
	return ctx, http.StatusOK, "pass"
}

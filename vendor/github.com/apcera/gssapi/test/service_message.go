// Copyright 2013-2015 Apcera Inc. All rights reserved.

package test

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"

	"github.com/apcera/gssapi"
)

// This test handler accepts the context, unwraps, and then re-wraps the request body
func HandleUnwrap(c *Context, w http.ResponseWriter, r *http.Request) (code int, message string) {
	ctx, code, message := allowed(c, w, r)
	if ctx == nil {
		return code, message
	}

	// Unwrap the request
	wrappedbytes, err := ioutil.ReadAll(
		base64.NewDecoder(base64.StdEncoding, r.Body))
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	wrapped, err := c.MakeBufferBytes(wrappedbytes)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	defer wrapped.Release()

	unwrapped, _, _, err := ctx.Unwrap(wrapped)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	defer unwrapped.Release()

	// Re-wrap the for the response
	_, wrapped, err = ctx.Wrap(true, gssapi.GSS_C_QOP_DEFAULT, unwrapped)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	defer wrapped.Release()

	wrapped64 := base64.StdEncoding.EncodeToString(wrapped.Bytes())
	w.Write([]byte(wrapped64))
	return http.StatusOK, "OK"
}

func HandleVerifyMIC(c *Context, w http.ResponseWriter, r *http.Request) (code int, message string) {
	ctx, code, message := allowed(c, w, r)
	if ctx == nil {
		return code, message
	}

	mic64 := r.Header.Get(micHeader)
	if mic64 == "" {
		return http.StatusInternalServerError, "No " + micHeader + " header"
	}
	micbytes, err := base64.StdEncoding.DecodeString(mic64)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	mic, err := c.MakeBufferBytes(micbytes)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	bodybytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	body, err := c.MakeBufferBytes(bodybytes)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	_, err = ctx.VerifyMIC(body, mic)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	w.Write([]byte("OK"))
	return http.StatusOK, "OK"
}

// Copyright 2013-2015 Apcera Inc. All rights reserved.

// +build clienttest

package test

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/apcera/gssapi"
)

func TestClientWrap(t *testing.T) {
	b := "test message in body"

	bodyf := func(ctx *gssapi.CtxId) string {
		// Wrap and send a message to the service
		buf, err := c.MakeBufferString(b)
		if err != nil {
			t.Fatal(err)
		}
		defer buf.Release()

		_, wrapped, err := ctx.Wrap(true, gssapi.GSS_C_QOP_DEFAULT, buf)
		if err != nil {
			t.Fatal(err)
		}
		defer wrapped.Release()

		return base64.StdEncoding.EncodeToString(wrapped.Bytes())
	}

	ctx, r := initClientContext(t, "POST", "/unwrap/", bodyf)
	defer ctx.Release()

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// if successful, the response body is the same message, re-wrapped by
	// the service, unwrap and compare
	wrapped64bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	wrappedbytes, err := base64.StdEncoding.DecodeString(string(wrapped64bytes))
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := c.MakeBufferBytes(wrappedbytes)
	if err != nil {
		t.Fatal(err)
	}
	defer wrapped.Release()
	unwrapped, _, _, err := ctx.Unwrap(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	defer unwrapped.Release()

	if unwrapped.String() != b {
		t.Fatalf("Got %q, expected %q", unwrapped.String(), b)
	}
}

func TestClientMIC(t *testing.T) {
	b := "test message in body"

	ctx, r := initClientContext(t, "POST", "/verify_mic/",
		func(ctx *gssapi.CtxId) string {
			return b
		})
	defer ctx.Release()

	body, err := c.MakeBufferString(b)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Release()

	mic, err := ctx.GetMIC(gssapi.GSS_C_QOP_DEFAULT, body)
	if err != nil {
		t.Fatal(err)
	}
	defer mic.Release()

	r.Header.Set(micHeader,
		base64.StdEncoding.EncodeToString(mic.Bytes()))

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %v, got %v", http.StatusOK, resp.StatusCode)
	}
}

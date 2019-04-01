//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package utils

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/heketi/tests"
)

type testDest struct {
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

type dummyReader struct {
	emsg string
}

type dummyCloser struct {
	io.Reader
}

func (d *dummyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New(d.emsg)
}

func (c dummyCloser) Close() error { return nil }

func TestGetJsonFromRequestOk(t *testing.T) {
	dest := &testDest{}
	req, err := http.NewRequest(
		http.MethodPut,
		"/foo/bar",
		bytes.NewBufferString(`{"foo": "MyTest", "bar": 88}`))
	tests.Assert(t, err == nil, "error calling http.NewRequest:", err)
	err = GetJsonFromRequest(req, dest)
	tests.Assert(t, err == nil, "error calling GetJsonFromRequest", err)
	tests.Assert(t, dest.Foo == "MyTest",
		`expected Foo == "MyTest", got:`, dest.Foo)
	tests.Assert(t, dest.Bar == 88,
		"expected Bar == 88, got:", dest.Bar)
}

func TestGetJsonFromRequestBadJson(t *testing.T) {
	dest := &testDest{}
	req, err := http.NewRequest(
		http.MethodPut,
		"/foo/bar",
		bytes.NewBufferString(`{"foo": `))
	tests.Assert(t, err == nil, "error calling http.NewRequest:", err)
	err = GetJsonFromRequest(req, dest)
	tests.Assert(t, err != nil,
		"expected error from GetJsonFromRequest, got nil")
}

func TestGetJsonFromRequestEmptyBuf(t *testing.T) {
	dest := &testDest{}
	req, err := http.NewRequest(
		http.MethodPut,
		"/foo/bar",
		new(bytes.Buffer))
	tests.Assert(t, err == nil, "error calling http.NewRequest:", err)
	err = GetJsonFromRequest(req, dest)
	tests.Assert(t, err != nil,
		"expected error from GetJsonFromRequest, got nil")
}

func TestGetJsonFromRequestBadIo(t *testing.T) {
	dest := &testDest{}
	req, err := http.NewRequest(
		http.MethodPut,
		"/foo/bar",
		&dummyReader{"ouch"})
	tests.Assert(t, err == nil, "error calling http.NewRequest:", err)
	err = GetJsonFromRequest(req, dest)
	tests.Assert(t, err != nil,
		"expected error from GetJsonFromRequest, got nil")
}

func TestGetJsonFromResponseOk(t *testing.T) {
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       dummyCloser{bytes.NewBufferString(`{"foo": "port", "bar": 22}`)},
	}
	dest := &testDest{}
	err := GetJsonFromResponse(resp, dest)
	tests.Assert(t, err == nil, "error calling GetJsonFromResponse:", err)
	tests.Assert(t, dest.Foo == "port", `expected Foo == "port", got:`, dest.Foo)
	tests.Assert(t, dest.Bar == 22, `expected Bar == 22, got:`, dest.Bar)
}

func TestGetJsonFromResponseBadJson(t *testing.T) {
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       dummyCloser{bytes.NewBufferString(`{_}`)},
	}
	dest := &testDest{}
	err := GetJsonFromResponse(resp, dest)
	tests.Assert(t, err != nil,
		"expected error calling GetJsonFromResponse, got nil")
}

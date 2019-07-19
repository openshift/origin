//
// Copyright (c) 2018 The heketi Authors
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
	"net/http"
	"testing"

	"github.com/heketi/tests"
)

type testGetStringFromResponseBody struct {
	Data string
}

func (b testGetStringFromResponseBody) Close() error { return nil }

func (r testGetStringFromResponseBody) Read(p []byte) (int, error) {
	if r.Data == "" {
		return 0, errors.New("bzzt")
	} else {
		return copy(p, r.Data), nil
	}
}

func TestGetStringFromResponseOK(t *testing.T) {
	bodytext := "Hello, Heketi!"
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          testGetStringFromResponseBody{bodytext},
		ContentLength: int64(len(bodytext)),
	}
	s, err := GetStringFromResponse(resp)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, s == "Hello, Heketi!",
		`expected s == "Hello, Heketi!", got:`, s)
}

func TestGetStringFromResponseShort(t *testing.T) {
	bodytext := "Hello, Heketi!"
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          testGetStringFromResponseBody{bodytext},
		ContentLength: int64(len(bodytext) - 4),
	}
	short := bodytext[:10]
	s, err := GetStringFromResponse(resp)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(s) == len(short),
		"expected len(s) == len(short), got:", len(s), len(short))
	tests.Assert(t, s == short,
		"expected s == short, got:", s, short)
}

func TestGetStringFromResponseBadRead(t *testing.T) {
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          testGetStringFromResponseBody{},
		ContentLength: 102,
	}
	s, err := GetStringFromResponse(resp)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, err.Error() == "bzzt",
		`expected err.Error() == "bzzt", got:`, err.Error())
	tests.Assert(t, s == "", `expected s == "", got:`, s)
}

func TestGetErrorFromResponseBadRead(t *testing.T) {
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          testGetStringFromResponseBody{},
		ContentLength: 102,
	}
	err := GetErrorFromResponse(resp)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, err.Error() == "bzzt",
		`expected err.Error() == "bzzt", got:`, err.Error())
}

func TestGetErrorFromResponseErrString(t *testing.T) {
	bodytext := "Something went horribly wrong"
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          dummyCloser{bytes.NewBufferString(bodytext)},
		ContentLength: int64(len(bodytext)),
	}
	err := GetErrorFromResponse(resp)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, err.Error() == bodytext,
		"expected err.Error() == bodytext, got", err.Error(), bodytext)
}

func TestGetErrorFromResponseErrStringSpace(t *testing.T) {
	bodytext := " whoa nellie\n"
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          dummyCloser{bytes.NewBufferString(bodytext)},
		ContentLength: int64(len(bodytext)),
	}
	err := GetErrorFromResponse(resp)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, err.Error() == "whoa nellie",
		`expected err.Error() == "whoa nellie", got:`, err.Error())
}

func TestGetErrorFromResponseEmptyString(t *testing.T) {
	bodytext := "\n"
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          dummyCloser{bytes.NewBufferString(bodytext)},
		ContentLength: int64(len(bodytext)),
	}
	err := GetErrorFromResponse(resp)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, err.Error() == "server did not provide a message (status 200: OK)",
		`expected err.Error() == "server did not provide a message (status 200: OK)", got:`, err.Error())
}

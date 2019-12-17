//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"strings"
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/executors"
)

func TestCheckHandle(t *testing.T) {
	var e error

	e = checkHandle(&executors.DeviceVgHandle{})
	tests.Assert(t, e != nil, "expected e != nil, got:", e)

	e = checkHandle(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{UUID: "bob"},
		VgId:         "abc123",
	})
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = checkHandle(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{Paths: []string{"bob"}},
		VgId:         "abc123",
	})
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = checkHandle(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{Paths: []string{"bob"}},
	})
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
}

func TestHandlePaths(t *testing.T) {
	var p []string

	p = handlePaths(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{
			Paths: []string{"/dev/bob"},
		},
	})
	tests.Assert(t, len(p) == 1)
	tests.Assert(t, p[0] == "/dev/bob")

	p = handlePaths(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{
			UUID: "abcdef1",
		},
	})
	tests.Assert(t, len(p) == 1)
	tests.Assert(t, p[0] == "/dev/disk/by-id/lvm-pv-uuid-abcdef1")

	p = handlePaths(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{
			UUID:  "abcdef1",
			Paths: []string{"/dev/bob"},
		},
	})
	tests.Assert(t, len(p) == 2)
	tests.Assert(t, p[0] == "/dev/disk/by-id/lvm-pv-uuid-abcdef1")
	tests.Assert(t, p[1] == "/dev/bob")

	p = handlePaths(&executors.DeviceVgHandle{
		DeviceHandle: executors.DeviceHandle{
			UUID:  "abcdef1",
			Paths: []string{"/dev/bob", "/dev/carla"},
		},
	})
	tests.Assert(t, len(p) == 3)
	tests.Assert(t, p[0] == "/dev/disk/by-id/lvm-pv-uuid-abcdef1")
	tests.Assert(t, p[1] == "/dev/bob")
	tests.Assert(t, p[2] == "/dev/carla")
}

func TestParsePvsResult(t *testing.T) {
	var (
		s   string
		err error
	)

	s, err = parsePvsResult(``)
	tests.Assert(t, err != nil)
	tests.Assert(t, strings.Contains(err.Error(), "Failed to parse"),
		`expected "Failed to parse" in err, got `, err)
	tests.Assert(t, s == "")

	s, err = parsePvsResult(`
{
 "report": [
  {
   "pv": [
    {"pv_name":"/dev/foo", "pv_uuid":"abcdef1", "vg_name":"bloop"}
   ]
  }
 ]
}
`)
	tests.Assert(t, err == nil)
	tests.Assert(t, s == "abcdef1", "expected abcdef1, got:", s)

	// the function accepts vgs output as well
	s, err = parsePvsResult(`
{
 "report": [
  {
   "vg": [
    {"pv_name":"/dev/foo", "pv_uuid":"1337b33f", "vg_name":"bloop"}
   ]
  }
 ]
}
`)
	tests.Assert(t, err == nil)
	tests.Assert(t, s == "1337b33f", "expected 1337b33f, got:", s)

	// make sure multiple reports fails
	s, err = parsePvsResult(`
{
 "report": [
  {
   "pv": [
    {"pv_name":"/dev/foo", "pv_uuid":"abcdef1", "vg_name":"bloop"}
   ]
  },
  {"zip": 1}
 ]
}
`)
	tests.Assert(t, err != nil)

	// make sure multi pvs in output fails
	s, err = parsePvsResult(`
{
 "report": [
  {
   "pv": [
    {"pv_name":"/dev/foo", "pv_uuid":"abcdef1", "vg_name":"bloop"},
    {"pv_name":"/dev/bar", "pv_uuid":"1234555", "vg_name":"gloop"}
   ]
  }
 ]
}
`)
	tests.Assert(t, err != nil)
}

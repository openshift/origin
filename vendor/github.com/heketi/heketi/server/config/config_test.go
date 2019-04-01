//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package config

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/heketi/tests"
)

func configString(s string) io.Reader {
	return bytes.NewBuffer([]byte(s))
}

func TestParseConfigDummy(t *testing.T) {
	data := configString(`{
		"pow": "whomp"
	}`)
	_, err := ParseConfig(data)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestParseConfigSimple(t *testing.T) {
	data := configString(`{
		"port": "7890",
		"glusterfs": {
			"executor": "fishy",
			"db": "/tmp/wonderful.db"
		}
	}`)
	c, err := ParseConfig(data)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, c.Port == "7890", `expected c.Port == "7890", got:`, c.Port)
	tests.Assert(t, c.GlusterFS.Executor == "fishy",
		`expected c.GlusterFS.Executor == "fishy", got:`, c.GlusterFS.Executor)
	tests.Assert(t, c.GlusterFS.DBfile == "/tmp/wonderful.db",
		`expected c.GlusterFS.DBfile == "/tmp/wonderful.db", got:`,
		c.GlusterFS.DBfile)
}

func TestParseConfigError(t *testing.T) {
	data := configString(`{
		"port": "7890",
		"glusterfs`)
	_, err := ParseConfig(data)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

func TestReadConfigSimple(t *testing.T) {
	phonyConfig := tests.Tempfile()
	defer os.Remove(phonyConfig)

	f, err := os.OpenFile(phonyConfig, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0700)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer f.Close()
	_, err = io.Copy(f, configString(`{
		"port": "7890",
		"glusterfs": {
			"executor": "fishy",
			"db": "/tmp/wonderful.db"
		}
	}`))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	c, err := ReadConfig(phonyConfig)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, c.Port == "7890", `expected c.Port == "7890", got:`, c.Port)
	tests.Assert(t, c.GlusterFS.Executor == "fishy",
		`expected c.GlusterFS.Executor == "fishy", got:`, c.GlusterFS.Executor)
	tests.Assert(t, c.GlusterFS.DBfile == "/tmp/wonderful.db",
		`expected c.GlusterFS.DBfile == "/tmp/wonderful.db", got:`,
		c.GlusterFS.DBfile)
}

func TestReadConfigError(t *testing.T) {
	_, err := ReadConfig("/this/path.should/never_exist/asdf")
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

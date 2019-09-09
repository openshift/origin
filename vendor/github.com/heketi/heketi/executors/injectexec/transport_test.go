//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package injectexec

import (
	"fmt"
	"testing"

	"github.com/heketi/tests"

	rex "github.com/heketi/heketi/pkg/remoteexec"
)

type DummyTransport struct {
	snapShotLimit      int
	rebalance          bool
	cliTimeout         uint32
	dataAlignment      string
	physicalExtentSize string
	chunkSize          string
	xfsSw              int
	xfsSu              int
}

func (d *DummyTransport) ExecCommands(
	host string, commands []string, timeoutMinutes int) (rex.Results, error) {

	out := make(rex.Results, len(commands))
	for i, c := range commands {
		out[i].Completed = true
		out[i].Output = fmt.Sprintf("RESULT(%v)", c)
	}
	return out, nil
}

func (d *DummyTransport) RebalanceOnExpansion() bool {
	return d.rebalance
}

func (d *DummyTransport) SnapShotLimit() int {
	return d.snapShotLimit
}

func (d *DummyTransport) GlusterCliTimeout() uint32 {
	return d.cliTimeout
}

func (d *DummyTransport) PVDataAlignment() string {
	return d.dataAlignment
}

func (d *DummyTransport) VGPhysicalExtentSize() string {
	return d.physicalExtentSize
}

func (d *DummyTransport) LVChunkSize() string {
	return d.chunkSize
}

func (d *DummyTransport) XfsSw() int {
	return d.xfsSw
}

func (d *DummyTransport) XfsSu() int {
	return d.xfsSu
}

func TestWrapCommandTransport(t *testing.T) {
	w1 := &WrapCommandTransport{Transport: &DummyTransport{}}
	w2 := &WrapCommandTransport{Transport: w1}

	r, err := w2.ExecCommands("foo", []string{"ls 1", "ls 2"}, 10)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 2, "expected len(r) == 2, got:", len(r))
	tests.Assert(t, r[0].Output == "RESULT(ls 1)")
	tests.Assert(t, r[1].Output == "RESULT(ls 2)")

	w1.handleBefore = func(c string) rex.Result {
		return rex.Result{
			Completed: true,
			Output:    "foo(" + c + ")",
		}
	}

	r, err = w2.ExecCommands("foo", []string{"ls 1", "ls 2"}, 10)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 2, "expected len(r) == 2, got:", len(r))
	tests.Assert(t, r[0].Output == "foo(ls 1)")
	tests.Assert(t, r[1].Output == "foo(ls 2)")

	w2.handleAfter = func(c string, r rex.Result) rex.Result {
		return rex.Result{
			Completed: true,
			Output:    "XXX" + r.Output + "XXX",
		}
	}

	r, err = w2.ExecCommands("foo", []string{"ls 1", "ls 2"}, 10)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 2, "expected len(r) == 2, got:", len(r))
	tests.Assert(t, r[0].Output == "XXXfoo(ls 1)XXX", r[0])
	tests.Assert(t, r[1].Output == "XXXfoo(ls 2)XXX", r[1])

	l := w2.SnapShotLimit()
	tests.Assert(t, l == 0, "expected l == 0, got:", l)

	b := w2.RebalanceOnExpansion()
	tests.Assert(t, b == false, "expected b == false, got:", b)
}

func TestWrapCommandTransportError(t *testing.T) {
	w1 := &WrapCommandTransport{Transport: &DummyTransport{}}
	w2 := &WrapCommandTransport{Transport: w1}

	r, err := w2.ExecCommands("foo", []string{"ls 1", "ls 2"}, 10)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(r) == 2, "expected len(r) == 2, got:", len(r))
	tests.Assert(t, r[0].Output == "RESULT(ls 1)")
	tests.Assert(t, r[1].Output == "RESULT(ls 2)")

	w1.handleBefore = func(c string) rex.Result {
		return rex.Result{
			Completed: true,
			Err:       fmt.Errorf("HipHooray"),
		}
	}
	r, err = w2.ExecCommands("foo", []string{"ls 1", "ls 2"}, 10)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, !r.Ok(), "expected r.Ok() is false")
	tests.Assert(t, r[0].Err.Error() == "HipHooray",
		"expected err == HipHooray, got:", r[0].Err.Error())
}

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

	"github.com/heketi/heketi/executors/cmdexec"
	"github.com/heketi/heketi/executors/mockexec"
)

func TestBasicWrapping(t *testing.T) {
	me, err := mockexec.NewMockExecutor()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	ie := NewInjectExecutor(me, &InjectConfig{})
	tests.Assert(t, ie != nil, "expected ie != nil, got:", ie)

	err = ie.PeerProbe("foo", "target")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	me.MockPeerProbe = func(host, newnode string) error {
		tests.Assert(t, host == "foo", "expected host == foo, got:", host)
		return fmt.Errorf("FAKE")
	}

	err = ie.PeerProbe("foo", "target")
	tests.Assert(t, err.Error() == "FAKE", "expected err == FAKE, got:", err)

	// now test override behavior
	ie.Pre.MockPeerProbe = func(host, newnode string) error {
		return fmt.Errorf("FOOBAR")
	}
	err = ie.PeerProbe("foo", "target")
	tests.Assert(t, err.Error() == "FOOBAR", "expected err == FOOBAR, got:", err)
}

func newDummyExecutor() *cmdexec.CmdExecutor {
	d := &cmdexec.CmdExecutor{}
	d.RemoteExecutor = &DummyTransport{}
	d.Init(&cmdexec.CmdConfig{})
	return d
}

func TestCmdexecWrapping(t *testing.T) {
	ic := &InjectConfig{}
	de := newDummyExecutor()
	ie := NewInjectExecutor(de, ic)
	tests.Assert(t, ie != nil, "expected ie != nil, got:", ie)

	err := ie.PeerProbe("foo", "target")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestCmdexecWrapCommand(t *testing.T) {
	ic := &InjectConfig{}
	ic.CmdInjection.CmdHooks = CmdHooks{
		CmdHook{
			Cmd:      ".*",
			Reaction: Reaction{Err: "BLAMMO"},
		},
	}
	de := newDummyExecutor()
	ie := NewInjectExecutor(de, ic)
	tests.Assert(t, ie != nil, "expected ie != nil, got:", ie)

	err := ie.PeerProbe("foo", "target")
	tests.Assert(t, err != nil, "expected err == nil, got:", err)
	tests.Assert(t, err.Error() == "BLAMMO", "expected err == BLAMMO, got:", err)
}

func TestCmdexecWrapResult(t *testing.T) {
	ic := &InjectConfig{}
	ic.CmdInjection.ResultHooks = ResultHooks{
		ResultHook{
			Result: ".*",
			CmdHook: CmdHook{
				Cmd:      ".*",
				Reaction: Reaction{Err: "BLAMMO"},
			},
		},
	}
	de := newDummyExecutor()
	ie := NewInjectExecutor(de, ic)
	tests.Assert(t, ie != nil, "expected ie != nil, got:", ie)

	err := ie.PeerProbe("foo", "target")
	tests.Assert(t, err != nil, "expected err == nil, got:", err)
	tests.Assert(t, err.Error() == "BLAMMO", "expected err == BLAMMO, got:", err)
}

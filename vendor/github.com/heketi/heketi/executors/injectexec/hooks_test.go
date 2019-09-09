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

func TestCmdHookString(t *testing.T) {
	h := CmdHook{Cmd: "gluster volume status"}
	s := h.String()
	tests.Assert(t, s == "CmdHook(gluster volume status)")
}

func TestResultHookString(t *testing.T) {
	h := ResultHook{
		CmdHook: CmdHook{Cmd: "gluster volume status"},
		Result:  ".*",
	}
	s := h.String()
	tests.Assert(t, s == "ResultHook(gluster volume status)")
}

func TestSimpleReaction(t *testing.T) {
	r := Reaction{Result: "Hello"}
	res := r.React()
	tests.Assert(t, res.Ok(), "expected res.Ok() to be true")
	tests.Assert(t, res.Output == "Hello",
		"expected res.Output == Hello, got:", res.Output)

	r = Reaction{Err: "Yikes"}
	res = r.React()
	tests.Assert(t, !res.Ok(), "expected res.Ok() to be false")
	tests.Assert(t, res.Error() == "Yikes",
		"expected res.Error() == Yikes, got:", res.Error())
	tests.Assert(t, res.Output == "",
		"expected res.Output == \"\", got:", res.Output)
}

func TestPanicReaction(t *testing.T) {
	r := Reaction{Panic: "Kaboom"}
	defer func() {
		v := recover()
		tests.Assert(t, v == "Kaboom", "expected panic with Kaboom, got", v)
	}()

	r.React()
	t.Fatalf("should not get here")
}

func TestHookCommandsNoMatch(t *testing.T) {
	result := HookCommands([]CmdHook{}, "ls -l")
	tests.Assert(t, !result.Completed, "expected not hooked")
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "", "expected result == \"\", got:",
		result)
}

func TestHookCommandsSimpleMatch(t *testing.T) {
	result := HookCommands(
		[]CmdHook{
			{Cmd: "vgs", Reaction: Reaction{Result: "Yo"}},
		},
		"vgs")
	tests.Assert(t, result.Completed, "expected hooked")
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "Yo", "expected result == Yo, got:",
		result.Output)
}

func TestHookCommandsErrorMatch(t *testing.T) {
	result := HookCommands(
		[]CmdHook{
			{Cmd: "vgs", Reaction: Reaction{Err: "Zap"}},
		},
		"vgs")
	tests.Assert(t, result.Completed, "expected hooked")
	tests.Assert(t, result.Err.Error() == "Zap", "expected err == Zap, got:", result.Err)
	tests.Assert(t, result.Output == "", "expected result == \"\", got:",
		result.Output)
}

func TestHookResultsNoMatch(t *testing.T) {
	result := HookResults(
		[]ResultHook{},
		"ls",
		rex.Result{
			Completed: true,
			Output:    "bin etc lib",
		})
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "bin etc lib",
		"expected result == \"bin etc lib\", got:",
		result.Output)
}

func TestHookResultsNoMatch2(t *testing.T) {
	h := ResultHook{}
	h.Cmd = "vgs"
	h.Result = ".*"
	result := HookResults(
		[]ResultHook{h},
		"ls",
		rex.Result{
			Completed: true,
			Output:    "bin etc lib",
		})
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "bin etc lib",
		"expected result == \"bin etc lib\", got:",
		result.Output)
}

func TestHookResultsNoMatchResult(t *testing.T) {
	h := ResultHook{}
	h.Cmd = "vgs"
	h.Result = "flippy"
	result := HookResults(
		[]ResultHook{h},
		"ls",
		rex.Result{
			Completed: true,
			Output:    "bin etc lib",
		})
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "bin etc lib",
		"expected result == \"bin etc lib\", got:",
		result.Output)
}

func TestHookResultsMatch(t *testing.T) {
	h := ResultHook{}
	h.Cmd = "ls"
	h.Result = ".*"
	h.Reaction.Result = "foo bar baz"
	result := HookResults(
		[]ResultHook{h},
		"ls",
		rex.Result{
			Completed: true,
			Output:    "bin etc lib",
		})
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "foo bar baz",
		"expected result == \"foo bar baz\", got:",
		result.Output)
}

func TestHookResultsMatchErrOut(t *testing.T) {
	h := ResultHook{}
	h.Cmd = "ls"
	h.Result = ".*"
	h.Reaction.Err = "LALALA"
	result := HookResults(
		[]ResultHook{h},
		"ls",
		rex.Result{
			Completed: true,
			Output:    "bin etc lib",
		})
	tests.Assert(t, result.Err.Error() == "LALALA", "expected err == LALALA, got:", result.Err)
	tests.Assert(t, result.Output == "",
		"expected result == \"\", got:",
		result.Output)
}

func TestHookResultsMatchErrIn(t *testing.T) {
	h := ResultHook{}
	h.Cmd = "ls"
	h.Result = "point"
	h.Reaction.Result = "funky"
	result := HookResults(
		[]ResultHook{h},
		"ls",
		rex.Result{
			Completed: true,
			Err:       fmt.Errorf("point"),
		})
	tests.Assert(t, result.Err == nil, "expected err == nil, got:", result.Err)
	tests.Assert(t, result.Output == "funky",
		"expected result == \"funky\", got:",
		result.Output)
}

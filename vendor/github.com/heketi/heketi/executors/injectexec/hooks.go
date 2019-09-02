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
	"os"
	"regexp"
	"time"

	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// Reaction represents the type of result or error to produce when
// a hook is encountered. This can be used to give back a fake
// result, an error, panic the server or pause for a time.
type Reaction struct {
	// Result is used to return a new "fake" result string
	Result string
	// Err is used to create a new forced error
	Err string
	// Pause will sleep for the given time in seconds. Pause can
	// be combined with the other other options.
	Pause uint64
	// Panic will trigger a panic.
	Panic string

	// The following opts help specify more customized results

	// ExitStatus allows tests to set a custom exit code
	ExitStatus int
	// ErrOutput allows tests to set custom "stderr" content
	ErrOutput string
	// ForceErr forces an empty error string
	ForceErr bool
	// ForceErrOutput prevents Err from overriding empty ErrOutput
	ForceErrOutput bool
}

// React triggers the kind of error or result the Reaction
// is configured for.
func (r Reaction) React() rex.Result {
	if r.Pause != 0 {
		time.Sleep(time.Second * time.Duration(r.Pause))
	}
	if r.Panic != "" {
		panic(r.Panic)
	}
	result := rex.Result{
		Completed:  true,
		Output:     r.Result,
		ExitStatus: r.ExitStatus,
		ErrOutput:  r.ErrOutput,
	}
	if r.Err != "" || r.ForceErr {
		result.Err = fmt.Errorf(r.Err)
	}
	if result.Err != nil && result.ExitStatus == 0 {
		result.ExitStatus = 1
	}
	if result.Err != nil && result.ErrOutput == "" && !r.ForceErrOutput {
		result.ErrOutput = r.Err
	}
	return result
}

// CmdHook is a hook for a given command. Provide a regex as a string
// to match a command flowing through one of the executors and if
// the hook matches, the hook's reaction will be called instead of
// the real command.
type CmdHook struct {
	Cmd      string
	CondFile string
	Reaction Reaction
}

// Match returns true if the provided command matches the regex of
// the hook.
func (c *CmdHook) Match(command string) bool {
	m, e := regexp.MatchString(c.Cmd, command)
	return (e == nil && m)
}

// CheckConditions returns true if all of the conditions on
// the hook are true (currenly only the CondFile condition).
func (c *CmdHook) CheckConditions() bool {
	if c.CondFile == "" {
		// no condition file set. return true
		return true
	}
	logger.Info("condition file configured: checking for %v", c.CondFile)
	_, err := os.Stat(c.CondFile)
	present := err == nil
	logger.Info("condition file: %v present=%v", c.CondFile, present)
	return present
}

// String returns a string representation of the hook.
func (c *CmdHook) String() string {
	return fmt.Sprintf("CmdHook(%v)", c.Cmd)
}

// ResultHook is a hook for a given command and result. This hook
// is checked after a command has run and only fires if both the
// command and the result match.
type ResultHook struct {
	Result string
	CmdHook
}

// Match returns true if the provided command and the command's result
// string match the hook's regexes for the Result and Cmd fields.
func (r *ResultHook) Match(command, result string) bool {
	m1, e1 := regexp.MatchString(r.Cmd, command)
	if e1 != nil {
		logger.Warning("regexp error: %v", e1)
	}
	m2, e2 := regexp.MatchString(r.Result, result)
	if e2 != nil {
		logger.Warning("regexp error: %v", e2)
	}
	return (e1 == nil && m1) && (e2 == nil && m2)
}

// String returns a string representation of the hook.
func (r *ResultHook) String() string {
	return fmt.Sprintf("ResultHook(%v)", r.Cmd)
}

type CmdHooks []CmdHook

type ResultHooks []ResultHook

// HookCommands checks a list of command hooks against a given command
// string. For the first matching hook the hook's reaction is returned.
func HookCommands(hooks CmdHooks, c string) rex.Result {

	logger.Info("Checking for hook on %v", c)
	for _, h := range hooks {
		if h.Match(c) && h.CheckConditions() {
			logger.Debug("found hook for %v: %v", c, h)
			return logHookResult(c, h.Reaction.React())
		}
	}
	return rex.Result{}
}

// HookResults checks a list of result hooks against a given command
// and its result or error (as a string). For the first matching hook
// the hook's reaction is returned.
func HookResults(hooks ResultHooks, c string, result rex.Result) rex.Result {

	compare := result.Output
	if !result.Ok() {
		compare = result.Err.Error()
	}

	for _, h := range hooks {
		logger.Info("Checking for hook on %v -> %v", c, compare)
		if h.Match(c, compare) && h.CheckConditions() {
			logger.Debug("found hook for %v/%v: %v", c, compare, h)
			return logHookResult(c, h.Reaction.React())
		}
	}
	return result
}

func logHookResult(c string, r rex.Result) rex.Result {
	if r.Ok() {
		logger.Debug(
			"Hook command [%v] on [internal]: Stdout [%v]: Stderr [%v]",
			c, r.Output, r.ErrOutput)
	} else {
		logger.LogError(
			"Hook command [%v] on [internal]: Err[%v]: Stdout [%v]: Stderr [%v]",
			c, r.Err, r.Output, r.ErrOutput)
	}
	return r
}

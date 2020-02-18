//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package log

import (
	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// Logger interface exists to allow the library to use a
// logging object provided by the caller.
type Logger interface {
	LogError(s string, v ...interface{}) error
	Err(e error) error
	Critical(s string, v ...interface{})
	Debug(s string, v ...interface{})
}

type CommandLogger struct {
	logger Logger
}

func NewCommandLogger(l Logger) *CommandLogger {
	return &CommandLogger{logger: l}
}

func (cl *CommandLogger) Before(c rex.Cmd, where string) {
	cl.logger.Debug(
		"Will run command [%v] on [%v]",
		c.String(), where)
}

func (cl *CommandLogger) Success(c rex.Cmd, where, out, errout string) {
	if c.Opts().Quiet {
		cl.logger.Debug(
			"Ran command [%v] on [%v]: Stdout filtered, Stderr filtered",
			c.String(), where)
		return
	}
	cl.logger.Debug(
		"Ran command [%v] on [%v]: Stdout [%v]: Stderr [%v]",
		c.String(), where, out, errout)
}

func (cl *CommandLogger) Error(c rex.Cmd, err error, where, out, errout string) {
	if c.Opts().ErrorOk {
		cl.Success(c, where, out, errout)
		return
	}
	cl.logger.LogError(
		"Failed to run command [%v] on [%v]: Err[%v]: Stdout [%v]: Stderr [%v]",
		c.String(), where, err, out, errout)
}

func (cl *CommandLogger) Timeout(c rex.Cmd, err error, where, out, errout string) {
	cl.logger.LogError(
		"Timeout on command [%v] on [%v]: Err[%v]: Stdout [%v]: Stderr [%v]",
		c.String(), where, err, out, errout)
}

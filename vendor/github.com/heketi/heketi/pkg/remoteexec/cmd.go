//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package remoteexec

type CmdOpts struct {
	Quiet   bool // suppress output logging on success
	ErrorOk bool // treat error conditions same as successes
}

type Cmd interface {
	String() string
	Opts() CmdOpts
}

type StringCmd struct {
	Command string
	Options CmdOpts
}

func (sc StringCmd) String() string {
	return sc.Command
}

func (sc StringCmd) Opts() CmdOpts {
	return sc.Options
}

type Cmds []Cmd

// conversion functions

// ToCmd converts a single string representing a command to a StringCmd.
func ToCmd(s string) StringCmd {
	return StringCmd{Command: s}
}

// ToCmds converts a string slice to a Cmds group.
func ToCmds(s []string) Cmds {
	c := make(Cmds, len(s))
	for i, val := range s {
		c[i] = StringCmd{Command: val}
	}
	return c
}

// OneCmd converts a single string representing a command to a Cmds group
// containing just one command.
func OneCmd(s string) Cmds {
	return Cmds{StringCmd{Command: s}}
}

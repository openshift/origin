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
	"github.com/heketi/heketi/executors/cmdexec"
	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// WrapCommandTransport can be used to replace a real transport
// and intercept the commands passing through it. The real transport
// must be set and the optional handleBefore and handleAfter functions
// can be used to artificially manipulate the command results.
type WrapCommandTransport struct {
	handleBefore func(string) rex.Result
	handleAfter  func(string, rex.Result) rex.Result
	Transport    cmdexec.RemoteCommandTransport
}

func (w *WrapCommandTransport) ExecCommands(
	host string, commands []string, timeoutMinutes int) (rex.Results, error) {

	results := make(rex.Results, len(commands))
	for i, c := range commands {
		r := w.Before(c)
		if r.Completed {
			results[i] = r
			continue
		}
		tres, err := w.Transport.ExecCommands(
			host, []string{c}, timeoutMinutes)
		if err != nil {
			return results, err
		}
		results[i] = w.After(c, tres)
	}
	return results, nil
}

func (w *WrapCommandTransport) RebalanceOnExpansion() bool {
	return w.Transport.RebalanceOnExpansion()
}

func (w *WrapCommandTransport) SnapShotLimit() int {
	return w.Transport.SnapShotLimit()
}

func (w *WrapCommandTransport) GlusterCliTimeout() uint32 {
	return w.Transport.GlusterCliTimeout()
}

func (w *WrapCommandTransport) PVDataAlignment() string {
	return w.Transport.PVDataAlignment()
}

func (w *WrapCommandTransport) VGPhysicalExtentSize() string {
	return w.Transport.VGPhysicalExtentSize()
}

func (w *WrapCommandTransport) LVChunkSize() string {
	return w.Transport.LVChunkSize()
}

func (w *WrapCommandTransport) XfsSw() int {
	return w.Transport.XfsSw()
}

func (w *WrapCommandTransport) XfsSu() int {
	return w.Transport.XfsSu()
}

// Before calls the WrapCommandTransport's handleBefore function
// if one is set. If the command was handled the and no additional
// processing of the command is needed the first return value will
// be true. The remaining return values are the command's results
// or error.
func (w *WrapCommandTransport) Before(command string) rex.Result {

	if w.handleBefore != nil {
		return w.handleBefore(command)
	}
	return rex.Result{}
}

// After calls the WrapCommandTransport's handleAfter function
// if one is set. The handleAfter function may or may not alter
// the results of the input results or error condition.
func (w *WrapCommandTransport) After(
	command string, results rex.Results) rex.Result {

	r := results[0]
	if w.handleAfter != nil {
		return w.handleAfter(command, r)
	}
	return r
}

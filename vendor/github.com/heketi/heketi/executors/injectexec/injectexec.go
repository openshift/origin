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
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/executors/cmdexec"
	"github.com/heketi/heketi/executors/kubeexec"
	"github.com/heketi/heketi/executors/mockexec"
	"github.com/heketi/heketi/executors/sshexec"
	"github.com/heketi/heketi/executors/stack"
	"github.com/heketi/heketi/pkg/logging"
	rex "github.com/heketi/heketi/pkg/remoteexec"
)

var (
	logger = logging.NewLogger("[injectexec]", logging.LEVEL_DEBUG)
)

type InjectExecutor struct {
	stack.ExecutorStack

	realExecutor  executors.Executor
	realTransport cmdexec.RemoteCommandTransport

	// hooks
	Pre    *mockexec.MockExecutor
	config *InjectConfig
}

func NewInjectExecutor(
	e executors.Executor, config *InjectConfig) *InjectExecutor {

	ie := &InjectExecutor{}
	ie.config = config
	ie.Pre = newMockBase()
	ie.realExecutor = e

	// set up the exec stack to include our dummy "pre"-executor
	// and the real executor
	ie.SetExec([]executors.Executor{ie.Pre, ie.realExecutor})

	// set up wrapping of real executors that are command based
	// to load the hooks for individual commands
	switch x := e.(type) {
	case *sshexec.SshExecutor:
		logger.Info("injecting executor with transport")
		ie.realTransport = x.RemoteExecutor
		x.RemoteExecutor = ie.Wrap(x.RemoteExecutor)
	case *kubeexec.KubeExecutor:
		logger.Info("injecting executor with transport")
		ie.realTransport = x.RemoteExecutor
		x.RemoteExecutor = ie.Wrap(x.RemoteExecutor)
	case *cmdexec.CmdExecutor:
		logger.Info("injecting executor with transport")
		ie.realTransport = x.RemoteExecutor
		x.RemoteExecutor = ie.Wrap(x.RemoteExecutor)
	default:
		logger.Warning("not wrapping executor without known transport")
	}
	logger.Info("created inject executor (from %T)", ie.realExecutor)
	return ie
}

// Wrap takes a command transport and returns a wrapped transport that
// runs the commands passing through the transport through the
// hooks.
func (ie *InjectExecutor) Wrap(t cmdexec.RemoteCommandTransport) cmdexec.RemoteCommandTransport {
	return &WrapCommandTransport{
		Transport: t,
		handleBefore: func(c string) rex.Result {
			logger.Info("injectexec wrapped command: %v", c)
			return HookCommands(ie.config.CmdInjection.CmdHooks, c)
		},
		handleAfter: func(c string, r rex.Result) rex.Result {
			logger.Info("injectexec wrapped command (result): %v", c)
			return HookResults(ie.config.CmdInjection.ResultHooks, c, r)
		},
	}
}

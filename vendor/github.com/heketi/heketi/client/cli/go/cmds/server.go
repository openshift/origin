//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package cmds

import (
	"fmt"
	"os"
	"text/template"

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/spf13/cobra"
)

var serverCommand = &cobra.Command{
	Use:   "server",
	Short: "Heketi Server Management",
	Long:  "Heketi Server Information & Management",
}

var operationsCommand = &cobra.Command{
	Use:   "operations",
	Short: "Manage ongoing server operations",
	Long:  "Manage ongoing server operations",
}

var opInfoTemplate = `Operation Counts:
  Total: {{.Total}}
  In-Flight: {{.InFlight}}
  New: {{.New}}
  Stale: {{.Stale}}
`

var operationsInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "Get a summary of server operations",
	Long:    "Get a summary of server operations",
	Example: `  $ heketi-cli server operations info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		t, err := template.New("opInfo").Parse(opInfoTemplate)
		if err != nil {
			return err
		}
		opInfo, err := heketi.OperationsInfo()
		if err == nil {
			t.Execute(os.Stdout, opInfo)
		}
		return err
	},
}

var modeCommand = &cobra.Command{
	Use:   "mode",
	Short: "Manage server mode",
	Long:  "Manage server mode",
}

var getModeCommand = &cobra.Command{
	Use:     "get",
	Short:   "Print current server mode",
	Long:    "Print current server mode",
	Example: `  $ heketi-cli server operations info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		adminStatus, err := heketi.AdminStatusGet()
		if err != nil {
			return err
		}
		fmt.Printf("%v\n", adminStatus.State)
		return nil
	},
}

var setModeCommand = &cobra.Command{
	Use:     "set [normal|local-client|read-only]",
	Short:   "Print current server mode",
	Long:    "Print current server mode",
	Example: `  $ heketi-cli server operations info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing mode argument")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		value := api.AdminState(args[0])
		err = heketi.AdminStatusSet(&api.AdminStatus{State: value})
		return err
	},
}

func init() {
	RootCmd.AddCommand(serverCommand)
	// operations command(s)
	serverCommand.AddCommand(operationsCommand)
	operationsCommand.SilenceUsage = true
	operationsCommand.AddCommand(operationsInfoCommand)
	operationsInfoCommand.SilenceUsage = true
	// admin mode command(s)
	serverCommand.AddCommand(modeCommand)
	modeCommand.SilenceUsage = true
	modeCommand.AddCommand(getModeCommand)
	getModeCommand.SilenceUsage = true
	modeCommand.AddCommand(setModeCommand)
	setModeCommand.SilenceUsage = true
}

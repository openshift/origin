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
  Failed: {{.Failed}}
  Stale: {{.Stale}}
`

var popListTemplate = `
{{- range .PendingOperations -}}
Id:{{.Id}}  Type:{{.TypeName}}  Status:
{{- if eq .Status ""}}New{{ else }}{{.Status}}{{end}} {{.SubStatus}}
{{ end -}}
`

var popDetailsTemplate = `
Id: {{.Id}}
Type: {{.TypeName}}
Status: {{if eq .Status ""}}New{{ else }}{{.Status}}{{end}} {{.SubStatus}}
Changes:
{{- range .Changes }}
    {{.Description}}: {{.Id}}
{{- end }}
`

var operationsInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "Get a summary of server operations",
	Long:    "Get a summary of server operations",
	Example: `  $ heketi-cli server operations info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// I probably should not have used "info" to get a simple
		// summary of operations when it was used elsewhere to
		// display details about a particular id. This keeps
		// backwards compatibility while keeping the pattern of
		// "info <id>" for more info about a particular item.
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return operationsInfoSummary()
		} else {
			return operationDetails(cmd.Flags().Arg(0))
		}
	},
}

func operationsInfoSummary() error {
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
}

func operationDetails(id string) error {
	heketi, err := newHeketiClient()
	if err != nil {
		return err
	}
	t, err := template.New("popDetails").Parse(popDetailsTemplate)
	if err != nil {
		return err
	}
	opInfo, err := heketi.PendingOperationDetails(id)
	if err == nil {
		err := t.Execute(os.Stdout, opInfo)
		if err != nil {
			return err
		}
	}
	return err
}

var operationsListCommand = &cobra.Command{
	Use:     "list",
	Short:   "Get a list of pending operations",
	Long:    "Get a list of pending operations",
	Example: `  $ heketi-cli server operations list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		t, err := template.New("popList").Parse(popListTemplate)
		if err != nil {
			return err
		}
		popList, err := heketi.PendingOperationList()
		if err == nil {
			err := t.Execute(os.Stdout, popList)
			if err != nil {
				return err
			}
		}
		return err
	},
}

var operationsCleanUpCommand = &cobra.Command{
	Use:     "cleanup",
	Short:   "Clean up stale or failed pending operations",
	Long:    "Clean up stale or failed pending operations",
	Example: `  $ heketi-cli server operations cleanup`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		request := api.PendingOperationsCleanRequest{}
		if len(args) > 0 {
			request.Operations = args
		}
		err = heketi.PendingOperationCleanUp(&request)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr,
			"Note: Operation clean up is a batch operation.\n"+
				"* The results of individual operation clean ups"+
				" are not reported here.\n"+
				"* Use 'heketi-cli server operations [list|info]'"+
				" to view operations.")
		return nil
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
	Example: `  $ heketi-cli server mode get`,
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
	Example: `  $ heketi-cli server mode set read-only`,
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

var stateCommand = &cobra.Command{
	Use:   "state",
	Short: "View and/or modify state of server",
	Long:  "View and/or modify state of server",
}

var stateExamineCommand = &cobra.Command{
	Use:   "examine",
	Short: "Compare state of server",
	Long:  "Compare state of server",
}

var stateExamineGlusterCommand = &cobra.Command{
	Use:     "gluster",
	Short:   "Compare state of server with gluster",
	Long:    "Compare state of server with gluster",
	Example: `  $ heketi-cli server state examine gluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		result, err := heketi.StateExamineGluster()
		if err != nil {
			return err
		}

		fmt.Fprintf(stdout, "%v", result)

		return nil

	},
}

func init() {
	RootCmd.AddCommand(serverCommand)
	// operations command(s)
	serverCommand.AddCommand(operationsCommand)
	operationsCommand.SilenceUsage = true
	operationsCommand.AddCommand(operationsInfoCommand)
	operationsInfoCommand.SilenceUsage = true
	operationsCommand.AddCommand(operationsListCommand)
	operationsListCommand.SilenceUsage = true
	operationsCommand.AddCommand(operationsCleanUpCommand)
	operationsCleanUpCommand.SilenceUsage = true
	// admin mode command(s)
	serverCommand.AddCommand(modeCommand)
	modeCommand.SilenceUsage = true
	modeCommand.AddCommand(getModeCommand)
	getModeCommand.SilenceUsage = true
	modeCommand.AddCommand(setModeCommand)
	setModeCommand.SilenceUsage = true
	// state command(s)
	serverCommand.AddCommand(stateCommand)
	stateCommand.SilenceUsage = true
	stateCommand.AddCommand(stateExamineCommand)
	stateExamineCommand.SilenceUsage = true
	stateExamineCommand.AddCommand(stateExamineGlusterCommand)
	stateExamineGlusterCommand.SilenceUsage = true
}

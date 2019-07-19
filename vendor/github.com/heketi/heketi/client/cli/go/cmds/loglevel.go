//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmds

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func init() {
	RootCmd.AddCommand(logLevelCommand)
	logLevelCommand.AddCommand(logLevelGetCommand)
	logLevelGetCommand.SilenceUsage = true
	logLevelCommand.AddCommand(logLevelSetCommand)
	logLevelSetCommand.SilenceUsage = true
}

var logLevelCommand = &cobra.Command{
	Use:   "loglevel",
	Short: "Heketi Log Level",
	Long:  "Manage Heketi server Log Level",
}

var logLevelGetCommand = &cobra.Command{
	Use:     "get",
	Short:   "Get Heketi server Log Level",
	Long:    "Get Heketi server Log Level",
	Example: `  $ heketi-cli loglevel get`,
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		llinfo, err := heketi.LogLevelGet()
		if err == nil {
			fmt.Fprintf(stdout, "%s\n", llinfo.LogLevel["glusterfs"])
		}
		return err
	},
}

var logLevelSetCommand = &cobra.Command{
	Use:     "set",
	Short:   "Set Heketi server Log Level",
	Long:    "Set Heketi server Log Level",
	Example: `  $ heketi-cli loglevel set debug`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing log-level argument")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		err = heketi.LogLevelSet(&api.LogLevelInfo{
			LogLevel: map[string]string{"glusterfs": args[0]},
		})
		if err == nil {
			fmt.Fprintf(stdout, "Server log level updated\n")
		}
		return err
	},
}

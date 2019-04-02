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
)

func init() {
	RootCmd.AddCommand(dbCommand)
	dbCommand.AddCommand(dumpDbCommand)
	dumpDbCommand.SilenceUsage = true
}

var dbCommand = &cobra.Command{
	Use:   "db",
	Short: "Heketi Database Management",
	Long:  "Heketi Database Management",
}

var dumpDbCommand = &cobra.Command{
	Use:     "dump",
	Short:   "dumps the database in json format",
	Long:    "dumps the database in json format",
	Example: "  $ heketi-cli db dump",
	RunE: func(cmd *cobra.Command, args []string) error {
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		dump, err := heketi.DbDump()
		if err != nil {
			return err
		}

		fmt.Fprintf(stdout, dump)

		return nil
	},
}

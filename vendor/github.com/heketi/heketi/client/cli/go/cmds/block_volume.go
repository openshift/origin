//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmds

import (
	"encoding/json"
	"errors"
	"fmt"
	//	"os"
	"strings"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/spf13/cobra"
)

var (
	bv_size     int
	bv_volname  string
	bv_auth     bool
	bv_clusters string
	bv_ha       int
)

func init() {
	RootCmd.AddCommand(blockVolumeCommand)
	blockVolumeCommand.AddCommand(blockVolumeCreateCommand)
	blockVolumeCommand.AddCommand(blockVolumeDeleteCommand)
	blockVolumeCommand.AddCommand(blockVolumeInfoCommand)
	blockVolumeCommand.AddCommand(blockVolumeListCommand)

	blockVolumeCreateCommand.Flags().IntVar(&bv_size, "size", 0,
		"\n\tSize of volume in GiB")
	blockVolumeCreateCommand.Flags().IntVar(&bv_ha, "ha", 0,
		"\n\tHA count for block volume")
	blockVolumeCreateCommand.Flags().BoolVar(&bv_auth, "auth", false,
		"\n\tOptional: Enable Authentication for block volume access")
	blockVolumeCreateCommand.Flags().StringVar(&bv_volname, "name", "",
		"\n\tOptional: Name of volume. Only set if really necessary")
	blockVolumeCreateCommand.Flags().StringVar(&bv_clusters, "clusters", "",
		"\n\tOptional: Comma separated list of cluster ids where this volume"+
			"\n\tmust be allocated. If omitted, Heketi will allocate the volume"+
			"\n\ton any of the configured clusters which have the available space."+
			"\n\tProviding a set of clusters will ensure Heketi allocates storage"+
			"\n\tfor this volume only in the clusters specified.")
	blockVolumeCreateCommand.SilenceUsage = true
	blockVolumeDeleteCommand.SilenceUsage = true
	blockVolumeInfoCommand.SilenceUsage = true
	blockVolumeListCommand.SilenceUsage = true
}

var blockVolumeCommand = &cobra.Command{
	Use:   "blockvolume",
	Short: "Heketi Volume Management",
	Long:  "Heketi Volume Management",
}

var blockVolumeCreateCommand = &cobra.Command{
	Use:   "create",
	Short: "Create a GlusterFS block volume",
	Long:  "Create a GlusterFS block volume",
	Example: `  * Create a 100GiB block volume
      $ heketi-cli blockvolume create --size=100

  * Create a 100GiB block volume specifying two specific clusters:
      $ heketi-cli blockvolume create --size=100 \
        --clusters=0995098e1284ddccb46c7752d142c832,60d46d518074b13a04ce1022c8c7193c

  * Create a 100GiB block volume requesting ha count to be 2.
    (Otherwise HA count is all the nodes on which block hosting volume reside.):
	  $ heketi-cli blockvolume create --size=100 --ha=2

  * Create a 100GiB block volume specifying two specific clusters auth enabled:
      $ heketi-cli blockvolume create --size=100 --auth \
        --clusters=0995098e1284ddccb46c7752d142c832,60d46d518074b13a04ce1022c8c7193c
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if bv_size == 0 {
			return errors.New("Missing volume size")
		}

		req := &api.BlockVolumeCreateRequest{}
		req.Size = bv_size
		req.Auth = bv_auth
		if bv_clusters != "" {
			req.Clusters = strings.Split(bv_clusters, ",")
		}

		if bv_volname != "" {
			req.Name = bv_volname
		}

		if bv_ha >= 0 {
			req.Hacount = bv_ha
		} else {
			return errors.New("Invalid HA count")
		}

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		blockvolume, err := heketi.BlockVolumeCreate(req)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(blockvolume)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", blockvolume)
		}

		return nil
	},
}

var blockVolumeDeleteCommand = &cobra.Command{
	Use:     "delete",
	Short:   "Deletes the volume",
	Long:    "Deletes the volume",
	Example: "  $ heketi-cli volume delete 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Volume id missing")
		}

		//set volumeId
		volumeId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		err = heketi.BlockVolumeDelete(volumeId)
		if err == nil {
			fmt.Fprintf(stdout, "Volume %v deleted\n", volumeId)
		}

		return err
	},
}

var blockVolumeInfoCommand = &cobra.Command{
	Use:     "info",
	Short:   "Retreives information about the volume",
	Long:    "Retreives information about the volume",
	Example: "  $ heketi-cli volume info 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("Volume id missing")
		}

		// Set volume id
		volumeId := cmd.Flags().Arg(0)

		// Create a client to talk to Heketi
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// Create cluster
		info, err := heketi.BlockVolumeInfo(volumeId)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(info)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", info)
		}
		return nil

	},
}

var blockVolumeListCommand = &cobra.Command{
	Use:     "list",
	Short:   "Lists the volumes managed by Heketi",
	Long:    "Lists the volumes managed by Heketi",
	Example: "  $ heketi-cli blockvolume list",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// List volumes
		list, err := heketi.BlockVolumeList()
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(list)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			for _, id := range list.BlockVolumes {
				volume, err := heketi.BlockVolumeInfo(id)
				if err != nil {
					return err
				}

				fmt.Fprintf(stdout, "Id:%-35v Cluster:%-35v Name:%v\n",
					id,
					volume.Cluster,
					volume.Name)
			}
		}

		return nil
	},
}

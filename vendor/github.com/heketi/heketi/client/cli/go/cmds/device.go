//
// Copyright (c) 2015 The heketi Authors
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
	"strings"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/spf13/cobra"
)

var (
	device, nodeId string
)

func init() {
	RootCmd.AddCommand(deviceCommand)
	deviceCommand.AddCommand(deviceAddCommand)
	deviceCommand.AddCommand(deviceDeleteCommand)
	deviceCommand.AddCommand(deviceRemoveCommand)
	deviceCommand.AddCommand(deviceInfoCommand)
	deviceCommand.AddCommand(deviceEnableCommand)
	deviceCommand.AddCommand(deviceDisableCommand)
	deviceCommand.AddCommand(deviceResyncCommand)
	deviceResyncCommand.Flags().Bool("cluster", false,
		"Resync all devices under the cluster identified by object_id")
	deviceCommand.AddCommand(deviceSetTagsCommand)
	deviceCommand.AddCommand(deviceRmTagsCommand)
	deviceAddCommand.Flags().StringVar(&device, "name", "",
		"Name of device to add")
	deviceAddCommand.Flags().StringVar(&nodeId, "node", "",
		"Id of the node which has this device")
	deviceAddCommand.Flags().Bool("destroy-existing-data", false,
		"[DANGEROUS] Destroy any existing data on the device.")
	deviceSetTagsCommand.Flags().BoolP("exact", "e", false,
		"Set the object to this exact set of tags. Overwrites existing tags.")
	deviceRmTagsCommand.Flags().Bool("all", false,
		"Remove all tags.")
	deviceDeleteCommand.Flags().Bool("force-forget", false,
		"[DANGEROUS] Force heketi to forget a device, regardless of state.")
	deviceAddCommand.SilenceUsage = true
	deviceDeleteCommand.SilenceUsage = true
	deviceRemoveCommand.SilenceUsage = true
	deviceInfoCommand.SilenceUsage = true
	deviceResyncCommand.SilenceUsage = true
	deviceSetTagsCommand.SilenceUsage = true
	deviceRmTagsCommand.SilenceUsage = true
}

var deviceCommand = &cobra.Command{
	Use:   "device",
	Short: "Heketi device management",
	Long:  "Heketi Device Management",
}

var deviceAddCommand = &cobra.Command{
	Use:   "add",
	Short: "Add new device to node to be managed by Heketi",
	Long:  "Add new device to node to be managed by Heketi",
	Example: `  $ heketi-cli device add \
      --name=/dev/sdb
      --node=3e098cb4407d7109806bb196d9e8f095 `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check arguments
		if device == "" {
			return errors.New("Missing device name")
		}
		if nodeId == "" {
			return errors.New("Missing node id")
		}
		destroyData, err := cmd.Flags().GetBool("destroy-existing-data")
		if err != nil {
			return err
		}

		// Create request blob
		req := &api.DeviceAddRequest{}
		req.Name = device
		req.NodeId = nodeId
		req.DestroyData = destroyData

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// Add node
		err = heketi.DeviceAdd(req)
		if err != nil {
			return err
		} else {
			fmt.Fprintf(stdout, "Device added successfully\n")
		}

		return nil
	},
}

var deviceDeleteCommand = &cobra.Command{
	Use:     "delete [device_id]",
	Short:   "Deletes a device from Heketi node",
	Long:    "Deletes a device from Heketi node",
	Example: "  $ heketi-cli device delete 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Device id missing")
		}

		//set clusterId
		deviceId := cmd.Flags().Arg(0)

		forceForget, err := cmd.Flags().GetBool("force-forget")
		if err != nil {
			return err
		}

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		var opts api.DeviceDeleteOptions
		opts.ForceForget = forceForget
		err = heketi.DeviceDeleteWithOptions(deviceId, &opts)
		if err == nil {
			fmt.Fprintf(stdout, "Device %v deleted\n", deviceId)
		}

		return err
	},
}

var deviceRemoveCommand = &cobra.Command{
	Use:     "remove [device_id]",
	Short:   "Removes a device from Heketi node",
	Long:    "Removes a device from Heketi node",
	Example: "  $ heketi-cli device remove 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Device id missing")
		}

		//set clusterId
		deviceId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		req := &api.StateRequest{
			State: "failed",
		}
		err = heketi.DeviceState(deviceId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Device %v is now removed\n", deviceId)
		}

		return err
	},
}

var deviceInfoCommand = &cobra.Command{
	Use:     "info [device_id]",
	Short:   "Retrieves information about the device",
	Long:    "Retrieves information about the device",
	Example: "  $ heketi-cli device info 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("Device id missing")
		}

		// Set node id
		deviceId := cmd.Flags().Arg(0)

		// Create a client to talk to Heketi
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// Create cluster
		info, err := heketi.DeviceInfo(deviceId)
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
			fmt.Fprintf(stdout, "Device Id: %v\n"+
				"State: %v\n"+
				"Size (GiB): %v\n"+
				"Used (GiB): %v\n"+
				"Free (GiB): %v\n",
				info.Id,
				entryStateString(info.State),
				info.Storage.Total/(1024*1024),
				info.Storage.Used/(1024*1024),
				info.Storage.Free/(1024*1024))
			fmt.Fprintf(stdout, "Create Path: %v\n", info.Name)
			fmt.Fprintf(stdout, "Physical Volume UUID: %v\n", info.PvUUID)
			fmt.Fprintf(stdout, "Known Paths: %v\n",
				strings.Join(info.Paths, " "))
			if len(info.Tags) != 0 {
				fmt.Fprintf(stdout, "Tags:\n")
				for k, v := range info.Tags {
					fmt.Fprintf(stdout, "  %v: %v\n", k, v)
				}
			}

			fmt.Fprintf(stdout, "Bricks:\n")
			for _, d := range info.Bricks {
				fmt.Fprintf(stdout, "Id:%-35v"+
					"Size (GiB):%-8v"+
					"Path: %v\n",
					d.Id,
					d.Size/(1024*1024),
					d.Path)
			}
		}
		return nil

	},
}

var deviceEnableCommand = &cobra.Command{
	Use:     "enable [device_id]",
	Short:   "Allows device to go online",
	Long:    "Allows device to go online",
	Example: "  $ heketi-cli device enable 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("device id missing")
		}

		//set clusterId
		deviceId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		req := &api.StateRequest{
			State: "online",
		}
		err = heketi.DeviceState(deviceId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Device %v is now online\n", deviceId)
		}

		return err
	},
}

var deviceDisableCommand = &cobra.Command{
	Use:     "disable [device_id]",
	Short:   "Disallow usage of a device by placing it offline",
	Long:    "Disallow usage of a device by placing it offline",
	Example: "  $ heketi-cli device disable 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("device id missing")
		}

		//set clusterId
		deviceId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		req := &api.StateRequest{
			State: "offline",
		}
		err = heketi.DeviceState(deviceId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Device %v is now offline\n", deviceId)
		}

		return err
	},
}

func syncDevicesWithClusterId(heketi *client.Client, clusterId string) error {
	// Get Topology Info from Server
	topoinfo, err := heketi.TopologyInfo()
	if err != nil {
		return err
	}

	// Iterate over Clusters
	for cluster, _ := range topoinfo.ClusterList {
		thisCluster := topoinfo.ClusterList[cluster]
		// check cluster id.
		if thisCluster.Id == clusterId {
			// iterate over Nodes
			for node, _ := range thisCluster.Nodes {
				thisNode := thisCluster.Nodes[node]
				// resync every Device in this Node
				for _, device := range thisNode.DevicesInfo {
					fmt.Fprintf(stdout, "Now resyncing device %v..\n", device.Id)
					// Call DeviceResync from Heketi
					heketi.DeviceResync(device.Id)
					if err == nil {
						fmt.Fprintf(stdout, "Device %v (node %v) in cluster %v updated\n",
							device.Id,
							thisNode.Id,
							thisCluster.Id)
					}
				}
			}
		}
	}
	return nil
}

var deviceResyncCommand = &cobra.Command{
	Use:     "resync [object_id]",
	Short:   "Resync storage information about the device with operation system",
	Long:    "Resync storage information about the device with operation system",
	Example: `  $ heketi-cli device resync 886a86a868711bef83001 `,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("object/cluster id missing")
		}

		// set objectId
		objectId := cmd.Flags().Arg(0)
		// determine operation mode: CLUSTER od DEVICE
		resyncCluster, err := cmd.Flags().GetBool("cluster")
		if err != nil {
			return err
		}

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		if resyncCluster == false {
			err = heketi.DeviceResync(objectId)
			if err == nil {
				fmt.Fprintf(stdout, "Device %v updated\n", objectId)
			}
		} else { // requested cluster resync..
			fmt.Println("Cluster-wide resync operation requested. Using object_id as cluster_id.")
			err := syncDevicesWithClusterId(heketi, objectId)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var deviceSetTagsCommand = &cobra.Command{
	Use:     "settags [device_id] tag1:value1 tag2:value2...",
	Short:   "Sets tags on a device",
	Long:    "Sets user-controlled metadata tags on a device",
	Example: "  $ heketi-cli device settags 886a86a868711bef83001 foo:bar",
	RunE: func(cmd *cobra.Command, args []string) error {

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		return setTagsCommand(cmd, heketi.DeviceSetTags)
	},
}

var deviceRmTagsCommand = &cobra.Command{
	Use:     "rmtags [device_id] tag1:value1 tag2:value2...",
	Aliases: []string{"deltags", "removetags"},
	Short:   "Removes tags from a device",
	Long:    "Removes user-controlled metadata tags on a device",
	Example: "  $ heketi-cli device rmtags 886a86a868711bef83001 foo",
	RunE: func(cmd *cobra.Command, args []string) error {

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		return rmTagsCommand(cmd, heketi.DeviceSetTags)
	},
}

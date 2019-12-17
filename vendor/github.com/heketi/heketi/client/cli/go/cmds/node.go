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
	"io"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/spf13/cobra"
)

var (
	zone               int
	managmentHostNames string
	storageHostNames   string
	clusterId          string
)

func init() {
	RootCmd.AddCommand(nodeCommand)
	nodeCommand.AddCommand(nodeAddCommand)
	nodeCommand.AddCommand(nodeDeleteCommand)
	nodeCommand.AddCommand(nodeInfoCommand)
	nodeCommand.AddCommand(nodeEnableCommand)
	nodeCommand.AddCommand(nodeDisableCommand)
	nodeCommand.AddCommand(nodeListCommand)
	nodeCommand.AddCommand(nodeRemoveCommand)
	nodeCommand.AddCommand(nodeSetTagsCommand)
	nodeCommand.AddCommand(nodeRmTagsCommand)
	nodeAddCommand.Flags().IntVar(&zone, "zone", 0, "The zone in which the node should reside")
	nodeAddCommand.Flags().StringVar(&clusterId, "cluster", "", "The cluster in which the node should reside")
	nodeAddCommand.Flags().StringVar(&managmentHostNames, "management-host-name", "", "Management host name")
	nodeAddCommand.Flags().StringVar(&storageHostNames, "storage-host-name", "", "Storage host name")
	nodeSetTagsCommand.Flags().BoolP("exact", "e", false,
		"Set the object to this exact set of tags. Overwrites existing tags.")
	nodeRmTagsCommand.Flags().Bool("all", false,
		"Remove all tags.")
	nodeAddCommand.SilenceUsage = true
	nodeDeleteCommand.SilenceUsage = true
	nodeInfoCommand.SilenceUsage = true
	nodeListCommand.SilenceUsage = true
	nodeRemoveCommand.SilenceUsage = true
	nodeSetTagsCommand.SilenceUsage = true
}

var nodeCommand = &cobra.Command{
	Use:   "node",
	Short: "Heketi Node Management",
	Long:  "Heketi Node Management",
}

var nodeAddCommand = &cobra.Command{
	Use:   "add",
	Short: "Add new node to be managed by Heketi",
	Long:  "Add new node to be managed by Heketi",
	Example: `  $ heketi-cli node add \
      --zone=3 \
      --cluster=3e098cb4407d7109806bb196d9e8f095 \
      --management-host-name=node1-manage.gluster.lab.com \
      --storage-host-name=node1-storage.gluster.lab.com
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check arguments
		if zone == 0 {
			return errors.New("Missing zone")
		}
		if managmentHostNames == "" {
			return errors.New("Missing management hostname")
		}
		if storageHostNames == "" {
			return errors.New("Missing storage hostname")
		}
		if clusterId == "" {
			return errors.New("Missing cluster id")
		}

		// Create request blob
		req := &api.NodeAddRequest{}
		req.ClusterId = clusterId
		req.Hostnames.Manage = []string{managmentHostNames}
		req.Hostnames.Storage = []string{storageHostNames}
		req.Zone = zone

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		// Add node
		node, err := heketi.NodeAdd(req)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(node)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Node information:\n")
			printNodeInfo(stdout, node)
		}
		return nil
	},
}

var nodeDeleteCommand = &cobra.Command{
	Use:     "delete [node_id]",
	Short:   "Deletes a node from Heketi management",
	Long:    "Deletes a node from Heketi management",
	Example: "  $ heketi-cli node delete 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		//set clusterId
		nodeId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		err = heketi.NodeDelete(nodeId)
		if err == nil {
			fmt.Fprintf(stdout, "Node %v deleted\n", nodeId)
		}

		return err
	},
}

var nodeEnableCommand = &cobra.Command{
	Use:     "enable [node_id]",
	Short:   "Allows node to go online",
	Long:    "Allows node to go online",
	Example: "  $ heketi-cli node enable 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		//set clusterId
		nodeId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		req := &api.StateRequest{
			State: "online",
		}
		err = heketi.NodeState(nodeId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Node %v is now online\n", nodeId)
		}

		return err
	},
}

var nodeDisableCommand = &cobra.Command{
	Use:     "disable [node_id]",
	Short:   "Disallow usage of a node by placing it offline",
	Long:    "Disallow usage of a node by placing it offline",
	Example: "  $ heketi-cli node disable 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		//set clusterId
		nodeId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		req := &api.StateRequest{
			State: "offline",
		}
		err = heketi.NodeState(nodeId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Node %v is now offline\n", nodeId)
		}

		return err
	},
}

var nodeListCommand = &cobra.Command{
	Use:     "list all nodes",
	Short:   "List all nodes in cluster",
	Long:    "List all nodes in cluster",
	Example: "  $ heketi-cli node list",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		clusters, err := heketi.ClusterList()
		if err != nil {
			return err
		}

		for _, clusterid := range clusters.Clusters {
			clusterinfo, err := heketi.ClusterInfo(clusterid)
			if err != nil {
				return err
			}
			for _, nodeid := range clusterinfo.Nodes {
				fmt.Fprintf(stdout,
					"Id:%v\tCluster:%v\n",
					nodeid,
					clusterid)
			}
		}

		return err
	},
}

var nodeInfoCommand = &cobra.Command{
	Use:     "info [node_id]",
	Short:   "Retrieves information about the node",
	Long:    "Retrieves information about the node",
	Example: "  $ heketi-cli node info 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		s := cmd.Flags().Args()
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		// Set node id
		nodeId := cmd.Flags().Arg(0)

		// Create a client to talk to Heketi
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		info, err := heketi.NodeInfo(nodeId)
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
			printNodeInfo(stdout, info)
			fmt.Fprintf(stdout, "Devices:\n")
			for _, d := range info.DevicesInfo {
				fmt.Fprintf(stdout, "Id:%-35v"+
					"Name:%-20v"+
					"State:%-10v"+
					"Size (GiB):%-8v"+
					"Used (GiB):%-8v"+
					"Free (GiB):%-8v"+
					"Bricks:%-8v\n",
					d.Id,
					d.Name,
					entryStateString(d.State),
					d.Storage.Total/(1024*1024),
					d.Storage.Used/(1024*1024),
					d.Storage.Free/(1024*1024),
					len(d.Bricks))
			}
		}
		return nil
	},
}

var nodeRemoveCommand = &cobra.Command{
	Use:     "remove [node_id]",
	Short:   "Removes a node and all its associated devices from Heketi",
	Long:    "Removes a node and all its associated devices from Heketi",
	Example: "  $ heketi-cli node remove 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Flags().Args()

		//ensure proper number of args
		if len(s) < 1 {
			return errors.New("Node id missing")
		}

		//set clusterId
		nodeId := cmd.Flags().Arg(0)

		// Create a client
		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}

		//set url
		req := &api.StateRequest{
			State: "failed",
		}
		err = heketi.NodeState(nodeId, req)
		if err == nil {
			fmt.Fprintf(stdout, "Node %v is now removed\n", nodeId)
		}

		return err
	},
}

var nodeSetTagsCommand = &cobra.Command{
	Use:     "settags [node_id] tag1:value1 tag2:value2...",
	Short:   "Sets tags on a node",
	Long:    "Sets user-controlled metadata tags on a node",
	Example: "  $ heketi-cli node settags 886a86a868711bef83001 foo:bar",
	RunE: func(cmd *cobra.Command, args []string) error {

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		return setTagsCommand(cmd, heketi.NodeSetTags)
	},
}

var nodeRmTagsCommand = &cobra.Command{
	Use:     "rmtags [node_id] tag1:value1 tag2:value2...",
	Aliases: []string{"deltags", "removetags"},
	Short:   "Removes tags from a node",
	Long:    "Removes user-controlled metadata tags on a node",
	Example: "  $ heketi-cli node rmtags 886a86a868711bef83001 foo",
	RunE: func(cmd *cobra.Command, args []string) error {

		heketi, err := newHeketiClient()
		if err != nil {
			return err
		}
		return rmTagsCommand(cmd, heketi.NodeSetTags)
	},
}

func printNodeInfo(w io.Writer, info *api.NodeInfoResponse) {
	fmt.Fprintf(stdout, "Node Id: %v\n"+
		"State: %v\n"+
		"Cluster Id: %v\n"+
		"Zone: %v\n"+
		"Management Hostname: %v\n"+
		"Storage Hostname: %v\n",
		info.Id,
		entryStateString(info.State),
		info.ClusterId,
		info.Zone,
		info.Hostnames.Manage[0],
		info.Hostnames.Storage[0])
	if len(info.Tags) != 0 {
		fmt.Fprintf(stdout, "Tags:\n")
		for k, v := range info.Tags {
			fmt.Fprintf(stdout, "  %v: %v\n", k, v)
		}
	}
}

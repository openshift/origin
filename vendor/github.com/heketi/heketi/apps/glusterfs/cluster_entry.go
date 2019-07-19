//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/lpabon/godbc"
)

type ClusterEntry struct {
	Info api.ClusterInfoResponse
}

func ClusterList(tx *bolt.Tx) ([]string, error) {

	list := EntryKeys(tx, BOLTDB_BUCKET_CLUSTER)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewClusterEntry() *ClusterEntry {
	entry := &ClusterEntry{}
	entry.Info.Nodes = make(sort.StringSlice, 0)
	entry.Info.Volumes = make(sort.StringSlice, 0)
	entry.Info.BlockVolumes = make(sort.StringSlice, 0)
	entry.Info.Block = false
	entry.Info.File = false

	return entry
}

func NewClusterEntryFromRequest(req *api.ClusterCreateRequest) *ClusterEntry {
	godbc.Require(req != nil)

	entry := NewClusterEntry()
	entry.Info.Id = idgen.GenUUID()
	entry.Info.Block = req.Block
	entry.Info.File = req.File

	return entry
}

func NewClusterEntryFromId(tx *bolt.Tx, id string) (*ClusterEntry, error) {

	entry := NewClusterEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (c *ClusterEntry) BucketName() string {
	return BOLTDB_BUCKET_CLUSTER
}

func (c *ClusterEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(c.Info.Id) > 0)

	return EntrySave(tx, c, c.Info.Id)
}

func (c *ClusterEntry) ConflictString() string {
	return fmt.Sprintf("Unable to delete cluster [%v] because it contains volumes and/or nodes", c.Info.Id)
}

func (c *ClusterEntry) Delete(tx *bolt.Tx) error {
	godbc.Require(tx != nil)

	// Check if the cluster still has nodes or volumes
	if len(c.Info.Nodes) > 0 || len(c.Info.Volumes) > 0 {
		logger.Warning(c.ConflictString())
		return ErrConflict
	}

	return EntryDelete(tx, c, c.Info.Id)
}

func (c *ClusterEntry) NewClusterInfoResponse(tx *bolt.Tx) (*api.ClusterInfoResponse, error) {

	info := &api.ClusterInfoResponse{}
	*info = c.Info

	return info, nil
}

func (c *ClusterEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*c)

	return buffer.Bytes(), err
}

func (c *ClusterEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(c)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if c.Info.Nodes == nil {
		c.Info.Nodes = make(sort.StringSlice, 0)
	}
	if c.Info.Volumes == nil {
		c.Info.Volumes = make(sort.StringSlice, 0)
	}
	if c.Info.BlockVolumes == nil {
		c.Info.BlockVolumes = make(sort.StringSlice, 0)
	}

	return nil
}

func (c *ClusterEntry) NodeEntryFromClusterIndex(tx *bolt.Tx, index int) (*NodeEntry, error) {
	node, err := NewNodeEntryFromId(tx, c.Info.Nodes[index])
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (c *ClusterEntry) NodeAdd(id string) {
	c.Info.Nodes = append(c.Info.Nodes, id)
	c.Info.Nodes.Sort()
}

func (c *ClusterEntry) VolumeAdd(id string) {
	c.Info.Volumes = append(c.Info.Volumes, id)
	c.Info.Volumes.Sort()
}

func (c *ClusterEntry) VolumeDelete(id string) {
	c.Info.Volumes = sortedstrings.Delete(c.Info.Volumes, id)
}

func (c *ClusterEntry) BlockVolumeAdd(id string) {
	c.Info.BlockVolumes = append(c.Info.BlockVolumes, id)
	c.Info.BlockVolumes.Sort()
}

func (c *ClusterEntry) BlockVolumeDelete(id string) {
	c.Info.BlockVolumes = sortedstrings.Delete(c.Info.BlockVolumes, id)
}

func (c *ClusterEntry) NodeDelete(id string) {
	c.Info.Nodes = sortedstrings.Delete(c.Info.Nodes, id)
}

func ClusterEntryUpgrade(tx *bolt.Tx) error {
	err := addBlockFileFlagsInClusterEntry(tx)
	if err != nil {
		return err
	}
	return nil
}

func addBlockFileFlagsInClusterEntry(tx *bolt.Tx) error {
	entry, err := NewDbAttributeEntryFromKey(tx, DB_CLUSTER_HAS_FILE_BLOCK_FLAG)
	// This key won't exist if we are introducing the feature now
	if err != nil && err != ErrNotFound {
		return err
	}

	if err == ErrNotFound {
		entry = NewDbAttributeEntry()
		entry.Key = DB_CLUSTER_HAS_FILE_BLOCK_FLAG
		entry.Value = "no"
	} else {
		// This case is only for future, if ever we want to set this key to "no"
		if entry.Value == "yes" {
			return nil
		}
	}

	clusters, err := ClusterList(tx)
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		clusterEntry, err := NewClusterEntryFromId(tx, cluster)
		if err != nil {
			return err
		}
		clusterEntry.Info.Block = true
		clusterEntry.Info.File = true
		err = clusterEntry.Save(tx)
		if err != nil {
			return err
		}
	}

	entry.Value = "yes"
	return entry.Save(tx)
}

func (c *ClusterEntry) DeleteBricksWithEmptyPath(tx *bolt.Tx) error {

	logger.Debug("Deleting bricks with empty path in cluster [%v].",
		c.Info.Id)

	for _, nodeid := range c.Info.Nodes {
		node, err := NewNodeEntryFromId(tx, nodeid)
		if err == ErrNotFound {
			logger.Warning("Ignoring nonexisting node [%v] in "+
				"cluster [%v].", nodeid, c.Info.Id)
			continue
		}
		if err != nil {
			return err
		}

		err = node.DeleteBricksWithEmptyPath(tx)
		if err != nil {
			return err
		}
	}
	return nil
}

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
	"sync"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/utils"
)

// ReclaimMap tracks what bricks freed underlying storage when deleted.
// Deleting a brick does not always free space on the LV if snapshots are
// in use. The ReclaimMap values are set to true if the given brick id
// in the key freed underlying storage and false if not.
type ReclaimMap map[string]bool

// brickHostMap maps brick entries to hostnames.
// The host names are generally used for execution of brick commands.
// A brick host map gathers all the information needed to execute
// commands from the db prior to execution.
type brickHostMap map[*BrickEntry]string

// newBrickHostMap creates a mapping of brick entries to the hosts
// that commands for that brick can be executed on.
func newBrickHostMap(
	db wdb.RODB, bricks []*BrickEntry) (brickHostMap, error) {

	bmap := brickHostMap{}
	for _, brick := range bricks {
		var err error
		bmap[brick], err = brick.host(db)
		if err != nil {
			return nil, err
		}
	}
	return bmap, nil
}

// create executes commands to create every brick in the map using
// the mapped host. If a brick fails to create the function
// tries to automatically clean up the other created bricks.
func (bmap brickHostMap) create(executor executors.Executor) error {
	sg := utils.NewStatusGroup()
	// Create a goroutine for each brick
	for brick, host := range bmap {
		sg.Add(1)
		go func(b *BrickEntry, host string) {
			defer sg.Done()
			logger.Info("Creating brick %v", b.Info.Id)
			_, err := executor.BrickCreate(host, b.createReq())
			sg.Err(err)
		}(brick, host)
	}

	err := sg.Result()
	if err != nil {
		logger.Err(err)
		// Destroy all bricks and cleanup
		if _, err := bmap.destroy(executor); err != nil {
			logger.LogError(
				"error destroying bricks after create failure: %v", err)
		}
	}

	return err
}

// destroy executes commands to destroy/delete every brick in the map
// using the mapped host.
func (bmap brickHostMap) destroy(
	executor executors.Executor) (ReclaimMap, error) {

	sg := utils.NewStatusGroup()
	// return a map with the deviceId as key, and a bool if the space has been free'd
	reclaimed := map[string]bool{}
	// the mutex is used to prevent "fatal error: concurrent map writes"
	mutex := sync.Mutex{}

	// Create a goroutine for each brick
	for brick, host := range bmap {
		sg.Add(1)
		go func(b *BrickEntry, host string, r map[string]bool, m *sync.Mutex) {
			defer sg.Done()
			spaceReclaimed, err := executor.BrickDestroy(host, b.destroyReq())
			if err != nil {
				logger.LogError("error destroying brick %v: %v",
					b.Info.Id, err)
			} else {
				// mark space from device as freed
				m.Lock()
				r[b.Info.DeviceId] = spaceReclaimed
				m.Unlock()
			}

			sg.Err(err)
		}(brick, host, reclaimed, &mutex)
	}

	err := sg.Result()
	if err != nil {
		logger.Err(err)
	}

	return reclaimed, err
}

// CreateBricks is a deprecated wrapper for creating multiple bricks.
func CreateBricks(db wdb.RODB, executor executors.Executor, brick_entries []*BrickEntry) error {
	bmap, err := newBrickHostMap(db, brick_entries)
	if err != nil {
		return err
	}
	return bmap.create(executor)
}

// DestroyBricks is a deprecated wrapper for destroying multiple bricks.
func DestroyBricks(db wdb.RODB, executor executors.Executor, brick_entries []*BrickEntry) (ReclaimMap, error) {
	bmap, err := newBrickHostMap(db, brick_entries)
	if err != nil {
		return nil, err
	}
	return bmap.destroy(executor)
}

//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"encoding/json"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
)

func (a *App) BlockVolumeCreate(w http.ResponseWriter, r *http.Request) {

	var msg api.BlockVolumeCreateRequest
	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w, "request unable to be parsed", 422)
		return
	}

	err = msg.Validate()
	if err != nil {
		http.Error(w, "validation failed: "+err.Error(), http.StatusBadRequest)
		logger.LogError("validation failed: " + err.Error())
		return
	}

	if msg.Size < 1 {
		http.Error(w, "Invalid volume size", http.StatusBadRequest)
		logger.LogError("Invalid volume size")
		return
	}

	// TODO: factor this into a function (it's also in VolumeCreate)
	// Check that the clusters requested are available
	err = a.db.View(func(tx *bolt.Tx) error {

		// :TODO: All we need to do is check for one instead of gathering all keys
		clusters, err := ClusterList(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		if len(clusters) == 0 {
			err := logger.LogError("No clusters configured")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return ErrNotFound
		}

		// Check the clusters requested are correct
		for _, clusterid := range msg.Clusters {
			_, err := NewClusterEntryFromId(tx, clusterid)
			if err != nil {
				err := logger.LogError("Cluster id %v not found", clusterid)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return err
			}
		}

		return nil
	})
	if err != nil {
		return
	}

	blockVolume := NewBlockVolumeEntryFromRequest(&msg)

	bvc := NewBlockVolumeCreateOperation(blockVolume, a.db)
	if err := AsyncHttpOperation(a, w, r, bvc); err != nil {
		OperationHttpErrorf(w, err, "Failed to allocate new block volume: %v", err)
		return
	}
}

func (a *App) BlockVolumeList(w http.ResponseWriter, r *http.Request) {

	var list api.BlockVolumeListResponse

	err := a.db.View(func(tx *bolt.Tx) error {
		var err error

		list.BlockVolumes, err = ListCompleteBlockVolumes(tx)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logger.Err(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send list back
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(list); err != nil {
		panic(err)
	}
}

func (a *App) BlockVolumeInfo(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	// Get volume information
	var info *api.BlockVolumeInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewBlockVolumeEntryFromId(tx, id)
		if err == ErrNotFound || !entry.Visible() {
			http.Error(w, "Id not found", http.StatusNotFound)
			return ErrNotFound
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (a *App) BlockVolumeDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var blockVolume *BlockVolumeEntry
	err := a.db.View(func(tx *bolt.Tx) error {
		var err error
		blockVolume, err = NewBlockVolumeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	vdel := NewBlockVolumeDeleteOperation(blockVolume, a.db)
	if err := AsyncHttpOperation(a, w, r, vdel); err != nil {
		OperationHttpErrorf(w, err, "Failed to set up block volume delete: %v", err)
		return
	}
}

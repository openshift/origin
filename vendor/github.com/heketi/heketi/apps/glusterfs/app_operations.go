//
// Copyright (c) 2018 The heketi Authors
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

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
)

func (a *App) OperationsInfo(w http.ResponseWriter, r *http.Request) {
	info := &api.OperationsInfo{}

	err := a.db.View(func(tx *bolt.Tx) error {
		ops, err := PendingOperationList(tx)
		if err != nil {
			return err
		}
		info.Total = uint64(len(ops))
		m, err := PendingOperationStateCount(tx)
		if err != nil {
			return err
		}
		info.New = uint64(m[NewOperation])
		info.Stale = uint64(m[StaleOperation])
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info.InFlight = a.opcounter.Get()

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

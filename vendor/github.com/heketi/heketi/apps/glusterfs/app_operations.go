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
	"fmt"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
)

func (a *App) AppOperationsInfo() (*api.OperationsInfo, error) {
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
		info.Failed = uint64(m[FailedOperation])
		return nil
	})
	if err != nil {
		return nil, err
	}

	info.InFlight = a.optracker.Get()

	return info, nil
}

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
		info.Failed = uint64(m[FailedOperation])
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info.InFlight = a.optracker.Get()

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (a *App) PendingOperationList(w http.ResponseWriter, r *http.Request) {
	p := &api.PendingOperationListResponse{}
	tracked := a.optracker.Tracked()

	err := a.db.View(func(tx *bolt.Tx) error {
		ops, err := PendingOperationList(tx)
		if err != nil {
			return err
		}
		p.PendingOperations = make([]api.PendingOperationInfo, len(ops))
		for i, pid := range ops {
			pop, err := NewPendingOperationEntryFromId(tx, pid)
			if err != nil {
				return err
			}
			p.PendingOperations[i] = pop.ToInfo()
			if tracked[pop.Id] {
				p.PendingOperations[i].SubStatus = "in-flight"
			}
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		panic(err)
	}
}

func (a *App) PendingOperationDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pid := vars["id"]
	var info *api.PendingOperationDetails

	err := a.db.View(func(tx *bolt.Tx) error {
		pop, err := NewPendingOperationEntryFromId(tx, pid)
		if err != nil {
			return err
		}
		info = &api.PendingOperationDetails{
			PendingOperationInfo: pop.ToInfo(),
			Changes:              make([]api.PendingChangeInfo, len(pop.Actions)),
		}
		for i, a := range pop.Actions {
			info.Changes[i] = api.PendingChangeInfo{
				Id:          a.Id,
				Description: a.Change.Name(),
			}
		}
		return nil
	})
	if err == ErrNotFound {
		http.Error(w, fmt.Sprintf("Id not found: %v", pid), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (a *App) PendingOperationCleanUp(w http.ResponseWriter, r *http.Request) {

	// Unmarshal JSON
	var msg api.PendingOperationsCleanRequest
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

	// create an ops-to-delete map from request
	// if this is empty we will attempt to clean *all* ops
	ops := map[string]bool{}
	for _, id := range msg.Operations {
		ops[id] = true
	}

	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		err := a.OnDemandCleaner(ops).Clean()
		if err != nil {
			return "", err
		}
		return "", nil
	})
}

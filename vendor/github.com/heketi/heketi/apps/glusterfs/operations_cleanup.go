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
	"errors"
	"time"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"

	"github.com/boltdb/bolt"
)

type OperationCleaner struct {
	db       wdb.DB
	sel      func(*PendingOperationEntry) bool
	executor executors.Executor

	// operations tracker. This will be unset if run in offline mode
	optracker *OpTracker
	opClass   OpClass
}

func (oc OperationCleaner) Clean() error {
	logger.Debug("Going to clean up operations")
	var pops []*PendingOperationEntry
	err := oc.db.View(func(tx *bolt.Tx) error {
		var err error
		pops, err = PendingOperationEntrySelection(tx, oc.sel)
		return err
	})
	if err != nil {
		return err
	}

	for _, pop := range pops {
		logger.Info("Found operation %v in need of clean up", pop.Id)
		op, err := LoadOperation(oc.db, pop)
		if _, ok := err.(ErrNotLoadable); ok {
			logger.Err(err)
			continue
		} else if err == ErrNotFound {
			// TODO: flag/process pending ops with bad references more sanely
			// for now, just skip over them
			logger.LogError("unable to load operation [%v]: %v",
				pop.Id, err)
			continue
		} else if err != nil {
			return err
		}
		cop, ok := op.(CleanableOperation)
		if !ok {
			logger.Warning("%v operation %v not cleanable", op.Label(), pop.Id)
			continue
		}
		// TODO gather errors
		err = oc.cleanOp(cop)
		if err != nil {
			// cleanOp errors are non-fatal as they can always
			// be retried later
			logger.Warning("Unable to clean operation %v: %v",
				cop.Id(), err)
		}
	}
	return nil
}

func (oc OperationCleaner) cleanOp(cop CleanableOperation) error {
	if !oc.cleanBegin(cop.Id()) {
		logger.Warning("Not starting clean of %v, not ready", cop.Id())
		return nil
	}
	defer oc.cleanEnd(cop.Id())
	err := cop.Clean(oc.executor)
	if err != nil {
		logger.Warning("Clean phase of operation %v encountered error: %v",
			cop.Id(), err)
		return err
	}
	return cop.CleanDone()
}

// cleanBegin returns true if a clean of the given operation
// can start at this time.
func (oc OperationCleaner) cleanBegin(id string) bool {
	if oc.optracker == nil {
		// no tracker in use
		return true
	}
	if oc.optracker.ThrottleOrAdd(id, oc.opClass) {
		logger.Warning("Clean of operation %v thottled (class=%v)",
			id, oc.opClass)
		return false
	}
	return true
}

// cleanEnd releases any resources taken by cleanBegin
func (oc OperationCleaner) cleanEnd(id string) {
	if oc.optracker == nil {
		return
	}
	oc.optracker.Remove(id)
}

// MarkStale finds pending operations in the db that are not yet
// stale and marks them as such. This is useful in case the
// pending operation failed but the pending op could not be
// marked failed (db error) in a long running server.
func (oc OperationCleaner) MarkStale() error {
	if oc.optracker == nil {
		return errors.New("Can not mark stale without op tracker")
	}
	logger.Debug("Going to mark stale operations")
	now := operationTimestamp()
	sel := func(p *PendingOperationEntry) bool {
		// operations must be new & older than 60 seconds to be selected
		return p.Status == NewOperation && (now-p.Timestamp) >= 60
	}
	return oc.db.Update(func(tx *bolt.Tx) error {
		pops, err := PendingOperationEntrySelection(tx, sel)
		if err != nil {
			return err
		}
		tracked := oc.optracker.Tracked()
		for _, pop := range pops {
			if tracked[pop.Id] {
				continue
			}
			logger.Info("found untracked new operation [%v]: marking stale",
				pop.Id)
			pop.Status = StaleOperation
			if err := pop.Save(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

type backgroundOperationCleaner struct {
	cleaner OperationCleaner

	// timing params
	StartInterval time.Duration
	CheckInterval time.Duration

	// to stop the monitor
	stop chan<- interface{}
}

// Start creates a background goroutine to run periodic cleans
// of stale and failed pending operations.
func (boc *backgroundOperationCleaner) Start() {
	startTimer := time.NewTimer(boc.StartInterval)
	ticker := time.NewTicker(boc.CheckInterval)
	stop := make(chan interface{})
	boc.stop = stop

	go func() {
		logger.Info("Started background pending operations cleaner")
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				logger.Info("Stopping background pending operations cleaner")
				return
			case <-startTimer.C:
				err := boc.cleaner.Clean()
				if err != nil {
					logger.LogError(
						"Background pending operations cleaner: %v", err)
				}
			case <-ticker.C:
				// the periodic clean first looks for any ops that have
				// gone stale in the meantime. This is not done for the
				// start time because we would have just restarted and
				// marked everything in the db as stale.
				err := boc.cleaner.MarkStale()
				if err != nil {
					logger.LogError(
						"Background pending operations mark stale: %v", err)
				}
				err = boc.cleaner.Clean()
				if err != nil {
					logger.LogError(
						"Background pending operations cleaner: %v", err)
				}
			}
		}
	}()
}

// Stop the background operations cleaner.
func (boc *backgroundOperationCleaner) Stop() {
	boc.stop <- true
}

func CleanAll(p *PendingOperationEntry) bool {
	return p.Status == StaleOperation || p.Status == FailedOperation
}

// CleanSelectedOps is a factory function that returns a new selection
// function which will only match cleanable pending ops with IDs
// in the specified map.
func CleanSelectedOps(
	ops map[string]bool) func(p *PendingOperationEntry) bool {

	return func(p *PendingOperationEntry) bool {
		if !ops[p.Id] {
			return false
		}
		if !CleanAll(p) {
			logger.Debug("Selected pending op id %v was not cleanable", p.Id)
			return false
		}
		logger.Debug("matched pending operation id: %v", p.Id)
		return true
	}
}

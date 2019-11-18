// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

type rsrc struct {
	closed bool
}

func initRsrc() interface{} {
	return &rsrc{}
}

func closeRsrc(v interface{}) {
	v.(*rsrc).closed = true
}

func alwaysExpired(_ interface{}) bool {
	return true
}

func neverExpired(_ interface{}) bool {
	return false
}

// expiredCounter is used to implement an expiredFunc that will return true a fixed number of times.
type expiredCounter struct {
	total, expiredCalled, closeCalled int32
	closeChan                         chan struct{}
}

func newExpiredCounter(total int32) expiredCounter {
	return expiredCounter{
		total:     total,
		closeChan: make(chan struct{}, 1),
	}
}

func (ec *expiredCounter) expired(_ interface{}) bool {
	atomic.AddInt32(&ec.expiredCalled, 1)
	return ec.expiredCalled <= ec.total
}

func (ec *expiredCounter) close(_ interface{}) {
	atomic.AddInt32(&ec.closeCalled, 1)
	if ec.closeCalled == ec.total {
		ec.closeChan <- struct{}{}
	}
}

func initPool(t *testing.T, minSize uint64, expFn expiredFunc, closeFn closeFunc, initFn initFunc, pruneInterval time.Duration) *resourcePool {
	rpc := resourcePoolConfig{
		MinSize:          minSize,
		MaintainInterval: pruneInterval,
		ExpiredFn:        expFn,
		CloseFn:          closeFn,
		InitFn:           initFn,
	}
	rp, err := newResourcePool(rpc)
	require.NoError(t, err, "error creating new resource pool")
	rp.initialize()
	rp.maintainTimer.Reset(rp.maintainInterval)
	return rp
}

func TestResourcePool(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		t.Run("remove stale resources", func(t *testing.T) {
			ec := newExpiredCounter(5)
			rp := initPool(t, 1, ec.expired, ec.close, initRsrc, time.Minute)
			rp.maintainTimer.Stop()

			if got := rp.Get(); got != nil {
				t.Fatalf("resource mismatch; expected nil, got %v", got)
			}
			if rp.size != 0 {
				t.Fatalf("length mismatch; expected 0, got %d", rp.size)
			}
			if ec.expiredCalled != 1 {
				t.Fatalf("incorrect number of expire checks, expected 1, got %v", ec.expiredCalled)
			}
			if ec.closeCalled != 1 {
				t.Fatalf("incorrect number of closes called, expected 1, got %v", ec.closeCalled)
			}
		})
		t.Run("recycle resources", func(t *testing.T) {
			rp := initPool(t, 1, neverExpired, closeRsrc, initRsrc, time.Minute)
			rp.maintainTimer.Stop()
			for i := 0; i < 5; i++ {
				got := rp.Get()
				if got == nil {
					t.Fatalf("resource mismatch; expected a resource but got nil")
				}
				if rp.size != 0 {
					t.Fatalf("length mismatch; expected 0, got %d", rp.size)
				}
				rp.Put(got)
				if rp.size != 1 {
					t.Fatalf("length mismatch; expected 1, got %d", rp.size)
				}
			}
		})
	})
	t.Run("Put", func(t *testing.T) {
		t.Run("returned resources are returned to front of pool", func(t *testing.T) {
			rp := initPool(t, 0, neverExpired, closeRsrc, initRsrc, time.Minute)
			ret := &rsrc{}
			if !rp.Put(ret) {
				t.Fatal("return value mismatch; expected true, got false")
			}
			if rp.size != 1 {
				t.Fatalf("length mismatch; expected 1, got %d", rp.size)
			}
			if headVal := rp.Get(); headVal != ret {
				t.Fatalf("resource mismatch; expected %v at head of pool, got %v", ret, headVal)
			}
		})
		t.Run("stale resource not returned", func(t *testing.T) {
			rp := initPool(t, 1, alwaysExpired, closeRsrc, initRsrc, time.Minute)
			ret := &rsrc{}
			if rp.Put(ret) {
				t.Fatal("return value mismatch; expected false, got true")
			}
		})
	})
	t.Run("Prune", func(t *testing.T) {
		t.Run("removes all stale resources", func(t *testing.T) {
			ec := newExpiredCounter(3)
			rp := initPool(t, 0, ec.expired, ec.close, initRsrc, time.Minute)
			for i := 0; i < 5; i++ {
				ret := &rsrc{}
				_ = rp.Put(ret)
			}
			rp.Maintain()
			if rp.size != 2 {
				t.Fatalf("length mismatch; expected 2, got %d", rp.size)
			}
			if ec.expiredCalled != 7 {
				t.Fatalf("count mismatch; expected ec.stale to be called 7 times, got %v", ec.expiredCalled)
			}
			if ec.closeCalled != 3 {
				t.Fatalf("count mismatch; expected ex.closeConnection to be called 3 times, got %v", ec.closeCalled)
			}
		})
	})
	t.Run("Background cleanup", func(t *testing.T) {
		t.Run("runs once every interval", func(t *testing.T) {
			ec := newExpiredCounter(3)
			dur := 100 * time.Millisecond
			rp := initPool(t, 0, neverExpired, ec.close, initRsrc, dur)
			rp.maintainTimer.Stop()

			for i := 0; i < 5; i++ {
				ret := &rsrc{}
				_ = rp.Put(ret)
			}

			rp.expiredFn = ec.expired
			rp.maintainTimer.Reset(dur)

			select {
			case <-ec.closeChan:
			case <-time.After(5 * time.Second):
				t.Fatalf("value was not read on closeChan after 5 seconds")
			}

			if atomic.LoadInt32(&ec.expiredCalled) != 5 {
				t.Fatalf("count mismatch; expected ec.stale to be called 5 times, got %v", ec.expiredCalled)
			}
			if atomic.LoadInt32(&ec.closeCalled) != 3 {
				t.Fatalf("count mismatch; expected ec.closeConnection to be called 5 times, got %v", ec.closeCalled)
			}
		})
	})
}

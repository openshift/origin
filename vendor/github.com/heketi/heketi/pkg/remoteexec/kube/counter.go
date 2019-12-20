//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kube

import (
	"sync"
)

type connectionCounter struct {
	count uint64
	lock  *sync.RWMutex
}

func newConnectionCounter() *connectionCounter {
	return &connectionCounter{
		count: 0,
		lock:  &sync.RWMutex{},
	}
}

func (c *connectionCounter) increment() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.count++
}

func (c *connectionCounter) decrement() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.count--
}

func (c *connectionCounter) get() uint64 {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.count
}

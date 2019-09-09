//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package admin

import (
	"fmt"
	"sync"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

type ServerState struct {
	state api.AdminState
	lock  sync.RWMutex
}

func New() *ServerState {
	return &ServerState{
		state: api.AdminStateNormal,
	}
}

func (s *ServerState) Set(newState api.AdminState) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.state = newState
}

func (s *ServerState) Get() api.AdminState {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.state
}

func (s *ServerState) SetString(v string) error {
	if v == "" {
		return nil // do nothing for empty string
	}
	state := api.AdminState(v)
	if state == api.AdminStateNormal ||
		state == api.AdminStateLocal ||
		state == api.AdminStateReadOnly {

		s.Set(state)
		return nil
	}
	return fmt.Errorf("unknown admin state name: %v", v)
}

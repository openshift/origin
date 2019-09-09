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
	"fmt"
)

// nodeHosts is a mapping from the node ID to the hosts's
// management name.
type nodeHosts map[string]string

type tryOnHosts struct {
	Hosts nodeHosts
	done  func(error) bool
	// nodesUp allows ht user of try on hosts to override the default
	// function for fetching
	nodesUp func() map[string]bool
}

func newTryOnHosts(hosts nodeHosts) *tryOnHosts {
	return &tryOnHosts{Hosts: hosts}
}

// nodeStatus return a map of all the known node status
// with a true value indicating the node is known to be up.
// The default behavior of this function can be controlled
// by setting the 'nodesUp' function in the struct.
func (c *tryOnHosts) nodeStatus() map[string]bool {
	if c.nodesUp == nil {
		return currentNodeHealthStatus()
	}
	return c.nodesUp()
}

// once returns a tryOnHosts that only tries one host known
// to be up.
func (c *tryOnHosts) once() *tryOnHosts {
	return &tryOnHosts{
		Hosts:   c.Hosts,
		nodesUp: c.nodesUp,
		done: func(err error) bool {
			return true
		},
	}
}

func (c *tryOnHosts) run(f func(host string) error) error {
	// if a custom done is not provided only stop
	// if err == nil
	done := c.done
	if done == nil {
		done = func(err error) bool {
			return err == nil
		}
	}

	tries := 0
	nodeUp := c.nodeStatus()
	for nodeId, host := range c.Hosts {
		if up, found := nodeUp[nodeId]; found && !up {
			// if the node is in the cache and we know it was not
			// recently healthy, skip it
			logger.Debug("skipping node. %v (%v) is presumed unhealthy",
				nodeId, host)
			continue
		}
		logger.Debug("running function on node %v (%v)", nodeId, host)
		tries++
		err := f(host)
		logger.Debug("running function on node %v got: %v", nodeId, err)
		if done(err) {
			return err
		}
		logger.Warning("error running on node %v (%v): %v", nodeId, host, err)
	}
	if tries == 0 {
		return fmt.Errorf("no hosts available (%v total)", len(c.Hosts))
	}
	return fmt.Errorf(
		"all hosts failed (%v total, %v tried)", len(c.Hosts), tries)
}

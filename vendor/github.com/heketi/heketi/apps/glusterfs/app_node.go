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
	"encoding/json"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
)

func (a *App) NodeAdd(w http.ResponseWriter, r *http.Request) {
	var msg api.NodeAddRequest

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

	// Check information in JSON request
	if len(msg.Hostnames.Manage) == 0 {
		http.Error(w, "Manage hostname missing", http.StatusBadRequest)
		return
	}
	if len(msg.Hostnames.Storage) == 0 {
		http.Error(w, "Storage hostname missing", http.StatusBadRequest)
		return
	}

	// Zone value of 0 is not allowed because we do not know
	// if it is because it was set to zero, or it is the default
	// value used for missing 'zone' in JSON
	if msg.Zone == 0 {
		http.Error(w, "Zone cannot be zero or value is missing", http.StatusBadRequest)
		return
	}

	// Check for correct values
	for _, name := range append(msg.Hostnames.Manage, msg.Hostnames.Storage...) {
		if name == "" {
			http.Error(w, "Hostname cannot be an empty string", http.StatusBadRequest)
			return
		}
	}

	// Create a node entry
	node := NewNodeEntryFromRequest(&msg)

	// Get cluster and peer node hostname
	var cluster *ClusterEntry
	var peer_node_hostname string
	err = a.db.Update(func(tx *bolt.Tx) error {
		var err error
		cluster, err = NewClusterEntryFromId(tx, msg.ClusterId)
		if err == ErrNotFound {
			http.Error(w, "Cluster id does not exist", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Register node
		err = node.Register(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	// Get a node's hostname in the cluster to execute the Gluster peer command
	// only if there is more than one node
	if len(cluster.Info.Nodes) > 0 {
		peer_node_hostname, err = GetVerifiedManageHostname(a.db, a.executor, cluster.Info.Id)
		if err != nil {
			logger.Err(err)
			err := logger.LogError("None of the nodes in cluster has glusterd running")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		err := a.executor.GlusterdCheck(node.ManageHostName())
		if err != nil {
			logger.Err(err)
			err := logger.LogError("New Node doesn't have glusterd running")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Add node
	logger.Info("Adding node %v", node.ManageHostName())
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (seeother string, e error) {

		// Cleanup in case of failure
		defer func() {
			if e != nil {
				a.db.Update(func(tx *bolt.Tx) error {
					node.Deregister(tx)
					return nil
				})
			}
		}()

		// Peer probe if there is at least one other node
		// TODO: What happens if the peer_node is not responding.. we need to choose another.
		// It will only choose the working one now. Hence done.
		if peer_node_hostname != "" {
			err := a.executor.PeerProbe(peer_node_hostname, node.StorageHostName())
			if err != nil {
				return "", err
			}
		}

		// Add node entry into the db
		err = a.db.Update(func(tx *bolt.Tx) error {
			cluster, err := NewClusterEntryFromId(tx, msg.ClusterId)
			if err == ErrNotFound {
				http.Error(w, "Cluster id does not exist", http.StatusNotFound)
				return err
			} else if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			// Add node to cluster
			cluster.NodeAdd(node.Info.Id)

			// Save cluster
			err = cluster.Save(tx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			// Save node
			err = node.Save(tx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			return nil

		})
		if err != nil {
			return "", err
		}
		logger.Info("Added node " + node.Info.Id)
		return "/nodes/" + node.Info.Id, nil
	})
}

func (a *App) NodeInfo(w http.ResponseWriter, r *http.Request) {

	// Get node id from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get Node information
	var info *api.NodeInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		info, err = entry.NewInfoReponse(tx)
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

func (a *App) NodeDelete(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get node info
	var (
		peer_node, node *NodeEntry
		cluster         *ClusterEntry
	)
	err := a.db.View(func(tx *bolt.Tx) error {

		// Access node entry
		var err error
		node, err = NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return logger.Err(err)
		}

		// Check the node can be deleted
		if !node.IsDeleteOk() {
			http.Error(w, node.ConflictString(), http.StatusConflict)
			logger.LogError(node.ConflictString())
			return ErrConflict
		}

		// Access cluster information and peer node
		cluster, err = NewClusterEntryFromId(tx, node.Info.ClusterId)
		if err == ErrNotFound {
			http.Error(w, "Cluster id does not exist", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return logger.Err(err)
		}

		// Get a node in the cluster to execute the Gluster peer command
		// If it only has one in the list, then there is no need to do a
		// peer detach.
		if len(cluster.Info.Nodes) > 1 {
			for index := range cluster.Info.Nodes {
				peer_node, err = cluster.NodeEntryFromClusterIndex(tx, index)
				if err != nil {
					return logger.Err(err)
				}

				// Cannot peer detach from the same node, we need to execute
				// the command from another node
				if peer_node.Info.Id != node.Info.Id {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	// Delete node asynchronously
	logger.Info("Deleting node %v [%v]", node.ManageHostName(), node.Info.Id)
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {

		// Remove from trusted pool
		if peer_node != nil {
			err := a.executor.PeerDetach(peer_node.ManageHostName(), node.StorageHostName())
			if err != nil {
				return "", err
			}
		}

		// Remove from db
		err = a.db.Update(func(tx *bolt.Tx) error {
			// Get Cluster
			cluster, err := NewClusterEntryFromId(tx, node.Info.ClusterId)
			if err == ErrNotFound {
				logger.Critical("Cluster id %v is expected be in db. Pointed to by node %v",
					node.Info.ClusterId,
					node.Info.Id)
				return err
			} else if err != nil {
				logger.Err(err)
				return err
			}
			cluster.NodeDelete(node.Info.Id)

			// Save cluster
			err = cluster.Save(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			// Remove hostnames
			node.Deregister(tx)

			// Delete node from db
			err = node.Delete(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			err = refreshVolumeNodes(tx, node)
			if err != nil {
				logger.Err(err)
				return err
			}
			return nil

		})
		if err != nil {
			return "", err
		}
		// Show that the key has been deleted
		logger.Info("Deleted node [%s]", id)

		return "", nil

	})
}

func refreshVolumeNodes(tx *bolt.Tx, node *NodeEntry) error {
	clusterID := node.Info.ClusterId
	deletedNodeHostName := node.StorageHostName()
	txdb := wdb.WrapTx(tx)
	// Get Cluster
	cluster, err := NewClusterEntryFromId(tx, clusterID)
	if err != nil {
		logger.Critical("Cluster id %v is expected be in db. Pointed to by node %v",
			clusterID,
			clusterID)
		return err
	}

	hosts, err := getHostsFromCluster(txdb, clusterID)
	if err != nil {
		return err
	}
	for _, volID := range cluster.Info.Volumes {
		volEntry, err := NewVolumeEntryFromId(tx, volID)
		if err != nil {
			logger.LogError("Get volume entry for ID %s Failed with error %s", volID, err.Error())
			continue
		}
		if volEntry.Info.Block {
			for _, id := range volEntry.Info.BlockInfo.BlockVolumes {

				blockEntry, err := NewBlockVolumeEntryFromId(tx, id)
				if err != nil {
					logger.LogError("Get block volume entry for ID %s Failed with error %s", id, err.Error())
					continue
				}

				blockEntry.updateHosts(hosts)

				if err := blockEntry.Save(tx); err != nil {
					logger.LogError("Save block entry for ID %s Failed with error %s", id, err.Error())
					continue
				}
			}
		}
		//update mountpoint info
		volEntry.updateHostandMountPoint(hosts, deletedNodeHostName)

		if err = volEntry.Save(tx); err != nil {
			logger.LogError("Save volume entry for ID  %s Failed with error %s", volEntry.Info.Id, err.Error())
			continue
		}
	}

	return nil
}
func (a *App) NodeSetState(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]
	var node *NodeEntry

	// Unmarshal JSON
	var msg api.StateRequest
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

	// Check state is supported
	err = a.db.View(func(tx *bolt.Tx) error {
		node, err = NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
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

	// Setting the state to failed can involve long running operations
	// and thus needs to be checked for operations throttle
	// However, we don't want to block "cheap" changes like setting
	// the item offline
	if msg.State == api.EntryStateFailed {
		if a.opcounter.ThrottleOrInc() {
			OperationHttpErrorf(w, ErrTooManyOperations, "")
			return
		}
	}

	// Set state
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		defer func() {
			if msg.State == api.EntryStateFailed {
				a.opcounter.Dec()
			}
		}()
		err = node.SetState(a.db, a.executor, msg.State)
		if err != nil {
			return "", err
		}
		return "", nil

	})

}

func (a *App) NodeSetTags(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]
	var node *NodeEntry

	// Unmarshal JSON
	var msg api.TagsChangeRequest
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

	err = a.db.Update(func(tx *bolt.Tx) error {
		node, err = NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		ApplyTags(node, msg)
		if err := node.Save(tx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		return nil
	})
	if err != nil {
		logger.Err(err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(node.AllTags()); err != nil {
		panic(err)
	}
}

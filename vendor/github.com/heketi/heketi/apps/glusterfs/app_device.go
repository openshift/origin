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
	"fmt"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
)

func (a *App) DeviceAdd(w http.ResponseWriter, r *http.Request) {

	var msg api.DeviceAddRequest
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

	// Check the message has devices
	if msg.Name == "" {
		http.Error(w, "no devices added", http.StatusBadRequest)
		return
	}

	// Create device entry
	device := NewDeviceEntryFromRequest(&msg)

	// Check the node is in the db
	var node *NodeEntry
	err = a.db.Update(func(tx *bolt.Tx) error {
		var err error
		node, err = NewNodeEntryFromId(tx, msg.NodeId)
		if err == ErrNotFound {
			http.Error(w, "Node id does not exist", http.StatusNotFound)
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

	// Log the devices are being added
	logger.Info("Adding device %v to node %v", msg.Name, msg.NodeId)

	// Add device in an asynchronous function
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (seeOtherUrl string, e error) {

		// Setup device on node
		info, err := a.executor.DeviceSetup(node.ManageHostName(),
			device.Info.Name, device.Info.Id, false)
		if err != nil && msg.DestroyData {
			errReason := allowDestroyDevice(a.db, err)
			if errReason != nil {
				// not allowed to destroy the device. return reason
				// as our error
				return "", errReason
			}
			info, err = a.executor.DeviceSetup(node.ManageHostName(),
				device.Info.Name, device.Info.Id, true)
		}
		if err != nil {
			return "", err
		}
		device.UpdateInfo(info)

		// Setup garbage collector on error
		defer func() {
			if e != nil {
				a.executor.DeviceTeardown(node.ManageHostName(), device.ToHandle())
			}
		}()

		// Save on db
		err = a.db.Update(func(tx *bolt.Tx) error {

			nodeEntry, err := NewNodeEntryFromId(tx, msg.NodeId)
			if err != nil {
				return err
			}

			// Add device to node
			nodeEntry.DeviceAdd(device.Info.Id)

			// Commit
			err = nodeEntry.Save(tx)
			if err != nil {
				return err
			}

			// Save drive
			err = device.Save(tx)
			if err != nil {
				return err
			}

			return nil

		})
		if err != nil {
			return "", err
		}

		logger.Info("Added device %v", msg.Name)

		// Done
		// Returning a null string instructs the async manager
		// to return http status of 204 (No Content)
		return "", nil
	})

}

func (a *App) DeviceInfo(w http.ResponseWriter, r *http.Request) {

	// Get device id from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get device information
	var info *api.DeviceInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewDeviceEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
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

func (a *App) DeviceDelete(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	var opts api.DeviceDeleteOptions
	if r.ContentLength > 0 {
		err := utils.GetJsonFromRequest(r, &opts)
		if err != nil {
			http.Error(w, "request unable to be parsed", 422)
			return
		}
	}

	// Check request
	var (
		device *DeviceEntry
		node   *NodeEntry
	)
	err := a.db.View(func(tx *bolt.Tx) error {
		var err error
		// Access device entry
		device, err = NewDeviceEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return logger.Err(err)
		}

		// Access node entry
		node, err = NewNodeEntryFromId(tx, device.NodeId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return logger.Err(err)
		}

		// Check if we can delete the device
		if err := device.CheckDelete(); err != nil {
			if err == ErrConflict {
				http.Error(w, device.ConflictString(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Delete device
	logger.Info("Deleting device %v on node %v", device.Info.Id, device.NodeId)
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {

		// Teardown device
		var err error
		dh := device.ToHandle()
		if opts.ForceForget {
			logger.Info("Delete request set force-forget option")
			err = a.executor.DeviceForget(node.ManageHostName(), dh)
		} else {
			err = a.executor.DeviceTeardown(node.ManageHostName(), dh)
		}
		if err != nil {
			return "", err
		}

		// Get info from db
		err = a.db.Update(func(tx *bolt.Tx) error {

			// Access node entry
			node, err := NewNodeEntryFromId(tx, device.NodeId)
			if err == ErrNotFound {
				logger.Critical(
					"Node id %v pointed to by device %v, but it is not in the db",
					device.NodeId,
					device.Info.Id)
				return err
			} else if err != nil {
				logger.Err(err)
				return err
			}

			// Delete device from node
			node.DeviceDelete(device.Info.Id)

			// Save node
			node.Save(tx)

			// Delete device from db
			err = device.Delete(tx)
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

func (a *App) DeviceSetState(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]
	var device *DeviceEntry

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

	// Check for valid id, return immediately if not valid
	err = a.db.View(func(tx *bolt.Tx) error {
		device, err = NewDeviceEntryFromId(tx, id)
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
	var token string
	if msg.State == api.EntryStateFailed {
		var throttled bool
		throttled, token = a.optracker.ThrottleOrToken()
		if throttled {
			OperationHttpErrorf(w, ErrTooManyOperations, "")
			return
		}
	}

	// Set state
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
		defer func() {
			if msg.State == api.EntryStateFailed {
				a.optracker.Remove(token)
			}
		}()
		err = device.SetState(a.db, a.executor, msg.State)
		if err != nil {
			return "", err
		}
		return "", nil
	})
}

func (a *App) DeviceResync(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	deviceId := vars["id"]

	var (
		device        *DeviceEntry
		node          *NodeEntry
		brickSizesSum uint64
	)

	err := a.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, deviceId)
		if err != nil {
			return err
		}
		node, err = NewNodeEntryFromId(tx, device.NodeId)
		return err
	})
	if err == ErrNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logger.Err(err)
		return
	}

	logger.Info("Checking for device %v changes", deviceId)

	// Check and update device in background
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (seeOtherUrl string, e error) {

		// Get actual device info from manage host
		info, err := a.executor.GetDeviceInfo(
			node.ManageHostName(), device.ToHandle())
		if err != nil {
			return "", err
		}

		err = a.db.Update(func(tx *bolt.Tx) error {

			if p, err := PendingOperationsOnDevice(wdb.WrapTx(tx), deviceId); err != nil {
				return err
			} else if p {
				logger.LogError("Found operations pending on device."+
					" Can not resync device %v at this time.",
					deviceId)
				return ErrConflict
			}

			// Reload device in current transaction
			device, err := NewDeviceEntryFromId(tx, deviceId)
			if err != nil {
				logger.Err(err)
				return err
			}

			for _, brick := range device.Bricks {
				brickEntry, err := NewBrickEntryFromId(tx, brick)
				if err != nil {
					return err
				}
				brickSizesSum += brickEntry.Info.Size
			}

			if brickSizesSum != info.UsedSize {
				logger.Info("Sum of sizes of all bricks on the device:%v differs from used size from LVM:%v", brickSizesSum, info.UsedSize)
				logger.Info("Database needs cleaning")
			}

			logger.Info("Updating device %v, total: %v -> %v, free: %v -> %v, used: %v -> %v", device.Info.Name,
				device.Info.Storage.Total, info.TotalSize, device.Info.Storage.Free, info.FreeSize, device.Info.Storage.Used, info.UsedSize)

			device.StorageSet(info.TotalSize, info.FreeSize, info.UsedSize)

			// Save updated device
			err = device.Save(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			return nil
		})
		if err != nil {
			return "", err
		}

		logger.Info("Updated device %v", deviceId)

		return "", err
	})
}

func (a *App) DeviceSetTags(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]
	var device *DeviceEntry

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
		device, err = NewDeviceEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		ApplyTags(device, msg)
		if err := device.Save(tx); err != nil {
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
	if err := json.NewEncoder(w).Encode(device.AllTags()); err != nil {
		panic(err)
	}
}

func allowDestroyDevice(db wdb.RODB, e error) error {
	if derr, ok := e.(*executors.DeviceNotAvailableErr); ok {
		if !derr.ConnectionOk {
			// device status could not be checked. deny destroy
			return derr
		}
		if derr.CurrentMeta == nil {
			// device was checked but is not a Physical Volume. is ok to wipe.
			return nil
		}
		pvmap, dcount, err := allDevicePvUUID(db)
		if err != nil {
			logger.LogError("failed to read PV UUIDs from db: %v", err)
			// db lookups failed. deny
			return derr
		}
		if len(pvmap) != dcount {
			logger.Warning(
				"devices without PV UUIDs detected (found %v uuids, %v devices)",
				len(pvmap), dcount)
			// can't check all devices (backwards compat old devices). deny
			return fmt.Errorf(
				"Can not destroy device, destroy may not be safe: %v",
				derr)
		}
		if deviceId, found := pvmap[derr.CurrentMeta.UUID]; found {
			logger.Warning(
				"device uuid %v already tracked as %v",
				derr.CurrentMeta.UUID, deviceId)
			return fmt.Errorf(
				"Device already in use (ID: %v)",
				deviceId)
		}
		// it has lvm metadata but not (this) heketi's. ok to wipe
		// destroydata is really never safe, but this is what we allowed
		// prior to the new approach to devices, so whatever. :-\
		return nil
	}
	// not an expected error type. better safe than sorry, deny destroy
	return e
}

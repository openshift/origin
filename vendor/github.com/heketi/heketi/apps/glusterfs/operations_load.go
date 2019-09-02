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

	_ "github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
)

type ErrNotLoadable struct {
	id     string
	optype PendingOperationType
}

func NewErrNotLoadable(id string, optype PendingOperationType) ErrNotLoadable {
	return ErrNotLoadable{id, optype}
}

func (e ErrNotLoadable) Error() string {
	return fmt.Sprintf("Operation %v is not a loadable type (%v)",
		e.id,
		e.optype)
}

func LoadOperation(
	db wdb.DB, p *PendingOperationEntry) (Operation, error) {

	var (
		op  Operation
		err error
	)
	switch p.Type {
	// file volume operations
	case OperationCreateVolume:
		op, err = loadVolumeCreateOperation(db, p)
	case OperationDeleteVolume:
		op, err = loadVolumeDeleteOperation(db, p)
	case OperationExpandVolume:
		op, err = loadVolumeExpandOperation(db, p)
	// block volume operations
	case OperationCreateBlockVolume:
		op, err = loadBlockVolumeCreateOperation(db, p)
	case OperationDeleteBlockVolume:
		op, err = loadBlockVolumeDeleteOperation(db, p)
	default:
		err = NewErrNotLoadable(p.Id, p.Type)
	}
	return op, err
}

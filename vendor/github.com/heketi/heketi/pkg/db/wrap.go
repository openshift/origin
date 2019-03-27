//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package db

import (
	"errors"

	"github.com/boltdb/bolt"
)

// RODB provides an abstraction for all types of db connection that
// support Read-Only transactions.
type RODB interface {
	View(func(*bolt.Tx) error) error
}

// DB provides an abstraction for all types of db connection that
// support both read and write transactions.
type DB interface {
	RODB
	Update(func(*bolt.Tx) error) error
}

// DBWrap encapsulates a bolt DB object in a minimal interface
// with additional run-time hooks and checks.
type DBWrap struct {
	db       *bolt.DB
	readOnly bool
}

// NewDBWrap creates a new DBWrap that is defaulted to read/write mode.
func NewDBWrap(db *bolt.DB) *DBWrap {
	return &DBWrap{db, false}
}

// View wraps a read-only transaction.
func (w *DBWrap) View(cb func(*bolt.Tx) error) error {
	return w.db.View(func(tx *bolt.Tx) error {
		return cb(tx)
	})
}

// Update wraps a read-write transaction.
// If Update is called on a read-only DBWrap it panics.
func (w *DBWrap) Update(cb func(*bolt.Tx) error) error {
	if w.readOnly {
		panic(errors.New("Can not update a read-only DBWrap"))
	}
	return w.db.Update(func(tx *bolt.Tx) error {
		return cb(tx)
	})
}

// ReadOnly returns a new DBWrap object, based on the same connection as
// the current object, set for read-only mode.
func (w *DBWrap) ReadOnly() *DBWrap {
	return &DBWrap{w.db, true}
}

// TxWrap encapsulates a bolt DB Tx object in a wrapper that implements
// the RODB and DB interfaces. This is useful when defining a function
// that can be used inside a transcation or start a transcation.
type TxWrap struct {
	tx       *bolt.Tx
	readOnly bool
}

// View fakes a read-only transaction. The function signature of a read-only
// transaction is supported but no new transaction is started.
func (w *TxWrap) View(cb func(*bolt.Tx) error) error {
	return cb(w.tx)
}

// Update fakes a read-write transaction. The function signature of a read-write
// transaction is supported but no new transaction is started.
func (w *TxWrap) Update(cb func(*bolt.Tx) error) error {
	if w.readOnly {
		panic(errors.New("Can not update a read-only TxWrap"))
	}
	return cb(w.tx)
}

// WrapReadWrite wraps a db object when applicable. If db is
// already in a capsule the original object is returned (type cast).
// Panics if the type is not valid for read-write encapsulation.
func WrapReadWrite(db DB) *DBWrap {
	switch db.(type) {
	case *bolt.DB:
		return NewDBWrap(db.(*bolt.DB))
	case *DBWrap:
		w := db.(*DBWrap)
		if w.readOnly {
			panic(errors.New("read-only DBWrap may not be wrapped as read-write"))
		}
		return w
	default:
		panic(errors.New("type can not be wrapped in a db capsule"))
	}
}

// WrapReadOnly wraps a db object when applicable. If db is
// already in a capsule the original object is returned (type cast).
// Panics if the type is not valid for read-only encapsulation.
func WrapReadOnly(db DB) *DBWrap {
	switch db.(type) {
	case *bolt.DB:
		return NewDBWrap(db.(*bolt.DB)).ReadOnly()
	case *DBWrap:
		w := db.(*DBWrap)
		if w.readOnly {
			return w
		}
		return w.ReadOnly()
	default:
		panic(errors.New("type can not be wrapped in a read only db capsule"))
	}
}

// WrapTx takes a bolt db transaction object and return a TxWrap object.
// This new object can now be used where the DB or RODB interfaces are
// used without starting a new transaction.
func WrapTx(tx *bolt.Tx) *TxWrap {
	return &TxWrap{tx, false}
}

// WrapTx takes a bolt db transaction object and return a TxWrap object.
// This new object can now be used where the DB or RODB interfaces are
// used without starting a new transaction.
// This wrapper will panic is .Update is called at runtime.
func WrapTxReadOnly(tx *bolt.Tx) *TxWrap {
	return &TxWrap{tx, true}
}

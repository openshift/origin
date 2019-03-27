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
	"os"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestMinimalDBWrap(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := NewDBWrap(db)
	tests.Assert(t, !w.readOnly, "expected w.readOnly to be false, got:", w.readOnly)

	wasCalled := false
	w.View(func(tx *bolt.Tx) error {
		wasCalled = true
		return nil
	})
	tests.Assert(t, wasCalled, "expected wasCalled to be true, got:", wasCalled)

	wasCalled = false
	w.Update(func(tx *bolt.Tx) error {
		wasCalled = true
		return nil
	})
	tests.Assert(t, wasCalled, "expected wasCalled to be true, got:", wasCalled)
}

func TestDBWrapping(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := WrapReadWrite(db)
	tests.Assert(t, !w.readOnly, "expected w.readOnly to be false, got:", w.readOnly)

	w2 := WrapReadWrite(w)
	tests.Assert(t, !w2.readOnly, "expected w.readOnly to be false, got:", w.readOnly)

	tests.Assert(t, w == w2, "expected w == w2, got:", w, w2)
}

func TestDBWrapToReadOnly(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := WrapReadWrite(db)
	tests.Assert(t, !w.readOnly, "expected w.readOnly to be false, got:", w.readOnly)

	w2 := w.ReadOnly()
	tests.Assert(t, w2.readOnly, "expected w2.readOnly to be true, got:", w2.readOnly)
}

func TestDBWrappingReadOnly(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := WrapReadOnly(db)
	tests.Assert(t, w.readOnly, "expected w.readOnly to be true, got:", w.readOnly)

	w2 := WrapReadOnly(w)
	tests.Assert(t, w2.readOnly, "expected w.readOnly to be true, got:", w.readOnly)

	tests.Assert(t, w == w2, "expected w == w2, got:", w, w2)
}

func TestDBWrappingConvertToReadOnly(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := WrapReadWrite(db)
	tests.Assert(t, !w.readOnly, "expected w.readOnly to be false, got:", !w.readOnly)

	w2 := WrapReadOnly(w)
	tests.Assert(t, w2.readOnly, "expected w.readOnly to be true, got:", w.readOnly)

	tests.Assert(t, w != w2, "expected w != w2, got:", w, w2)
}

func TestDBWrappingInvalidConverToReadWrite(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := WrapReadOnly(db)
	tests.Assert(t, w.readOnly, "expected w.readOnly to be true, got:", w.readOnly)

	defer func() {
		e := recover()
		tests.Assert(t, e != nil, "expected e != nil, got", e)
	}()
	WrapReadWrite(w)
	t.Fatalf("should not reach this line")
}

func TestDBWrapPanicOnUpdateReadOnly(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	w := WrapReadOnly(db)
	tests.Assert(t, w.readOnly, "expected w.readOnly to be true, got:", w.readOnly)

	defer func() {
		e := recover()
		tests.Assert(t, e != nil, "expected e != nil, got", e)
	}()
	w.Update(func(tx *bolt.Tx) error {
		t.Fatalf("should never get called")
		return nil
	})
	t.Fatalf("should not reach this line")
}

type junkDB struct {
	Foo int
}

func (j junkDB) View(func(tx *bolt.Tx) error) error {
	return nil
}

func (j junkDB) Update(func(tx *bolt.Tx) error) error {
	return nil
}

func TestDBWrappingBadType(t *testing.T) {

	defer func() {
		e := recover()
		tests.Assert(t, e != nil, "expected e != nil, got", e)
	}()

	WrapReadWrite(junkDB{})
	t.Fatalf("should not reach this line")
}

func TestDBWrappingROBadType(t *testing.T) {

	defer func() {
		e := recover()
		tests.Assert(t, e != nil, "expected e != nil, got", e)
	}()

	WrapReadOnly(junkDB{})
	t.Fatalf("should not reach this line")
}

func TestTxWrapNestView(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	stuff := 0
	// f mimics a standalone function that wants a DB object but we want
	// to use it within an existing transaction
	f := func(db RODB) error {
		stuff++
		db.View(func(tx *bolt.Tx) error {
			stuff++
			return nil
		})
		stuff++
		return nil
	}

	db.View(func(tx *bolt.Tx) error {
		stuff++
		f(WrapTxReadOnly(tx))
		stuff++
		return nil
	})

	tests.Assert(t, stuff == 5, "expected stuff == 5, got:", 5)
}

func TestTxWrapNestUpdate(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	stuff := 0
	// f mimics a standalone function that wants a DB object but we want
	// to use it within an existing transaction
	f := func(db DB) error {
		stuff++
		db.Update(func(tx *bolt.Tx) error {
			stuff++
			return nil
		})
		stuff++
		return nil
	}

	db.Update(func(tx *bolt.Tx) error {
		stuff++
		f(WrapTx(tx))
		stuff++
		return nil
	})

	tests.Assert(t, stuff == 5, "expected stuff == 5, got:", 5)
}

func TestTxWrapFailUpdateOnRO(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create a db
	db, err := bolt.Open(tmpfile, 0600, &bolt.Options{Timeout: 3 * time.Second})
	tests.Assert(t, err == nil, "expected (bolt.Open) err == nil, got:", err)
	tests.Assert(t, db != nil, "expected (bolt.Open) db != nil, got:", err)

	stuff := 0
	// f mimics a standalone function that wants a DB object but we want
	// to use it within an existing transaction
	f := func(db DB) error {
		stuff++
		db.Update(func(tx *bolt.Tx) error {
			stuff++
			return nil
		})
		stuff++
		return nil
	}

	defer func() {
		e := recover()
		tests.Assert(t, e != nil, "expected e != nil, got", e)
		tests.Assert(t, stuff == 2, "expected stuff == 2, got:", stuff)
	}()

	// generally you would want to correctly use DB or RODB so that the type
	// system can catch errors. However we have extra runtime guards against
	// calling a write method on a R/O TxWrap.
	db.Update(func(tx *bolt.Tx) error {
		stuff++
		f(WrapTxReadOnly(tx))
		stuff++
		return nil
	})

	t.Fatalf("should not reach this line")
}

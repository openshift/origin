//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"os"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestMapDbAtrributeKeys(t *testing.T) {
	x := mapDbAtrributeKeys()
	tests.Assert(t, len(x) == len(dbAttributeKeys), "different lengths")
	for _, k := range dbAttributeKeys {
		tests.Assert(t, x[k], k, "missing from", x)
	}
}

func TestValidDbAttributeKeys(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)

	// test normal db attributes (for this version)
	app.db.View(func(tx *bolt.Tx) error {
		v := validDbAttributeKeys(tx, mapDbAtrributeKeys())
		tests.Assert(t, v, "expected db attributes valid")
		return nil
	})

	// add some fake known attributes. This is still valid
	// if the db lacks keys we know about. DB is probably just old.
	app.db.View(func(tx *bolt.Tx) error {
		m := mapDbAtrributeKeys()
		m["LOVELY_WATER"] = true
		m["THAT_SINKING_FEELING"] = true
		v := validDbAttributeKeys(tx, m)
		tests.Assert(t, v, "expected db attributes valid")
		return nil
	})

	app.db.Update(func(tx *bolt.Tx) error {
		entry := NewDbAttributeEntry()
		entry.Key = "LOVELY_FISH"
		entry.Value = "no"
		err := entry.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		v := validDbAttributeKeys(tx, mapDbAtrributeKeys())
		tests.Assert(t, !v, "expected db attributes not valid")
		return nil
	})
}

// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"bytes"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestSingleResult(t *testing.T) {
	t.Run("TestDecode", func(t *testing.T) {
		t.Run("DecodeTwice", func(t *testing.T) {
			// Test that Decode and DecodeBytes can be called more than once
			c, err := newCursor(newTestBatchCursor(1, 1), bson.DefaultRegistry)
			if err != nil {
				t.Fatalf("error creating cursor: %v", err)
			}

			sr := &SingleResult{cur: c, reg: bson.DefaultRegistry}
			var firstDecode, secondDecode bson.Raw
			if err = sr.Decode(&firstDecode); err != nil {
				t.Fatalf("error on first Decode call: %v", err)
			}
			if err = sr.Decode(&secondDecode); err != nil {
				t.Fatalf("error on second Decode call: %v", err)
			}
			decodeBytes, err := sr.DecodeBytes()
			if err != nil {
				t.Fatalf("error on DecodeBytes call: %v", err)
			}

			if !bytes.Equal(firstDecode, secondDecode) {
				t.Fatalf("Decode contents do not match; first returned %v, second returned %v",
					firstDecode, secondDecode)
			}
			if !bytes.Equal(firstDecode, decodeBytes) {
				t.Fatalf("Decode and DecodeBytes contents do not match; Decode returned %v, DecodeBytes "+
					"returned %v", firstDecode, decodeBytes)
			}
		})
	})

	t.Run("TestErr", func(t *testing.T) {
		sr := &SingleResult{}
		err := sr.Err()
		if err != ErrNoDocuments {
			t.Fatalf("Error returned by SingleResult.Err() was %v when ErrNoDocuments was expected", err)
		}
	})

	t.Run("TestDecodeWithErr", func(t *testing.T) {
		r := []byte("foo")
		sr := &SingleResult{rdr: r, err: errors.New("DecodeBytes error")}
		res, err := sr.DecodeBytes()
		if !bytes.Equal(res, r) {
			t.Fatalf("DecodeBytes contents do not match; expected %v, returned %v", res, r)
		}
		if err != sr.err {
			t.Fatalf("Error returned by DecodeBytes was %v when %v was expected", err, sr.err)
		}
	})
}

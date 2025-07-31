// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"bytes"
)

// DeepCopyInto creates a deep-copy of src by encoding it to JSON and then
// decoding that into dst.
// Please note, empty slices or maps in src that are set to omitempty will be
// nil in the copied object.
func DeepCopyInto[T AnyType](dst *T, src T) error {
	var w bytes.Buffer
	e := NewJSONEncoder(&w)
	if err := e.Encode(src); err != nil {
		return err
	}
	d := NewJSONDecoder(&w)
	if err := d.Decode(dst); err != nil {
		return err
	}
	return nil
}

// MustDeepCopyInto panics if DeepCopyInto returns an error.
func MustDeepCopyInto[T AnyType](dst *T, src T) error {
	if err := DeepCopyInto(dst, src); err != nil {
		panic(err)
	}
	return nil
}

// DeepCopy creates a deep-copy of src by encoding it to JSON and then decoding
// that into a new instance of T.
// Please note, empty slices or maps in src that are set to omitempty will be
// nil in the copied object.
func DeepCopy[T AnyType](src T) (T, error) {
	var dst T
	err := DeepCopyInto(&dst, src)
	return dst, err
}

// MustDeepCopy panics if DeepCopy returns an error.
func MustDeepCopy[T AnyType](src T) T {
	dst, err := DeepCopy(src)
	if err != nil {
		panic(err)
	}
	return dst
}

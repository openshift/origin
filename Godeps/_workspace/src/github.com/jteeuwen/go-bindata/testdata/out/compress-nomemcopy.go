package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"reflect"
	"unsafe"
)

func bindata_read(data, name string) []byte {
	var empty [0]byte
	sx := (*reflect.StringHeader)(unsafe.Pointer(&data))
	b := empty[:]
	bx := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bx.Data = sx.Data
	bx.Len = len(data)
	bx.Cap = bx.Len

	gz, err := gzip.NewReader(bytes.NewBuffer(b))
	if err != nil {
		log.Fatalf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	gz.Close()

	if err != nil {
		log.Fatalf("Read %q: %v", name, err)
	}

	return buf.Bytes()
}

var _in_b_test_asset = "\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\xd2\xd7\x57\x28\x4e\xcc\x2d\xc8\x49\x55\x48\xcb\xcc\x49\xe5\x02\x04\x00\x00\xff\xff\x8a\x82\x8c\x85\x0f\x00\x00\x00"

func in_b_test_asset() []byte {
	return bindata_read(
		_in_b_test_asset,
		"in/b/test.asset",
	)
}

var _in_test_asset = "\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\xd2\xd7\x57\x28\x4e\xcc\x2d\xc8\x49\x55\x48\xcb\xcc\x49\xe5\x02\x04\x00\x00\xff\xff\x8a\x82\x8c\x85\x0f\x00\x00\x00"

func in_test_asset() []byte {
	return bindata_read(
		_in_test_asset,
		"in/test.asset",
	)
}

var _in_a_test_asset = "\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\xd2\xd7\x57\x28\x4e\xcc\x2d\xc8\x49\x55\x48\xcb\xcc\x49\xe5\x02\x04\x00\x00\xff\xff\x8a\x82\x8c\x85\x0f\x00\x00\x00"

func in_a_test_asset() []byte {
	return bindata_read(
		_in_a_test_asset,
		"in/a/test.asset",
	)
}

var _in_c_test_asset = "\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\xd2\xd7\x57\x28\x4e\xcc\x2d\xc8\x49\x55\x48\xcb\xcc\x49\xe5\x02\x04\x00\x00\xff\xff\x8a\x82\x8c\x85\x0f\x00\x00\x00"

func in_c_test_asset() []byte {
	return bindata_read(
		_in_c_test_asset,
		"in/c/test.asset",
	)
}

// Asset loads and returns the asset for the given name.
// This returns nil of the asset could not be found.
func Asset(name string) []byte {
	if f, ok := _bindata[name]; ok {
		return f()
	}
	return nil
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() []byte{
	"in/b/test.asset": in_b_test_asset,
	"in/test.asset":   in_test_asset,
	"in/a/test.asset": in_a_test_asset,
	"in/c/test.asset": in_c_test_asset,
}

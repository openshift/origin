package main

import (
	"bytes"
	"io"
	"log"
	"os"
)

// bindata_read reads the given file from disk.
// It panics if anything went wrong.
func bindata_read(path, name string) []byte {
	fd, err := os.Open(path)
	if err != nil {
		log.Fatalf("Read %s: %v", name, err)
	}

	defer fd.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, fd)
	if err != nil {
		log.Fatalf("Read %s: %v", name, err)
	}

	return buf.Bytes()
}

// in_b_test_asset reads file data from disk.
// It panics if something went wrong in the process.
func in_b_test_asset() []byte {
	return bindata_read(
		"/a/code/go/src/github.com/jteeuwen/go-bindata/testdata/in/b/test.asset",
		"in/b/test.asset",
	)
}

// in_test_asset reads file data from disk.
// It panics if something went wrong in the process.
func in_test_asset() []byte {
	return bindata_read(
		"/a/code/go/src/github.com/jteeuwen/go-bindata/testdata/in/test.asset",
		"in/test.asset",
	)
}

// in_a_test_asset reads file data from disk.
// It panics if something went wrong in the process.
func in_a_test_asset() []byte {
	return bindata_read(
		"/a/code/go/src/github.com/jteeuwen/go-bindata/testdata/in/a/test.asset",
		"in/a/test.asset",
	)
}

// in_c_test_asset reads file data from disk.
// It panics if something went wrong in the process.
func in_c_test_asset() []byte {
	return bindata_read(
		"/a/code/go/src/github.com/jteeuwen/go-bindata/testdata/in/c/test.asset",
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

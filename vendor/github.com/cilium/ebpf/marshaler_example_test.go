package ebpf

import (
	"encoding"
	"fmt"
	"strings"
)

// Assert that customEncoding implements the correct interfaces.
var (
	_ encoding.BinaryMarshaler   = (*customEncoding)(nil)
	_ encoding.BinaryUnmarshaler = (*customEncoding)(nil)
)

type customEncoding struct {
	data string
}

func (ce *customEncoding) MarshalBinary() ([]byte, error) {
	return []byte(strings.ToUpper(ce.data)), nil
}

func (ce *customEncoding) UnmarshalBinary(buf []byte) error {
	ce.data = string(buf)
	return nil
}

// ExampleMarshaler shows how to use custom encoding with map methods.
func Example_customMarshaler() {
	hash := createHash()
	defer hash.Close()

	if err := hash.Put(&customEncoding{"hello"}, uint32(111)); err != nil {
		panic(err)
	}

	var (
		key     customEncoding
		value   uint32
		entries = hash.Iterate()
	)

	for entries.Next(&key, &value) {
		fmt.Printf("key: %s, value: %d\n", key.data, value)
	}

	if err := entries.Err(); err != nil {
		panic(err)
	}

	// Output: key: HELLO, value: 111
}

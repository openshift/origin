package btf

import (
	"bytes"
	"strings"
	"testing"
)

func TestStringTable(t *testing.T) {
	const in = "\x00one\x00two\x00"

	st, err := readStringTable(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal([]byte(in), []byte(st)) {
		t.Error("String table doesn't match input")
	}

	testcases := []struct {
		offset uint32
		want   string
	}{
		{0, ""},
		{1, "one"},
		{5, "two"},
	}

	for _, tc := range testcases {
		have, err := st.Lookup(tc.offset)
		if err != nil {
			t.Errorf("Offset %d: %s", tc.offset, err)
			continue
		}

		if have != tc.want {
			t.Errorf("Offset %d: want %s but have %s", tc.offset, tc.want, have)
		}
	}

	if _, err := st.Lookup(2); err == nil {
		t.Error("No error when using offset pointing into middle of string")
	}

	// Make sure we reject bogus tables
	_, err = readStringTable(strings.NewReader("\x00one"))
	if err == nil {
		t.Fatal("Accepted non-terminated string")
	}

	_, err = readStringTable(strings.NewReader("one\x00"))
	if err == nil {
		t.Fatal("Accepted non-empty first item")
	}
}

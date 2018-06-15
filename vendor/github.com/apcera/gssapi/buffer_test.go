// Copyright 2013-2015 Apcera Inc. All rights reserved.

package gssapi

import (
	"bytes"
	"testing"
)

func TestNewBuffer(t *testing.T) {
	l, err := testLoad()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Unload()

	for a := range []int{allocNone, allocMalloc, allocGSSAPI} {
		b, err := l.MakeBuffer(a)
		if err != nil {
			t.Fatalf("alloc: %v: %s", a, err)
		}
		defer b.Release()

		if b == nil {
			t.Fatalf("alloc: %v: Got nil, expected non-nil", a)
		}
		if b.Lib != l {
			t.Fatalf("alloc: %v: b.Lib didn't get set correctly, got %p, expected %p",
				a, b.Lib, l)
		}
		if b.C_gss_buffer_t == nil {
			t.Fatalf("alloc: %v: Got nil buffer, expected non-nil", a)
		}
		if b.String() != "" {
			t.Fatalf(`alloc: %v: String(): got %q, expected ""`,
				a, b.String())
		}
	}
}

// Also tests MakeBufferBytes, implicitly
func TestMakeBufferString(t *testing.T) {
	l, err := testLoad()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Unload()

	test := "testing"
	b, err := l.MakeBufferString(test)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()

	if b == nil {
		t.Fatal("Got nil, expected non-nil")
	}
	if b.Lib != l {
		t.Fatalf("b.Lib didn't get set correctly, got %p, expected %p", b.Lib, l)
	}
	if b.String() != test {
		t.Fatalf("Got %q, expected %q", b.String(), test)
	} else if !bytes.Equal(b.Bytes(), []byte(test)) {
		t.Fatalf("Got '%v'; expected '%v'", b.Bytes(), []byte(test))
	}
}

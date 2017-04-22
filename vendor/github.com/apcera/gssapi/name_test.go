// Copyright 2014 Apcera Inc. All rights reserved.

package gssapi

import (
	"testing"
)

// Tests importing exporting names
func TestNameImportExport(t *testing.T) {
	l, err := testLoad()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Unload()

	names := []string{
		`test@corp.example.com`,
		`test@corp.ExAmple.com`,
		`test@CORP.EXAMPLE.COM`,
	}

	makeName := func(n string) (name *Name) {
		b, _ := l.MakeBufferString(n)
		if err != nil {
			t.Fatalf("%q: Got error %v, expected nil", n, err)
		}
		if b == nil {
			t.Fatalf("%q: Got nil, expected non-nil", n)
		}
		defer b.Release()

		name, err := b.Name(l.GSS_C_NT_HOSTBASED_SERVICE)
		if err != nil {
			t.Fatalf("%q: Got error %v, expected nil", n, err)
		}
		if name == nil {
			t.Fatalf("%q: Got nil, expected non-nil", n)
		}
		return name
	}

	// Make the reference name
	n0 := makeName(names[0])
	defer n0.Release()

	// Make sure we can have the krb mechanism, and normalize the reference
	// name using it
	mechs, err := n0.InquireMechs()
	if err != nil {
		//TODO: need a better test for OS X since this InquireMechs doesn't
		// seem to work
		t.Skipf("Couldn't get mechs for %q, error: %v", names[0], err.Error())
	}

	// This OID seems to be an avalable merch on linux
	kerbOID, err := l.MakeOIDBytes([]byte{'\x2a', '\x86', '\x48', '\x86', '\xf7', '\x12', '\x01', '\x02', '\x02'})
	if err != nil {
		t.Fatalf("Got error %v, expected nil", err)
	}
	defer kerbOID.Release()

	if !mechs.Contains(kerbOID) {
		t.Fatalf("Expected %q to be in %q", kerbOID.DebugString(), mechs.DebugString())
	}

	makeNames := func(n string) (
		name *Name, canonical *Name, display string, exported *Buffer) {

		name = makeName(n)
		if name == nil {
			return nil, nil, "", nil
		}

		origDisplay, _, err := name.Display()
		if err != nil {
			t.Fatalf("Got error %q, expected nil", err.Error())
		}
		if origDisplay != n {
			t.Fatalf("Got %q, expected %q", origDisplay, n)
		}

		canonical, err = name.Canonicalize(kerbOID)
		if err != nil {
			t.Fatalf("Got error %q, expected nil", err.Error())
		}
		if canonical == nil {
			t.Fatal("Got nil, expected non-nil")
		}

		display, _, err = canonical.Display()
		if err != nil {
			t.Fatalf("Got error %q, expected nil", err.Error())
		}

		exported, err = canonical.Export()
		if err != nil {
			t.Fatalf("Got error %q, expected nil", err.Error())
		}
		if exported == nil {
			t.Fatal("Got nil, expected non-nil")
		}

		return name, canonical, display, exported
	}

	n0, _, d0, e0 := makeNames(names[0])
	if n0 == nil {
		t.Fatal("Got nil, expected non-nil")
	}

	for _, n := range names {
		n, _, d, e := makeNames(n)
		if n == nil {
			t.Fatalf("%s: Got nil, expected non-nil", n)
		}
		if d != d0 {
			t.Fatalf("%s: Got %q, expected %q", n, d, d0)
		}
		if !e.Equal(e0) {
			t.Fatalf("%s: Got %q, expected %q", n, e.String(), e0.String())
		}
	}
}

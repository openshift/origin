// +build !selinux

package selinux

import (
	"testing"
)

func TestSELinux(t *testing.T) {
	if GetEnabled() {
		t.Fatal("SELinux enabled with build-tag !selinux.")
	}

	if _, err := FileLabel("/etc"); err != nil {
		t.Fatal(err)
	}

	if err := SetFileLabel("/etc", "foobar"); err != nil {
		t.Fatal(err)
	}

	if err := SetFSCreateLabel("foobar"); err != nil {
		t.Fatal(err)
	}

	if _, err := FSCreateLabel(); err != nil {
		t.Fatal(err)
	}
	if _, err := CurrentLabel(); err != nil {
		t.Fatal(err)
	}

	if _, err := PidLabel(0); err != nil {
		t.Fatal(err)
	}

	ClearLabels()

	ReserveLabel("foobar")
	ReleaseLabel("foobar")
	DupSecOpt("foobar")
	DisableSecOpt()
	SetDisabled()
	if enabled := GetEnabled(); enabled {
		t.Fatal("Should not be enabled")
	}
	if err := SetExecLabel("foobar"); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecLabel(); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalizeContext("foobar"); err != nil {
		t.Fatal(err)
	}
	if err := SetSocketLabel("foobar"); err != nil {
		t.Fatal(err)
	}
	if _, err := SocketLabel(); err != nil {
		t.Fatal(err)
	}
	if err := SetKeyLabel("foobar"); err != nil {
		t.Fatal(err)
	}
	if _, err := KeyLabel(); err != nil {
		t.Fatal(err)
	}
	con, err := NewContext("foobar")
	if err != nil {
		t.Fatal(err)
	}
	con.Get()
	if err := SetEnforceMode(1); err != nil {
		t.Fatal(err)
	}
	DefaultEnforceMode()
	EnforceMode()
	ROFileLabel()
	ContainerLabels()
	if err := SecurityCheckContext("foobar"); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyLevel("foo", "bar"); err != nil {
		t.Fatal(err)
	}
}

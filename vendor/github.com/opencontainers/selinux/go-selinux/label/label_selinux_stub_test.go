// +build !selinux !linux

package label

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	var testNull []string
	_, _, err := InitLabels(testNull)
	if err != nil {
		t.Log("InitLabels Failed")
		t.Fatal(err)
	}
	testDisabled := []string{"disable"}
	roMountLabel := ROMountLabel()
	if roMountLabel != "" {
		t.Errorf("ROMountLabel Failed")
	}
	plabel, _, err := InitLabels(testDisabled)
	if err != nil {
		t.Log("InitLabels Disabled Failed")
		t.Fatal(err)
	}
	if plabel != "" {
		t.Log("InitLabels Disabled Failed")
		t.FailNow()
	}
	testUser := []string{"user:user_u", "role:user_r", "type:user_t", "level:s0:c1,c15"}
	plabel, _, err = InitLabels(testUser)
	if err != nil {
		t.Log("InitLabels User Failed")
		t.Fatal(err)
	}
}

func TestRelabel(t *testing.T) {
	testdir, err := ioutil.TempDir("/tmp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testdir)
	label := "system_u:object_r:container_file_t:s0:c1,c2"
	if err := Relabel("/etc", label, false); err != nil {
		t.Fatalf("Relabel /etc succeeded")
	}
}

func TestSocketLabel(t *testing.T) {
	label := "system_u:object_r:container_t:s0:c1,c2"
	if err := SetSocketLabel(label); err != nil {
		t.Fatal(err)
	}
	if _, err := SocketLabel(); err != nil {
		t.Fatal(err)
	}
}

func TestKeyLabel(t *testing.T) {
	label := "system_u:object_r:container_t:s0:c1,c2"
	if err := SetKeyLabel(label); err != nil {
		t.Fatal(err)
	}
	if _, err := KeyLabel(); err != nil {
		t.Fatal(err)
	}
}

func TestProcessLabel(t *testing.T) {
	label := "system_u:object_r:container_t:s0:c1,c2"
	if err := SetProcessLabel(label); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessLabel(); err != nil {
		t.Fatal(err)
	}
}

func CheckLabelCompile(t *testing.T) {
	if _, _, err := GenLabels(""); err != nil {
		t.Fatal(err)
	}
	if test := FormatMountLabel("", ""); test != "" {
		t.Fatal("Format failed")
	}

	if test := FormatMountLabel("", ""); test != "" {
		t.Fatal("Format failed")
	}

	if _, err := FileLabel("/etc"); err != nil {
		t.Fatal(err)
	}

	if err := SetFileLabel("/etc", "foobar"); err != nil {
		t.Fatal(err)
	}

	if err := SetFileCreateLabel("foobar"); err != nil {
		t.Fatal(err)
	}

	if _, err := PidLabel(0); err != nil {
		t.Fatal(err)
	}

	ClearLabels()

	if err := ReserveLabel("foobar"); err != nil {
		t.Fatal(err)
	}

	if err := ReleaseLabel("foobar"); err != nil {
		t.Fatal(err)
	}

	DupSecOpt("foobar")
	DisableSecOpt()

	if err := Validate("foobar"); err != nil {
		t.Fatal(err)
	}
	if relabel := RelabelNeeded("foobar"); relabel {
		t.Fatal("Relabel failed")
	}
	if shared := IsShared("foobar"); shared {
		t.Fatal("isshared failed")
	}
}

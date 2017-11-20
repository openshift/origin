package fs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	testfs "github.com/openshift/source-to-image/pkg/test/fs"
)

func TestCopy(t *testing.T) {
	sep := string(filepath.Separator)

	// test plain file copy
	fake := &testfs.FakeFileSystem{
		Files: []os.FileInfo{
			&FileInfo{FileName: "file", FileMode: 0600},
		},
		OpenContent: "test",
	}
	err := doCopy(fake, sep+"file", sep+"dest")
	if err != nil {
		t.Error(err)
	}
	if fake.CreateFile != sep+"dest" {
		t.Error(fake.CreateFile)
	}
	if fake.CreateContent.String() != "test" {
		t.Error(fake.CreateContent.String())
	}
	if !reflect.DeepEqual(fake.ChmodFile, []string{sep + "dest"}) {
		t.Error(fake.ChmodFile)
	}
	if fake.ChmodMode != 0600 {
		t.Error(fake.ChmodMode)
	}

	// test broken symlink copy
	fake = &testfs.FakeFileSystem{
		Files: []os.FileInfo{
			&FileInfo{FileName: "link", FileMode: os.ModeSymlink},
		},
		ReadlinkName: sep + "linkdest",
	}
	err = doCopy(fake, sep+"link", sep+"dest")
	if err != nil {
		t.Error(err)
	}
	if fake.SymlinkNewname != sep+"dest" {
		t.Error(fake.SymlinkNewname)
	}
	if fake.SymlinkOldname != sep+"linkdest" {
		t.Error(fake.SymlinkOldname)
	}

	// test non-broken symlink copy
	fake = &testfs.FakeFileSystem{
		Files: []os.FileInfo{
			&FileInfo{FileName: "file", FileMode: 0600},
			&FileInfo{FileName: "link", FileMode: os.ModeSymlink},
		},
		OpenContent:  "test",
		ReadlinkName: sep + "file",
	}
	err = doCopy(fake, sep+"link", sep+"dest")
	if fake.CreateFile != sep+"dest" {
		t.Error(fake.CreateFile)
	}
	if fake.CreateContent.String() != "test" {
		t.Error(fake.CreateContent.String())
	}
	if !reflect.DeepEqual(fake.ChmodFile, []string{sep + "dest"}) {
		t.Error(fake.ChmodFile)
	}
	if fake.ChmodMode != 0600 {
		t.Error(fake.ChmodMode)
	}
}

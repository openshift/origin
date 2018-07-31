package fs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	testfs "github.com/openshift/source-to-image/pkg/test/fs"
)

func helper(t *testing.T, keepSymlinks bool) {
	sep := string(filepath.Separator)

	// test plain file copy
	fake := &testfs.FakeFileSystem{
		Files: []os.FileInfo{
			&FileInfo{FileName: "file", FileMode: 0600},
		},
		OpenContent: "test",
	}
	fake.KeepSymlinks(keepSymlinks)
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
	fake.KeepSymlinks(keepSymlinks)
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
	fake.KeepSymlinks(keepSymlinks)
	err = doCopy(fake, sep+"link", sep+"dest")
	if err != nil {
		t.Error(err)
	}
	if keepSymlinks {
		if fake.SymlinkNewname != sep+"dest" {
			t.Error(fake.SymlinkNewname)
		}
	} else {
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
}

func TestCopy(t *testing.T) {
	helper(t, false)
}

func TestCopyKeepSymlinks(t *testing.T) {
	helper(t, true)
}

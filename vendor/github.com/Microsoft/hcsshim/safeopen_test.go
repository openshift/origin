package hcsshim

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	winio "github.com/Microsoft/go-winio"
)

func tempRoot() (*os.File, error) {
	name, err := ioutil.TempDir("", "hcsshim-test")
	if err != nil {
		return nil, err
	}
	f, err := openRoot(name)
	if err != nil {
		os.Remove(name)
		return nil, err
	}
	return f, nil
}

func TestOpenRelative(t *testing.T) {
	badroot, err := tempRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(badroot.Name())
	defer badroot.Close()

	root, err := tempRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root.Name())
	defer root.Close()

	// Create a file
	f, err := openRelative("foo", root, 0, syscall.FILE_SHARE_READ, _FILE_CREATE, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create a directory
	err = mkdirRelative("dir", root)
	if err != nil {
		t.Fatal(err)
	}

	// Create a file in the bad root
	f, err = os.Create(filepath.Join(badroot.Name(), "badfile"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create a directory symlink to the bad root
	err = os.Symlink(badroot.Name(), filepath.Join(root.Name(), "dsymlink"))
	if err != nil {
		t.Fatal(err)
	}

	// Create a file symlink to the bad file
	err = os.Symlink(filepath.Join(badroot.Name(), "badfile"), filepath.Join(root.Name(), "symlink"))
	if err != nil {
		t.Fatal(err)
	}

	// Make sure opens cannot happen through the symlink
	f, err = openRelative("dsymlink/foo", root, 0, syscall.FILE_SHARE_READ, _FILE_CREATE, 0)
	if err == nil {
		f.Close()
		t.Fatal("created file in wrong tree!")
	}
	t.Log(err)

	// Check again using ensureNotReparsePointRelative
	err = ensureNotReparsePointRelative("dsymlink", root)
	if err == nil {
		t.Fatal("reparse check should have failed")
	}
	t.Log(err)

	// Make sure links work
	err = linkRelative("foo", root, "hardlink", root)
	if err != nil {
		t.Fatal(err)
	}

	// Even inside directories
	err = linkRelative("foo", root, "dir/bar", root)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure links cannot happen through the symlink
	err = linkRelative("foo", root, "dsymlink/hardlink", root)
	if err == nil {
		f.Close()
		t.Fatal("created link in wrong tree!")
	}
	t.Log(err)

	// In either direction
	err = linkRelative("dsymlink/badfile", root, "bar", root)
	if err == nil {
		f.Close()
		t.Fatal("created link in wrong tree!")
	}
	t.Log(err)

	// Make sure remove cannot happen through the symlink
	err = removeRelative("symlink/badfile", root)
	if err == nil {
		t.Fatal("remove in wrong tree!")
	}

	// Remove the symlink
	err = removeAllRelative("symlink", root)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure it's not possible to escape with .. (NT doesn't support .. at the kernel level)
	f, err = openRelative("..", root, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, _FILE_OPEN, 0)
	if err == nil {
		t.Fatal("escaped the directory")
	}
	t.Log(err)

	// Should not have touched the other directory
	if _, err = os.Lstat(filepath.Join(badroot.Name(), "badfile")); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRelativeReadOnly(t *testing.T) {
	root, err := tempRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root.Name())
	defer root.Close()

	p := filepath.Join(root.Name(), "foo")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	bi := winio.FileBasicInfo{}
	bi.FileAttributes = syscall.FILE_ATTRIBUTE_READONLY
	err = winio.SetFileBasicInfo(f, &bi)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	err = removeRelative("foo", root)
	if err != nil {
		t.Fatal(err)
	}
}

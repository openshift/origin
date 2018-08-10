package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mndrix/tap-go"
	"github.com/opencontainers/runtime-tools/validation/util"
	"golang.org/x/sys/unix"
)

func checkReadonlyPaths(t *tap.T) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	readonlyDir := "readonly-dir"
	readonlySubDir := "readonly-subdir"
	readonlyFile := "readonly-file"

	readonlyDirTop := filepath.Join("/", readonlyDir)
	readonlyFileTop := filepath.Join("/", readonlyFile)

	readonlyDirSub := filepath.Join(readonlyDirTop, readonlySubDir)
	readonlyFileSub := filepath.Join(readonlyDirTop, readonlyFile)
	readonlyFileSubSub := filepath.Join(readonlyDirSub, readonlyFile)

	g.AddLinuxReadonlyPaths(readonlyDirTop)
	g.AddLinuxReadonlyPaths(readonlyFileTop)
	g.AddLinuxReadonlyPaths(readonlyDirSub)
	g.AddLinuxReadonlyPaths(readonlyFileSub)
	g.AddLinuxReadonlyPaths(readonlyFileSubSub)
	g.AddAnnotation("TestName", "check read-only paths")
	err = util.RuntimeInsideValidate(g, t, func(path string) error {
		testDir := filepath.Join(path, readonlyDirSub)
		err = os.MkdirAll(testDir, 0777)
		if err != nil {
			return err
		}
		// create a temp file to make testDir non-empty
		tmpfile, err := ioutil.TempFile(testDir, "tmp")
		if err != nil {
			return err
		}
		defer os.Remove(tmpfile.Name())

		// runtimetest cannot check the readability of empty files, so
		// write something.
		testSubSubFile := filepath.Join(path, readonlyFileSubSub)
		if err := ioutil.WriteFile(testSubSubFile, []byte("immutable"), 0777); err != nil {
			return err
		}

		testSubFile := filepath.Join(path, readonlyFileSub)
		if err := ioutil.WriteFile(testSubFile, []byte("immutable"), 0777); err != nil {
			return err
		}

		testFile := filepath.Join(path, readonlyFile)
		return ioutil.WriteFile(testFile, []byte("immutable"), 0777)
	})
	return err
}

func checkReadonlyRelPaths(t *tap.T) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	// Deliberately set a relative path to be read-only, and expect an error
	readonlyRelPath := "readonly-relpath"

	g.AddLinuxReadonlyPaths(readonlyRelPath)
	g.AddAnnotation("TestName", "check read-only relative paths")
	err = util.RuntimeInsideValidate(g, t, func(path string) error {
		testFile := filepath.Join(path, readonlyRelPath)
		if _, err := os.Stat(testFile); err != nil && os.IsNotExist(err) {
			return err
		}

		return nil
	})
	if err != nil {
		return nil
	}
	return fmt.Errorf("expected: err != nil, actual: err == nil")
}

func checkReadonlySymlinks(t *tap.T) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	// Deliberately create a read-only symlink that points an invalid file,
	// and expect an error.
	readonlySymlink := "/readonly-symlink"

	g.AddLinuxReadonlyPaths(readonlySymlink)
	g.AddAnnotation("TestName", "check read-only symlinks")
	err = util.RuntimeInsideValidate(g, t, func(path string) error {
		testFile := filepath.Join(path, readonlySymlink)
		// ln -s .. /readonly-symlink ; readlink -f /readonly-symlink; ls -L /readonly-symlink
		if err := os.Symlink("../readonly-symlink", testFile); err != nil {
			return err
		}
		rPath, errR := os.Readlink(testFile)
		if errR != nil {
			return errR
		}
		_, errS := os.Stat(rPath)
		if errS != nil && os.IsNotExist(errS) {
			return errS
		}

		return nil
	})
	if err != nil {
		return nil
	}
	return fmt.Errorf("expected: err != nil, actual: err == nil")
}

func checkReadonlyDeviceNodes(t *tap.T, mode uint32) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	readonlyDevice := "/readonly-device"

	g.AddLinuxReadonlyPaths(readonlyDevice)
	g.AddAnnotation("TestName", "check read-only device nodes")
	return util.RuntimeInsideValidate(g, t, func(path string) error {
		testFile := filepath.Join(path, readonlyDevice)

		if err := unix.Mknod(testFile, mode, 0); err != nil {
			return err
		}

		if _, err := os.Stat(testFile); err != nil && os.IsNotExist(err) {
			return err
		}

		return nil
	})
}

func main() {
	t := tap.New()
	t.Header(0)
	defer t.AutoPlan()

	if err := checkReadonlyPaths(t); err != nil {
		util.Fatal(err)
	}

	if err := checkReadonlyRelPaths(t); err != nil {
		util.Fatal(err)
	}

	if err := checkReadonlySymlinks(t); err != nil {
		util.Fatal(err)
	}

	// test creation of different type of devices, i.e. block device,
	// character device, and FIFO.
	modes := []uint32{
		unix.S_IFBLK | 0666,
		unix.S_IFCHR | 0666,
		unix.S_IFIFO | 0666,
	}

	for _, m := range modes {
		if err := checkReadonlyDeviceNodes(t, m); err != nil {
			util.Fatal(err)
		}
	}
}

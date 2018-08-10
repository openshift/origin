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

func checkMaskedPaths(t *tap.T) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	maskedDir := "masked-dir"
	maskedSubDir := "masked-subdir"
	maskedFile := "masked-file"

	maskedDirTop := filepath.Join("/", maskedDir)
	maskedFileTop := filepath.Join("/", maskedFile)

	maskedDirSub := filepath.Join(maskedDirTop, maskedSubDir)
	maskedFileSub := filepath.Join(maskedDirTop, maskedFile)
	maskedFileSubSub := filepath.Join(maskedDirSub, maskedFile)

	g.AddLinuxMaskedPaths(maskedDirTop)
	g.AddLinuxMaskedPaths(maskedFileTop)
	g.AddLinuxMaskedPaths(maskedDirSub)
	g.AddLinuxMaskedPaths(maskedFileSub)
	g.AddLinuxMaskedPaths(maskedFileSubSub)
	g.AddAnnotation("TestName", "check masked paths")
	err = util.RuntimeInsideValidate(g, t, func(path string) error {
		testDir := filepath.Join(path, maskedDirSub)
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
		testSubSubFile := filepath.Join(path, maskedFileSubSub)
		if err := ioutil.WriteFile(testSubSubFile, []byte("secrets"), 0777); err != nil {
			return err
		}

		testSubFile := filepath.Join(path, maskedFileSub)
		if err := ioutil.WriteFile(testSubFile, []byte("secrets"), 0777); err != nil {
			return err
		}

		testFile := filepath.Join(path, maskedFile)
		return ioutil.WriteFile(testFile, []byte("secrets"), 0777)
	})
	return err
}

func checkMaskedRelPaths(t *tap.T) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	// Deliberately set a relative path to be masked, and expect an error
	maskedRelPath := "masked-relpath"

	g.AddLinuxMaskedPaths(maskedRelPath)
	g.AddAnnotation("TestName", "check masked relative paths")
	err = util.RuntimeInsideValidate(g, t, func(path string) error {
		testFile := filepath.Join(path, maskedRelPath)
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

func checkMaskedSymlinks(t *tap.T) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	// Deliberately create a masked symlink that points an invalid file,
	// and expect an error.
	maskedSymlink := "/masked-symlink"

	g.AddLinuxMaskedPaths(maskedSymlink)
	g.AddAnnotation("TestName", "check masked symlinks")
	err = util.RuntimeInsideValidate(g, t, func(path string) error {
		testFile := filepath.Join(path, maskedSymlink)
		// ln -s .. /masked-symlink ; readlink -f /masked-symlink; ls -L /masked-symlink
		if err := os.Symlink("../masked-symlink", testFile); err != nil {
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

func checkMaskedDeviceNodes(t *tap.T, mode uint32) error {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		return err
	}

	maskedDevice := "/masked-device"

	g.AddLinuxMaskedPaths(maskedDevice)
	g.AddAnnotation("TestName", "check masked device nodes")
	return util.RuntimeInsideValidate(g, t, func(path string) error {
		testFile := filepath.Join(path, maskedDevice)

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

	if err := checkMaskedPaths(t); err != nil {
		util.Fatal(err)
	}

	if err := checkMaskedRelPaths(t); err != nil {
		util.Fatal(err)
	}

	if err := checkMaskedSymlinks(t); err != nil {
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
		if err := checkMaskedDeviceNodes(t, m); err != nil {
			util.Fatal(err)
		}
	}
}

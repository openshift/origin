// +build functional uvmscsi

package functional

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Microsoft/hcsshim/internal/wclayer"

	"github.com/Microsoft/hcsshim/internal/lcow"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/test/functional/utilities"
	"github.com/sirupsen/logrus"
)

// TestSCSIAddRemovev2LCOW validates adding and removing SCSI disks
// from a utility VM in both attach-only and with a container path. Also does
// negative testing so that a disk can't be attached twice.
func TestSCSIAddRemoveLCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	u := testutilities.CreateLCOWUVM(t, t.Name())
	defer u.Close()

	testSCSIAddRemove(t, u, `/run/gcs/c/0/scsi`, "linux", []string{})

}

// TestSCSIAddRemoveWCOW validates adding and removing SCSI disks
// from a utility VM in both attach-only and with a container path. Also does
// negative testing so that a disk can't be attached twice.
func TestSCSIAddRemoveWCOW(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	u, layers, uvmScratchDir := testutilities.CreateWCOWUVM(t, t.Name(), "microsoft/nanoserver")
	defer os.RemoveAll(uvmScratchDir)
	defer u.Close()

	testSCSIAddRemove(t, u, `c:\`, "windows", layers)
}

func testSCSIAddRemove(t *testing.T, u *uvm.UtilityVM, pathPrefix string, operatingSystem string, wcowImageLayerFolders []string) {
	numDisks := 63 // Windows: 63 as the UVM scratch is at 0:0
	if operatingSystem == "linux" {
		numDisks++ //
	}

	// Create a bunch of directories each containing sandbox.vhdx
	disks := make([]string, numDisks)
	for i := 0; i < numDisks; i++ {
		tempDir := ""
		if operatingSystem == "windows" {
			tempDir = testutilities.CreateWCOWBlankRWLayer(t, wcowImageLayerFolders)
		} else {
			tempDir = testutilities.CreateLCOWBlankRWLayer(t, u.ID())
		}
		defer os.RemoveAll(tempDir)
		disks[i] = filepath.Join(tempDir, `sandbox.vhdx`)
	}

	// Add each of the disks to the utility VM. Attach-only, no container path
	logrus.Debugln("First - adding in attach-only")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], "", false)
		if err != nil {
			t.Fatalf("failed to add scsi disk %d %s: %s", i, disks[i], err)
		}
	}

	// Try to re-add. These should all fail.
	logrus.Debugln("Next - trying to re-add")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], "", false)
		if err == nil {
			t.Fatalf("should not be able to re-add the same SCSI disk!")
		}
		if err != uvm.ErrAlreadyAttached {
			t.Fatalf("expecting %s, got %s", uvm.ErrAlreadyAttached, err)
		}
	}

	// Remove them all
	logrus.Debugln("Removing them all")
	for i := 0; i < numDisks; i++ {
		if err := u.RemoveSCSI(disks[i]); err != nil {
			t.Fatalf("expected success: %s", err)
		}
	}

	// Now re-add but providing a container path
	logrus.Debugln("Next - re-adding with a container path")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], fmt.Sprintf(`%s%d`, pathPrefix, i), false)
		if err != nil {
			t.Fatalf("failed to add scsi disk %d %s: %s", i, disks[i], err)
		}
	}

	// Try to re-add. These should all fail.
	logrus.Debugln("Next - trying to re-add")
	for i := 0; i < numDisks; i++ {
		_, _, err := u.AddSCSI(disks[i], fmt.Sprintf(`%s%d`, pathPrefix, i), false)
		if err == nil {
			t.Fatalf("should not be able to re-add the same SCSI disk!")
		}
		if err != uvm.ErrAlreadyAttached {
			t.Fatalf("expecting %s, got %s", uvm.ErrAlreadyAttached, err)
		}
	}

	// Remove them all
	logrus.Debugln("Next - Removing them")
	for i := 0; i < numDisks; i++ {
		if err := u.RemoveSCSI(disks[i]); err != nil {
			t.Fatalf("expected success: %s", err)
		}
	}

	// TODO: Could extend to validate can't add a 64th disk (windows). 65th (linux).
}

func TestParallelScsiOps(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.RS5)
	u := testutilities.CreateLCOWUVM(t, t.Name())
	defer u.Close()

	// Create a sandbox to use
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create tmpdir for test: %v", err)
	}
	if err := lcow.CreateScratch(u, filepath.Join(tempDir, "sandbox.vhdx"), lcow.DefaultScratchSizeGB, "", u.ID()); err != nil {
		t.Fatalf("failed to create EXT4 scratch for LCOW test cases: %s", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove sandbox tmpdir: %v", err)
		}
	}()
	copySandbox := func(dir string, workerId, iteration int) (string, error) {
		orig, err := os.Open(filepath.Join(dir, "sandbox.vhdx"))
		if err != nil {
			return "", err
		}
		defer orig.Close()
		path := filepath.Join(dir, fmt.Sprintf("%d-%d-sandbox.vhdx", workerId, iteration))
		new, err := os.Create(path)
		if err != nil {
			return "", err
		}
		defer new.Close()

		_, err = io.Copy(new, orig)
		if err != nil {
			return "", err
		}
		return path, nil
	}

	// Note: maxWorkers cannot be > 64 for this code to work
	maxWorkers := 16
	opsChan := make(chan int, maxWorkers)
	opsWg := sync.WaitGroup{}
	opsWg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go func(scsiIndex int) {
			for {
				iteration, ok := <-opsChan
				if !ok {
					break
				}
				// Copy the goal sandbox.vhdx to a new path so we don't get the cached location
				path, err := copySandbox(tempDir, scsiIndex, iteration)
				if err != nil {
					t.Errorf("failed to copy sandbox for worker: %d, iteration: %d with err: %v", scsiIndex, iteration, err)
					continue
				}
				err = wclayer.GrantVmAccess(u.ID(), path)
				if err != nil {
					os.Remove(path)
					t.Errorf("failed to grantvmaccess for worker: %d, iteration: %d with err: %v", scsiIndex, iteration, err)
					continue
				}
				_, _, err = u.AddSCSI(path, "", false)
				if err != nil {
					os.Remove(path)
					t.Errorf("failed to AddSCSI for worker: %d, iteration: %d with err: %v", scsiIndex, iteration, err)
					continue
				}
				err = u.RemoveSCSI(path)
				if err != nil {
					t.Errorf("failed to RemoveSCSI for worker: %d, iteration: %d with err: %v", scsiIndex, iteration, err)
					// This worker cant continue because the index is dead. We have to stop
					break
				}
				_, _, err = u.AddSCSI(path, fmt.Sprintf("/run/gcs/c/0/scsi/%d", iteration), false)
				if err != nil {
					os.Remove(path)
					t.Errorf("failed to AddSCSI for worker: %d, iteration: %d with err: %v", scsiIndex, iteration, err)
					continue
				}
				err = u.RemoveSCSI(path)
				if err != nil {
					t.Errorf("failed to RemoveSCSI for worker: %d, iteration: %d with err: %v", scsiIndex, iteration, err)
					// This worker cant continue because the index is dead. We have to stop
					break
				}
				os.Remove(path)
			}
			opsWg.Done()
		}(i)
	}

	scsiOps := 1000
	for i := 0; i < scsiOps; i++ {
		opsChan <- i
	}
	close(opsChan)

	opsWg.Wait()
}

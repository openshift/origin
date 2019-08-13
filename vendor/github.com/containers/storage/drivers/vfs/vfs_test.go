// +build linux

package vfs

import (
	"testing"

	"github.com/containers/storage/drivers/graphtest"

	"github.com/containers/storage/pkg/reexec"
)

func init() {
	reexec.Init()
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestVfsSetup and TestVfsTeardown
func TestVfsSetup(t *testing.T) {
	graphtest.GetDriver(t, "vfs")
}

func TestVfsCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "vfs")
}

func TestVfsCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "vfs")
}

func TestVfsCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "vfs")
}

func TestVfsCreateFromTemplate(t *testing.T) {
	graphtest.DriverTestCreateFromTemplate(t, "vfs")
}

func TestVfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}

func TestVfsDiffApply100Files(t *testing.T) {
	graphtest.DriverTestDiffApply(t, 100, "vfs")
}

func TestVfsChanges(t *testing.T) {
	graphtest.DriverTestChanges(t, "vfs")
}

func TestVfsEcho(t *testing.T) {
	graphtest.DriverTestEcho(t, "vfs")
}

// +build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package fs

import (
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/containerd/continuity/testutil"
	"github.com/containerd/continuity/testutil/loopback"
)

func TestCopyReflinkWithXFS(t *testing.T) {
	testutil.RequiresRoot(t)
	mnt, err := ioutil.TempDir("", "containerd-test-copy-reflink-with-xfs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(mnt)

	loop, err := loopback.New(1 << 30) // sparse file (max=1GB)
	if err != nil {
		t.Fatal(err)
	}
	mkfs := []string{"mkfs.xfs", "-m", "crc=1", "-n", "ftype=1", "-m", "reflink=1"}
	if out, err := exec.Command(mkfs[0], append(mkfs[1:], loop.Device)...).CombinedOutput(); err != nil {
		// not fatal
		t.Skipf("could not mkfs (%v) %s: %v (out: %q)", mkfs, loop.Device, err, string(out))
	}
	loopbackSize, err := loop.HardSize()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Loopback file size (after mkfs (%v)): %d", mkfs, loopbackSize)
	if out, err := exec.Command("mount", loop.Device, mnt).CombinedOutput(); err != nil {
		// not fatal
		t.Skipf("could not mount %s: %v (out: %q)", loop.Device, err, string(out))
	}
	unmounted := false
	defer func() {
		if !unmounted {
			testutil.Unmount(t, mnt)
		}
		loop.Close()
	}()

	aPath := filepath.Join(mnt, "a")
	aSize := int64(100 << 20) // 100MB
	a, err := os.Create(aPath)
	if err != nil {
		t.Fatal(err)
	}
	randReader := rand.New(rand.NewSource(42))
	if _, err := io.CopyN(a, randReader, aSize); err != nil {
		a.Close()
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	bPath := filepath.Join(mnt, "b")
	if err := CopyFile(bPath, aPath); err != nil {
		t.Fatal(err)
	}
	testutil.Unmount(t, mnt)
	unmounted = true
	loopbackSize, err = loop.HardSize()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Loopback file size (after copying a %d-byte file): %d", aSize, loopbackSize)
	allowedSize := int64(120 << 20) // 120MB
	if loopbackSize > allowedSize {
		t.Fatalf("expected <= %d, got %d", allowedSize, loopbackSize)
	}
}

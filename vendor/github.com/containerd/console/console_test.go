// +build linux solaris

package console

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
)

func TestWinSize(t *testing.T) {
	c, _, err := NewPty()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Resize(WinSize{
		Width:  11,
		Height: 10,
	}); err != nil {
		t.Error(err)
		return
	}
	size, err := c.Size()
	if err != nil {
		t.Error(err)
		return
	}
	if size.Width != 11 {
		t.Errorf("width should be 11 but received %d", size.Width)
	}
	if size.Height != 10 {
		t.Errorf("height should be 10 but received %d", size.Height)
	}
}

func TestConsolePty(t *testing.T) {
	console, slavePath, err := NewPty()
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close()

	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()

	iteration := 10

	cmd := exec.Command("sh", "-c", fmt.Sprintf("for x in `seq 1 %d`; do echo -n test; done", iteration))
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	var (
		b  bytes.Buffer
		wg sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		io.Copy(&b, console)
		wg.Done()
	}()

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	slave.Close()
	wg.Wait()

	expectedOutput := ""
	for i := 0; i < iteration; i++ {
		expectedOutput += "test"
	}
	if out := b.String(); out != expectedOutput {
		t.Errorf("unexpected output %q", out)
	}
}

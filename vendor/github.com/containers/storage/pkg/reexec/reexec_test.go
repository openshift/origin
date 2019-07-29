package reexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sleepMessage = "sleeping"

func init() {
	Register("reexec", func() {
		panic("Return Error")
	})
	Register("sleep", func() {
		fmt.Printf(sleepMessage)
		time.Sleep(time.Hour)
		fmt.Printf("\nfinished " + sleepMessage)
	})
	Init()
}

func TestRegister(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			require.Equal(t, `reexec func already registered under name "reexec"`, r)
		}
	}()
	Register("reexec", func() {})
}

func TestCommand(t *testing.T) {
	cmd := Command("reexec")
	w, err := cmd.StdinPipe()
	require.NoError(t, err, "Error on pipe creation: %v", err)
	defer w.Close()

	err = cmd.Start()
	require.NoError(t, err, "Error on re-exec cmd: %v", err)
	err = cmd.Wait()
	require.EqualError(t, err, "exit status 2")
}

func TestCommandContext(t *testing.T) {
	stdout := &bytes.Buffer{}

	ctx, _ := context.WithDeadline(context.TODO(), time.Now().Add(5*time.Second))
	cmd := CommandContext(ctx, "sleep")
	w, err := cmd.StdinPipe()
	require.NoError(t, err, "Error on pipe creation: %v", err)
	defer w.Close()
	cmd.Stdout = stdout

	started := time.Now()
	err = cmd.Start()
	require.NoError(t, err, "Error on re-exec cmd: %v", err)
	err = cmd.Wait()
	require.NotNil(t, err, "Expected an error when the deadline was exceeded.")
	require.True(t, time.Since(started) < time.Hour/2, "Subprocess runtime exceeded deadline.")
	require.Equal(t, stdout.String(), sleepMessage, "error setting args for child process")
}

func TestNaiveSelf(t *testing.T) {
	if os.Getenv("TEST_CHECK") == "1" {
		os.Exit(2)
	}
	cmd := exec.Command(naiveSelf(), "-test.run=TestNaiveSelf")
	cmd.Env = append(os.Environ(), "TEST_CHECK=1")
	err := cmd.Start()
	require.NoError(t, err, "Unable to start command")
	err = cmd.Wait()
	require.EqualError(t, err, "exit status 2")

	os.Args[0] = "mkdir"
	assert.NotEqual(t, naiveSelf(), os.Args[0])
}

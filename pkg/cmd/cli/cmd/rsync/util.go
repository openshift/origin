package rsync

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

var (
	testRsyncCommand = []string{"rsync", "--version"}
	testTarCommand   = []string{"tar", "--version"}
)

// executeWithLogging will execute a command and log its output
func executeWithLogging(e executor, cmd []string) error {
	w := &bytes.Buffer{}
	err := e.Execute(cmd, nil, w, w)
	glog.V(4).Infof("%s", w.String())
	glog.V(4).Infof("error: %v", err)
	return err
}

// isWindows returns true if the current platform is windows
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// hasLocalRsync returns true if rsync is in current exec path
func hasLocalRsync() bool {
	_, err := exec.LookPath("rsync")
	if err != nil {
		return false
	}
	return true
}

// siblingCommand returns a sibling command to the current command
func siblingCommand(cmd *cobra.Command, name string) string {
	c := cmd.Parent()
	command := []string{}
	for c != nil {
		glog.V(5).Infof("Found parent command: %s", c.Name())
		command = append([]string{c.Name()}, command...)
		c = c.Parent()
	}
	// Replace the root command with what was actually used
	// in the command line
	glog.V(4).Infof("Setting root command to: %s", os.Args[0])
	command[0] = os.Args[0]

	// Append the sibling command
	command = append(command, name)
	glog.V(4).Infof("The sibling command is: %s", strings.Join(command, " "))

	return strings.Join(command, " ")
}

func isExitError(err error) bool {
	if err == nil {
		return false
	}
	_, exitErr := err.(*exec.ExitError)
	return exitErr
}

func checkRsync(e executor) error {
	return executeWithLogging(e, testRsyncCommand)
}

func checkTar(e executor) error {
	return executeWithLogging(e, testTarCommand)
}

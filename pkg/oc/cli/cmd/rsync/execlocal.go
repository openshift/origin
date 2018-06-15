package rsync

import (
	"io"
	"os/exec"
	"strings"

	"github.com/golang/glog"
)

// localExecutor will execute commands on the local machine
type localExecutor struct{}

// ensure localExecutor implements the executor interface
var _ executor = &localExecutor{}

// Execute will run a command locally
func (*localExecutor) Execute(command []string, in io.Reader, out, errOut io.Writer) error {
	glog.V(3).Infof("Local executor running command: %s", strings.Join(command, " "))
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	cmd.Stdin = in
	err := cmd.Run()
	if err != nil {
		glog.V(4).Infof("Error from local command execution: %v", err)
	}
	return err
}

// newLocalExecutor instantiates a local executor
func newLocalExecutor() executor {
	return &localExecutor{}
}

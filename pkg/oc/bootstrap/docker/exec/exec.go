package exec

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

// ExecHelper allows execution of commands on a running Docker container
type ExecHelper struct {
	client    dockerhelper.Interface
	container string
}

// ExecCommand is a command to execute with the helper
type ExecCommand struct {
	helper *ExecHelper
	cmd    []string
	input  io.Reader
}

// NewExecHelper creates a new ExecHelper
func NewExecHelper(client dockerhelper.Interface, container string) *ExecHelper {
	return &ExecHelper{
		client:    client,
		container: container,
	}
}

// Command creates a new command to execute
func (h *ExecHelper) Command(cmd ...string) *ExecCommand {
	return &ExecCommand{
		helper: h,
		cmd:    cmd,
	}
}

// Input sets an input reader on the exec command
func (c *ExecCommand) Input(in io.Reader) *ExecCommand {
	c.input = in
	return c
}

// Output executes the command and returns seprate stderr and stdout
func (c *ExecCommand) Output() (string, string, error) {
	stdOut, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	err := exec(c.helper, c.cmd, c.input, stdOut, errOut)
	return stdOut.String(), errOut.String(), err
}

// CombinedOutput executes the command and returns a single output
func (c *ExecCommand) CombinedOutput() (string, error) {
	out := &bytes.Buffer{}
	err := exec(c.helper, c.cmd, c.input, out, out)
	return out.String(), err
}

// Run executes the command
func (c *ExecCommand) Run() error {
	return exec(c.helper, c.cmd, c.input, ioutil.Discard, ioutil.Discard)
}

func exec(h *ExecHelper, cmd []string, stdIn io.Reader, stdOut, errOut io.Writer) error {
	glog.V(4).Infof("Remote exec on container: %s\nCommand: %v", h.container, cmd)
	response, err := h.client.ContainerExecCreate(h.container, types.ExecConfig{
		AttachStdin:  stdIn != nil,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return errors.NewError("Cannot create exec for command %v on container %s", cmd, h.container).WithCause(err)
	}
	glog.V(5).Infof("Created exec %q", response.ID)
	logOut, logErr := &bytes.Buffer{}, &bytes.Buffer{}
	outStream := io.MultiWriter(stdOut, logOut)
	errStream := io.MultiWriter(errOut, logErr)
	glog.V(5).Infof("Starting exec %q and blocking", response.ID)
	err = h.client.ContainerExecAttach(response.ID, stdIn, outStream, errStream)
	if err != nil {
		return errors.NewError("Cannot start exec for command %v on container %s", cmd, h.container).WithCause(err)
	}
	if glog.V(5) {
		glog.Infof("Exec %q completed", response.ID)
		if logOut.Len() > 0 {
			glog.Infof("Stdout:\n%s", logOut.String())
		}
		if logErr.Len() > 0 {
			glog.Infof("Stderr:\n%s", logErr.String())
		}
	}
	glog.V(5).Infof("Inspecting exec %q", response.ID)
	info, err := h.client.ContainerExecInspect(response.ID)
	if err != nil {
		return errors.NewError("Cannot inspect result of exec for command %v on container %s", cmd, h.container).WithCause(err)
	}
	glog.V(5).Infof("Exec %q info: %#v", response.ID, info)
	if info.ExitCode != 0 {
		return newExecError(err, info.ExitCode, logOut.String(), logErr.String(), h.container, cmd)
	}
	return nil
}

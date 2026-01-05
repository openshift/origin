package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// contains check list contain one string
func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(a, e) {
			return true
		}
	}
	return false
}

// DockerCLI provides function to run the docker command
type DockerCLI struct {
	CLI             *client.Client
	execPath        string
	ExecCommandPath string
	globalArgs      []string
	commandArgs     []string
	finalArgs       []string
	verbose         bool
	stdin           *bytes.Buffer
	stdout          io.Writer
	stderr          io.Writer
	showInfo        bool
}

// NewDockerCLI initialize the docker cli framework
func NewDockerCLI() *DockerCLI {
	newclient := &DockerCLI{}
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		e2e.Failf("get docker client failed")
	}
	newclient.CLI = cli
	newclient.execPath = "docker"
	newclient.showInfo = true
	return newclient
}

// Run executes given docker command
func (c *DockerCLI) Run(commands ...string) *DockerCLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	docker := &DockerCLI{
		execPath:        c.execPath,
		ExecCommandPath: c.ExecCommandPath,
	}
	docker.globalArgs = commands
	docker.stdin, docker.stdout, docker.stderr = in, out, errout
	return docker.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *DockerCLI) setOutput(out io.Writer) *DockerCLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the docker exutil.CLI command
func (c *DockerCLI) Args(args ...string) *DockerCLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	return c
}

func (c *DockerCLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *DockerCLI) Output() (string, error) {
	if c.verbose {
		e2e.Logf("DEBUG: docker %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	if c.ExecCommandPath != "" {
		e2e.Logf("set exec command path is %s\n", c.ExecCommandPath)
		cmd.Dir = c.ExecCommandPath
	}
	cmd.Stdin = c.stdin
	if c.showInfo {
		e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))
	}
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	switch err.(type) {
	case nil:
		c.stdout = bytes.NewBuffer(out)
		return trimmed, nil
	case *exec.ExitError:
		e2e.Logf("Error running %v:\n%s", cmd, trimmed)
		return trimmed, &ExitError{ExitError: err.(*exec.ExitError), Cmd: c.execPath + " " + strings.Join(c.finalArgs, " "), StdErr: trimmed}
	default:
		FatalErr(fmt.Errorf("unable to execute %q: %v", c.execPath, err))
		// unreachable code
		return "", nil
	}
}

// GetImageID is to get the image ID by image tag
func (c *DockerCLI) GetImageID(imageTag string) (string, error) {
	imageID := ""
	ctx := context.Background()
	images, err := c.CLI.ImageList(ctx, image.ListOptions{})
	if err != nil {
		e2e.Logf("get docker image list failed")
		return imageID, err
	}
	for _, image := range images {
		if strings.Contains(strings.Join(image.RepoTags, ","), imageTag) {
			e2e.Logf("image ID is %s\n", image.ID)
			return image.ID, nil
		}
	}
	return imageID, nil
}

// ImageRemove is to remove the image
func (c *DockerCLI) ImageRemove(imageID string) error {
	ctx := context.Background()
	_, err := c.CLI.ImageRemove(ctx, imageID, image.RemoveOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetImageList is to get the image list
func (c *DockerCLI) GetImageList() ([]string, error) {
	var imageList []string
	ctx := context.Background()

	images, err := c.CLI.ImageList(ctx, image.ListOptions{})
	if err != nil {
		e2e.Logf("get docker image list failed")
		return imageList, err
	}
	for _, image := range images {
		e2e.Logf("image: %s\n", strings.Join(image.RepoTags, ","))
		imageList = append(imageList, strings.Join(image.RepoTags, ","))
	}
	return imageList, nil
}

// CheckImageExist check the image exist
func (c *DockerCLI) CheckImageExist(imageIndex string) (bool, error) {
	imageList, err := c.GetImageList()
	if err != nil {
		return false, err
	}
	return contains(imageList, imageIndex), nil
}

func (c *DockerCLI) ContainerCreate(imageName string, containerName string, entrypoint string, openStdin bool) (string, error) {
	cli := c.CLI
	ctx := context.Background()
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      imageName,
		OpenStdin:  openStdin,
		Tty:        true,
		Entrypoint: []string{entrypoint},
	}, nil, nil, nil, containerName)
	return resp.ID, err
}

func (c *DockerCLI) ContainerStop(id string) error {
	cli := c.CLI
	ctx := context.Background()
	err := cli.ContainerStop(ctx, id, container.StopOptions{})
	return err
}

func (c *DockerCLI) ContainerRemove(id string) error {
	cli := c.CLI
	ctx := context.Background()
	err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
	return err
}

func (c *DockerCLI) ContainerStart(id string) error {
	cli := c.CLI
	ctx := context.Background()
	err := cli.ContainerStart(ctx, id, container.StartOptions{})
	return err
}

func (c *DockerCLI) Exec(id string, cmd []string) (int, string, string, error) {
	// prepare exec
	cli := c.CLI
	ctx := context.Background()
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	cresp, err := cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return 1, "", "", err
	}
	execID := cresp.ID

	// run it, with stdout/stderr attached
	aresp, err := cli.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{})
	if err != nil {
		return 1, "", "", err
	}
	defer aresp.Close()

	// read the output
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, aresp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return 1, "", "", err
		}
		break

	case <-ctx.Done():
		return 1, "", "", ctx.Err()
	}

	// get the exit code
	iresp, err := cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return 1, "", "", err
	}

	return iresp.ExitCode, outBuf.String(), errBuf.String(), nil
}

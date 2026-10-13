package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ExitError returns the error info
type ExitError struct {
	Cmd    string
	StdErr string
	*exec.ExitError
}

// FatalErr exits the test in case a fatal error has occurred.
func FatalErr(msg interface{}) {
	// the path that leads to this being called isn't always clear...
	fmt.Fprintln(g.GinkgoWriter, string(debug.Stack()))
	e2e.Failf("%v", msg)
}

// PodmanImage podman image
type PodmanImage struct {
	ID         string            `json:"Id"`
	Size       int64             `json:"Size"`
	Labels     map[string]string `json:"Labels"`
	Names      []string          `json:"Names"`
	Digest     string            `json:"Digest"`
	Digests    []string          `json:"Digests"`
	Dangling   bool              `json:"Dangling"`
	History    []string          `json:"History"`
	Containers int64             `json:"Containers"`
}

// PodmanCLI provides function to run the docker command
type PodmanCLI struct {
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
	UnsetProxy      bool
	env             []string
}

// NewPodmanCLI initialize the docker cli framework
func NewPodmanCLI() *PodmanCLI {
	newclient := &PodmanCLI{}
	newclient.execPath = "podman"
	newclient.showInfo = true
	newclient.UnsetProxy = false
	return newclient
}

// Run executes given Podman command
func (c *PodmanCLI) Run(commands ...string) *PodmanCLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	podman := &PodmanCLI{
		execPath:        c.execPath,
		ExecCommandPath: c.ExecCommandPath,
		UnsetProxy:      c.UnsetProxy,
		showInfo:        c.showInfo,
		env:             c.env,
	}
	podman.globalArgs = commands
	podman.stdin, podman.stdout, podman.stderr = in, out, errout
	return podman.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *PodmanCLI) setOutput(out io.Writer) *PodmanCLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the podman exutil.CLI command
func (c *PodmanCLI) Args(args ...string) *PodmanCLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	return c
}

func (c *PodmanCLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *PodmanCLI) Output() (string, error) {
	if c.verbose {
		e2e.Logf("DEBUG: podman %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	cmd.Env = os.Environ()
	if c.UnsetProxy {
		var envCmd []string
		for _, envIndex := range cmd.Env {
			if !(strings.Contains(strings.ToUpper(envIndex), "HTTP_PROXY") || strings.Contains(strings.ToUpper(envIndex), "HTTPS_PROXY") || strings.Contains(strings.ToUpper(envIndex), "NO_PROXY")) {
				envCmd = append(envCmd, envIndex)
			}
		}
		cmd.Env = envCmd
	}
	if c.env != nil {
		cmd.Env = append(cmd.Env, c.env...)
	}
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

// GetImageList to get the image list
func (c *PodmanCLI) GetImageList() ([]string, error) {
	var imageList []string
	images, err := c.GetImages()
	if err != nil {
		return imageList, err
	}
	for _, imageIndex := range images {
		e2e.Logf("ID %s, name: %s", imageIndex.ID, strings.Join(imageIndex.Names, ","))
		imageList = append(imageList, strings.Join(imageIndex.Names, ","))
	}
	return imageList, nil
}

func (c *PodmanCLI) GetImages() ([]PodmanImage, error) {
	output, err := c.Run("images").Args("--format", "json").Output()
	if err != nil {
		e2e.Logf("Failed to run 'podman images --format json'")
		return nil, err
	}

	images, err := c.GetImagesByJSON(output)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (c *PodmanCLI) GetImagesByJSON(jsonStr string) ([]PodmanImage, error) {
	var images []PodmanImage

	if err := json.Unmarshal([]byte(jsonStr), &images); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON file: %v", err)
	}

	return images, nil
}

// CheckImageExist is to check the image is exist
func (c *PodmanCLI) CheckImageExist(imageIndex string) (bool, error) {
	e2e.Logf("check image %s is exist", imageIndex)
	imageList, err := c.GetImageList()
	if err != nil {
		return false, err
	}
	return contains(imageList, imageIndex), nil
}

// GetImageID is to get the image ID by image tag
func (c *PodmanCLI) GetImageID(imageTag string) (string, error) {
	imageID, err := c.Run("images").Args(imageTag, "--format", "{{.ID}}").Output()
	if err != nil {
		e2e.Logf("Failed to run 'podman images --format {{.ID}}'")
		return "", err
	}
	return imageID, nil
}

// RemoveImage is to remove image
func (c *PodmanCLI) RemoveImage(imageIndex string) (bool, error) {
	imageID, err := c.GetImageID(imageIndex)
	if err != nil {
		return false, err
	}
	if imageID == "" {
		return true, nil
	}
	e2e.Logf("imageID is %s\n", imageID)
	_, err = c.Run("image").Args("rm", "-f", imageID).Output()
	if err != nil {
		e2e.Logf("remove image %s failed", imageID)
		return false, err
	}
	e2e.Logf("remove image %s success\n", imageID)

	return true, nil
}

func (c *PodmanCLI) ContainerCreate(imageName string, containerName string, entrypoint string, openStdin bool) (string, error) {
	interactiveStr := "--interactive=false"
	if openStdin {
		interactiveStr = "--interactive=true"
	}
	output, err := c.Run("create").Args(interactiveStr, "--entrypoint="+entrypoint, "--name="+containerName, imageName).Output()
	if err != nil {
		e2e.Logf("run podman create faild")
		return "", err
	}
	outputLines := strings.Split(strings.Trim(output, "\n"), "\n")
	containerID := outputLines[len(outputLines)-1]
	return containerID, nil
}

func (c *PodmanCLI) ContainerStart(id string) error {
	_, err := c.Run("start").Args(id).Output()
	if err != nil {
		e2e.Logf("run podman start %s failed", id)
	}
	return err
}

func (c *PodmanCLI) ContainerStop(id string) error {
	_, err := c.Run("stop").Args(id).Output()
	if err != nil {
		e2e.Logf("run podman stop %s failed", id)
	}
	return err
}

func (c *PodmanCLI) ContainerRemove(id string) error {
	_, err := c.Run("rm").Args(id, "-f").Output()
	if err != nil {
		e2e.Logf("run podman rm %s failed", id)
	}
	return err
}

func (c *PodmanCLI) Exec(id string, commands []string) (string, error) {
	commands = append([]string{id}, commands...)
	output, err := c.Run("exec").Args(commands...).Output()
	if err != nil {
		e2e.Logf("run podman exec %s faild", commands)
		return "", err
	}
	return output, nil
}

func (c *PodmanCLI) ExecBackgroud(id string, commands []string) (string, error) {
	commands = append([]string{"--detach", id}, commands...)
	output, err := c.Run("exec").Args(commands...).Output()
	if err != nil {
		e2e.Logf("run podman exec %s faild", commands)
		return "", err
	}
	return output, nil
}

func (c *PodmanCLI) CopyFile(id string, src string, target string) error {
	_, err := c.Run("cp").Args(src, id+":"+target).Output()
	if err != nil {
		e2e.Logf("run podman cp failed")
	}
	return err
}

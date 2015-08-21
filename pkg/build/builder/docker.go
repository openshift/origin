package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	dockercmd "github.com/docker/docker/builder/command"
	"github.com/docker/docker/builder/parser"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/source-to-image/pkg/git"
	"github.com/openshift/source-to-image/pkg/tar"
)

const (
	// urlCheckTimeout is the timeout used to check the source URL
	// If fetching the URL exceeds the timeout, then the build will
	// not proceed further and stop
	urlCheckTimeout = 16 * time.Second

	// noOutputDefaultTag is used as the tag name for docker built images that will
	// not be pushed because no Output value was defined in the BuildConfig
	noOutputDefaultTag = "no_repo/no_output_default_tag"
)

// DockerBuilder builds Docker images given a git repository URL
type DockerBuilder struct {
	dockerClient DockerClient
	git          git.Git
	tar          tar.Tar
	build        *api.Build
	urlTimeout   time.Duration
}

// NewDockerBuilder creates a new instance of DockerBuilder
func NewDockerBuilder(dockerClient DockerClient, build *api.Build) *DockerBuilder {
	return &DockerBuilder{
		dockerClient: dockerClient,
		build:        build,
		git:          git.New(),
		tar:          tar.New(),
		urlTimeout:   urlCheckTimeout,
	}
}

// Build executes a Docker build
func (d *DockerBuilder) Build() error {
	buildDir, err := ioutil.TempDir("", "docker-build")
	if err != nil {
		return err
	}
	if err = d.fetchSource(buildDir); err != nil {
		return err
	}
	if err = d.addBuildParameters(buildDir); err != nil {
		return err
	}
	glog.V(4).Infof("Starting Docker build from %s/%s BuildConfig ...", d.build.Namespace, d.build.Name)
	var push bool

	// if there is no output target, set one up so the docker build logic
	// will still work, but we won't push it at the end.
	if d.build.Spec.Output.To == nil || len(d.build.Spec.Output.To.Name) == 0 {
		d.build.Spec.Output.To = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: noOutputDefaultTag,
		}
		push = false
	} else {
		push = true
	}

	if err = d.dockerBuild(buildDir); err != nil {
		return err
	}

	defer removeImage(d.dockerClient, d.build.Spec.Output.To.Name)

	if push {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			d.build.Spec.Output.To.Name,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.Infof("Using provided push secret for pushing %s image", d.build.Spec.Output.To.Name)
		}
		glog.Infof("Pushing %s image ...", d.build.Spec.Output.To.Name)
		if err := pushImage(d.dockerClient, d.build.Spec.Output.To.Name, pushAuthConfig); err != nil {
			return fmt.Errorf("Failed to push image: %v", err)
		}
		glog.Infof("Successfully pushed %s", d.build.Spec.Output.To.Name)
	}
	return nil
}

// checkSourceURI performs a check on the URI associated with the build
// to make sure that it is live before proceeding with the build.
func (d *DockerBuilder) checkSourceURI() error {
	rawurl := d.build.Spec.Source.Git.URI
	if !d.git.ValidCloneSpec(rawurl) {
		return fmt.Errorf("Invalid git source url: %s", rawurl)
	}
	if strings.HasPrefix(rawurl, "git://") || strings.HasPrefix(rawurl, "git@") {
		return nil
	}
	if !strings.HasPrefix(rawurl, "http://") && !strings.HasPrefix(rawurl, "https://") {
		rawurl = fmt.Sprintf("https://%s", rawurl)
	}
	srcURL, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	host := srcURL.Host
	if strings.Index(host, ":") == -1 {
		switch srcURL.Scheme {
		case "http":
			host += ":80"
		case "https":
			host += ":443"
		}
	}
	dialer := net.Dialer{Timeout: d.urlTimeout}
	conn, err := dialer.Dial("tcp", host)
	if err != nil {
		return err
	}
	return conn.Close()

}

// fetchSource retrieves the git source from the repository. If a commit ID
// is included in the build revision, that commit ID is checked out. Otherwise
// if a ref is included in the source definition, that ref is checked out.
func (d *DockerBuilder) fetchSource(dir string) error {
	if err := d.checkSourceURI(); err != nil {
		return err
	}
	origProxy := make(map[string]string)
	var setHttp, setHttps bool
	// set the http proxy to be used by the git clone performed by S2I
	if len(d.build.Spec.Source.Git.HTTPSProxy) != 0 {
		glog.V(2).Infof("Setting https proxy variables for Git to %s", d.build.Spec.Source.Git.HTTPSProxy)
		origProxy["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		origProxy["https_proxy"] = os.Getenv("https_proxy")
		os.Setenv("HTTPS_PROXY", d.build.Spec.Source.Git.HTTPSProxy)
		os.Setenv("https_proxy", d.build.Spec.Source.Git.HTTPSProxy)
		setHttps = true
	}
	if len(d.build.Spec.Source.Git.HTTPProxy) != 0 {
		glog.V(2).Infof("Setting http proxy variables for Git to %s", d.build.Spec.Source.Git.HTTPSProxy)
		origProxy["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		origProxy["http_proxy"] = os.Getenv("http_proxy")
		os.Setenv("HTTP_PROXY", d.build.Spec.Source.Git.HTTPProxy)
		os.Setenv("http_proxy", d.build.Spec.Source.Git.HTTPProxy)
		setHttp = true
	}
	defer func() {
		// reset http proxy env variables to original value
		if setHttps {
			glog.V(4).Infof("Resetting HTTPS_PROXY variable for Git to %s", origProxy["HTTPS_PROXY"])
			os.Setenv("HTTPS_PROXY", origProxy["HTTPS_PROXY"])
			glog.V(4).Infof("Resetting https_proxy variable for Git to %s", origProxy["https_proxy"])
			os.Setenv("https_proxy", origProxy["https_proxy"])
		}
		if setHttp {
			glog.V(4).Infof("Resetting HTTP_PROXY variable for Git to %s", origProxy["HTTP_PROXY"])
			os.Setenv("HTTP_PROXY", origProxy["HTTP_PROXY"])
			glog.V(4).Infof("Resetting http_proxy variable for Git to %s", origProxy["http_proxy"])
			os.Setenv("http_proxy", origProxy["http_proxy"])
		}
	}()

	if err := d.git.Clone(d.build.Spec.Source.Git.URI, dir); err != nil {
		return err
	}

	if d.build.Spec.Source.Git.Ref == "" &&
		(d.build.Spec.Revision == nil ||
			d.build.Spec.Revision.Git == nil ||
			d.build.Spec.Revision.Git.Commit == "") {
		return nil
	}
	if d.build.Spec.Revision != nil &&
		d.build.Spec.Revision.Git != nil &&
		d.build.Spec.Revision.Git.Commit != "" {
		return d.git.Checkout(dir, d.build.Spec.Revision.Git.Commit)
	}
	return d.git.Checkout(dir, d.build.Spec.Source.Git.Ref)
}

// addBuildParameters checks if a Image is set to replace the default base image.
// If that's the case then change the Dockerfile to make the build with the given image.
// Also append the environment variables in the Dockerfile.
func (d *DockerBuilder) addBuildParameters(dir string) error {
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if d.build.Spec.Strategy.DockerStrategy != nil && len(d.build.Spec.Source.ContextDir) > 0 {
		dockerfilePath = filepath.Join(dir, d.build.Spec.Source.ContextDir, "Dockerfile")
	}

	fileStat, err := os.Lstat(dockerfilePath)
	if err != nil {
		return err
	}

	filePerm := fileStat.Mode()

	fileData, err := ioutil.ReadFile(dockerfilePath)
	if err != nil {
		return err
	}

	var newFileData string
	if d.build.Spec.Strategy.DockerStrategy.From != nil && d.build.Spec.Strategy.DockerStrategy.From.Kind == "DockerImage" {
		newFileData, err = replaceValidCmd(dockercmd.From, d.build.Spec.Strategy.DockerStrategy.From.Name, fileData)
		if err != nil {
			return err
		}
	} else {
		newFileData = newFileData + string(fileData)
	}

	envVars := getBuildEnvVars(d.build)
	newFileData = appendEnvVars(newFileData, envVars)

	if ioutil.WriteFile(dockerfilePath, []byte(newFileData), filePerm); err != nil {
		return err
	}

	return nil
}

// appendEnvVars appends environment variables to a string containing
// a valid Dockerfile
func appendEnvVars(fileData string, envVars map[string]string) string {
	if !strings.HasSuffix(fileData, "\n") {
		fileData += "\n"
	}
	first := true
	for k, v := range envVars {
		if first {
			fileData += fmt.Sprintf("ENV %s=\"%s\"", k, v)
			first = false
		} else {
			fileData += fmt.Sprintf(" \\\n\t%s=\"%s\"", k, v)
		}
	}
	fileData += "\n"
	return fileData
}

// invalidCmdErr represents an error returned from replaceValidCmd
// when an invalid Dockerfile command has been passed to
// replaceValidCmd
var invalidCmdErr = errors.New("invalid Dockerfile command")

// replaceCmdErr represents an error returned from replaceValidCmd
// when a command which has more than one valid occurrences inside
// a Dockerfile has been passed or the specified command cannot
// be found
var replaceCmdErr = errors.New("cannot replace given Dockerfile command")

// replaceValidCmd replaces the valid occurrence of a command
// in a Dockerfile with the given replaceArgs
func replaceValidCmd(cmd, replaceArgs string, fileData []byte) (string, error) {
	if _, ok := dockercmd.Commands[cmd]; !ok {
		return "", invalidCmdErr
	}
	buf := bytes.NewBuffer(fileData)
	// Parse with Docker parser
	node, err := parser.Parse(buf)
	if err != nil {
		return "", errors.New("cannot parse Dockerfile: " + err.Error())
	}

	pos := traverseAST(cmd, node)
	if pos == 0 {
		return "", replaceCmdErr
	}

	// Re-initialize the buffer
	buf = bytes.NewBuffer(fileData)
	var newFileData string
	var index int
	var replaceNextLn bool
	for {
		line, err := buf.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		line = strings.TrimSpace(line)

		// The current line starts with the specified command (cmd)
		if strings.HasPrefix(strings.ToUpper(line), strings.ToUpper(cmd)) {
			index++

			// The current line finishes on a backslash.
			// All we need to do is replace the next line
			// with our specified replaceArgs
			if line[len(line)-1:] == "\\" && index == pos {
				replaceNextLn = true

				args := strings.Split(line, " ")
				if len(args) > 2 {
					// Keep just our Dockerfile command and the backslash
					newFileData += args[0] + " \\" + "\n"
				} else {
					newFileData += line + "\n"
				}
				continue
			}

			// Normal ending line
			if index == pos {
				line = fmt.Sprintf("%s %s", strings.ToUpper(cmd), replaceArgs)
			}
		}

		// Previous line ended on a backslash
		// This line contains command arguments
		if replaceNextLn {
			if line[len(line)-1:] == "\\" {
				// Ignore all successive lines terminating on a backslash
				// since they all are going to be replaced by replaceArgs
				continue
			}
			replaceNextLn = false
			line = replaceArgs
		}

		if err == io.EOF {
			// Otherwise, the new Dockerfile will have one newline
			// more in the end
			newFileData += line
			break
		}
		newFileData += line + "\n"
	}

	// Parse output for validation
	buf = bytes.NewBuffer([]byte(newFileData))
	if _, err := parser.Parse(buf); err != nil {
		return "", errors.New("cannot parse new Dockerfile: " + err.Error())
	}

	return newFileData, nil
}

// traverseAST traverses the Abstract Syntax Tree output
// from the Docker parser and returns the valid position
// of the command it was requested to look for.
//
// Note that this function is intended to be used with
// Dockerfile commands that should be specified only once
// in a Dockerfile (FROM, CMD, ENTRYPOINT)
func traverseAST(cmd string, node *parser.Node) int {
	switch cmd {
	case dockercmd.From, dockercmd.Entrypoint, dockercmd.Cmd:
	default:
		return 0
	}

	index := 0
	if node.Value == cmd {
		index++
	}
	for _, n := range node.Children {
		index += traverseAST(cmd, n)
	}
	if node.Next != nil {
		for n := node.Next; n != nil; n = n.Next {
			if len(n.Children) > 0 {
				index += traverseAST(cmd, n)
			} else if n.Value == cmd {
				index++
			}
		}
	}
	return index
}

// setupPullSecret provides a Docker authentication configuration when the
// PullSecret is specified.
func (d *DockerBuilder) setupPullSecret() (*docker.AuthConfigurations, error) {
	if len(os.Getenv(dockercfg.PullAuthType)) == 0 {
		return nil, nil
	}
	r, err := os.Open(os.Getenv(dockercfg.PullAuthType))
	if err != nil {
		return nil, fmt.Errorf("'%s': %s", os.Getenv(dockercfg.PullAuthType), err)
	}
	return docker.NewAuthConfigurations(r)
}

// dockerBuild performs a docker build on the source that has been retrieved
func (d *DockerBuilder) dockerBuild(dir string) error {
	var noCache bool
	var forcePull bool
	if d.build.Spec.Strategy.DockerStrategy != nil {
		if d.build.Spec.Source.ContextDir != "" {
			dir = filepath.Join(dir, d.build.Spec.Source.ContextDir)
		}
		noCache = d.build.Spec.Strategy.DockerStrategy.NoCache
		forcePull = d.build.Spec.Strategy.DockerStrategy.ForcePull
	}

	// TODO: Remove this method call when Docker build auth is fixed
	var err error
	if forcePull, err = d.pullDockerImage(forcePull); err != nil {
		return err
	}

	auth, err := d.setupPullSecret()
	if err != nil {
		return err
	}
	return buildImage(d.dockerClient, dir, noCache, d.build.Spec.Output.To.Name, d.tar, auth, forcePull)
}

// TODO: Remove this method when Docker build auth is fixed
func (d *DockerBuilder) pullDockerImage(force bool) (bool, error) {
	if d.build.Spec.Strategy.DockerStrategy.From == nil || d.build.Spec.Strategy.DockerStrategy.From.Kind != "DockerImage" {
		return force, nil
	}
	image := d.build.Spec.Strategy.DockerStrategy.From.Name
	_, tag := docker.ParseRepositoryTag(image)
	if len(tag) == 0 {
		image = strings.Join([]string{image, imageapi.DefaultImageTag}, ":")
	}
	pullAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(image, dockercfg.PullAuthType)
	if !authPresent {
		return force, nil
	}
	glog.V(2).Infof("Pre-pulling docker image %s", image)
	pullOpts := docker.PullImageOptions{Repository: image}
	if err := d.dockerClient.PullImage(pullOpts, pullAuthConfig); err != nil {
		return force, fmt.Errorf("error pulling image %s: %v", image, err)
	}
	return false, nil
}

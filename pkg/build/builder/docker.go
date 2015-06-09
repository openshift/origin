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
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/source-to-image/pkg/git"
	"github.com/openshift/source-to-image/pkg/tar"

	docker "github.com/fsouza/go-dockerclient"
)

// urlCheckTimeout is the timeout used to check the source URL
// If fetching the URL exceeds the timeout, then the build will
// not proceed further and stop
const urlCheckTimeout = 16 * time.Second

// DockerBuilder builds Docker images given a git repository URL
type DockerBuilder struct {
	dockerClient DockerClient
	authPresent  bool
	auth         docker.AuthConfiguration
	git          git.Git
	tar          tar.Tar
	build        *api.Build
	urlTimeout   time.Duration
}

// NewDockerBuilder creates a new instance of DockerBuilder
func NewDockerBuilder(dockerClient DockerClient, authCfg docker.AuthConfiguration, authPresent bool, build *api.Build) *DockerBuilder {
	return &DockerBuilder{
		dockerClient: dockerClient,
		authPresent:  authPresent,
		auth:         authCfg,
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
	if err = d.dockerBuild(buildDir); err != nil {
		return err
	}
	tag := d.build.Parameters.Output.DockerImageReference
	defer removeImage(d.dockerClient, tag)

	dockerImageRef := d.build.Parameters.Output.DockerImageReference
	if len(dockerImageRef) != 0 {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			dockerImageRef,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.V(3).Infof("Using Docker authentication provided")
			d.auth = pushAuthConfig
		}
		glog.Infof("Pushing %s image ...", dockerImageRef)
		if err := pushImage(d.dockerClient, tag, d.auth); err != nil {
			return fmt.Errorf("Failed to push image: %v", err)
		}
		glog.Infof("Successfully pushed %s", dockerImageRef)
	}
	return nil
}

// checkSourceURI performs a check on the URI associated with the build
// to make sure that it is live before proceeding with the build.
func (d *DockerBuilder) checkSourceURI() error {
	rawurl := d.build.Parameters.Source.Git.URI
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
	if err := d.git.Clone(d.build.Parameters.Source.Git.URI, dir); err != nil {
		return err
	}
	if d.build.Parameters.Source.Git.Ref == "" &&
		(d.build.Parameters.Revision == nil ||
			d.build.Parameters.Revision.Git == nil ||
			d.build.Parameters.Revision.Git.Commit == "") {
		return nil
	}
	if d.build.Parameters.Revision != nil &&
		d.build.Parameters.Revision.Git != nil &&
		d.build.Parameters.Revision.Git.Commit != "" {
		return d.git.Checkout(dir, d.build.Parameters.Revision.Git.Commit)
	}
	return d.git.Checkout(dir, d.build.Parameters.Source.Git.Ref)
}

// addBuildParameters checks if a Image is set to replace the default base image.
// If that's the case then change the Dockerfile to make the build with the given image.
// Also append the environment variables in the Dockerfile.
func (d *DockerBuilder) addBuildParameters(dir string) error {
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if d.build.Parameters.Strategy.DockerStrategy != nil && len(d.build.Parameters.Source.ContextDir) > 0 {
		dockerfilePath = filepath.Join(dir, d.build.Parameters.Source.ContextDir, "Dockerfile")
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
	if d.build.Parameters.Strategy.DockerStrategy.From != nil && d.build.Parameters.Strategy.DockerStrategy.From.Kind == "DockerImage" {
		newFileData, err = replaceValidCmd(dockercmd.From, d.build.Parameters.Strategy.DockerStrategy.From.Name, fileData)
		if err != nil {
			return err
		}
	} else {
		newFileData = newFileData + string(fileData)
	}

	envVars := getBuildEnvVars(d.build)
	first := true
	for k, v := range envVars {
		if first {
			newFileData += fmt.Sprintf("ENV %s=\"%s\"", k, v)
			first = false
		} else {
			newFileData += fmt.Sprintf(" \\\n\t%s=\"%s\"", k, v)
		}
	}
	newFileData += "\n"

	if ioutil.WriteFile(dockerfilePath, []byte(newFileData), filePerm); err != nil {
		return err
	}

	return nil
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
	if len(os.Getenv("PULL_DOCKERCFG_PATH")) == 0 {
		return nil, nil
	}
	r, err := os.Open(os.Getenv("PULL_DOCKERCFG_PATH"))
	if err != nil {
		return nil, fmt.Errorf("'%s': %s", os.Getenv("PULL_DOCKERCFG_PATH"), err)
	}
	return docker.NewAuthConfigurations(r)
}

// dockerBuild performs a docker build on the source that has been retrieved
func (d *DockerBuilder) dockerBuild(dir string) error {
	var noCache bool
	if d.build.Parameters.Strategy.DockerStrategy != nil {
		if d.build.Parameters.Source.ContextDir != "" {
			dir = filepath.Join(dir, d.build.Parameters.Source.ContextDir)
		}
		noCache = d.build.Parameters.Strategy.DockerStrategy.NoCache
	}
	auth, err := d.setupPullSecret()
	if err != nil {
		return err
	}
	return buildImage(d.dockerClient, dir, noCache, d.build.Parameters.Output.DockerImageReference, d.tar, auth)
}

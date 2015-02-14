package builder

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/source-to-image/pkg/sti/git"
	"github.com/openshift/source-to-image/pkg/sti/tar"
)

// urlCheckTimeout is the timeout used to check the source URL
// If fetching the URL exceeds the timeout, then the build will
// not proceed further and stop
const urlCheckTimeout = 16 * time.Second

// imageRegex is used to substitute image names in buildconfigs with immutable image ids at build time.
var imageRegex = regexp.MustCompile(`^FROM\s+\w+.+`)

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
		git:          git.NewGit(),
		tar:          tar.NewTar(),
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
	if err = d.dockerBuild(buildDir); err != nil {
		return err
	}
	tag := d.build.Parameters.Output.DockerImageReference
	defer removeImage(d.dockerClient, tag)
	if len(d.build.Parameters.Output.DockerImageReference) != 0 {
		return pushImage(d.dockerClient, tag, d.auth)
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

// addBuildParameters checks if a BaseImage is set to replace the default base image.
// If that's the case then change the Dockerfile to make the build with the given image.
// Also append the environment variables in the Dockerfile.
func (d *DockerBuilder) addBuildParameters(dir string) error {
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if d.build.Parameters.Strategy.DockerStrategy != nil && len(d.build.Parameters.Strategy.DockerStrategy.ContextDir) > 0 {
		dockerfilePath = filepath.Join(dir, d.build.Parameters.Strategy.DockerStrategy.ContextDir, "Dockerfile")
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
	if d.build.Parameters.Strategy.DockerStrategy.BaseImage != "" {
		newFileData = imageRegex.ReplaceAllLiteralString(string(fileData), fmt.Sprintf("FROM %s", d.build.Parameters.Strategy.DockerStrategy.BaseImage))
	} else {
		newFileData = newFileData + string(fileData)
	}

	envVars := getBuildEnvVars(d.build)
	for k, v := range envVars {
		newFileData = newFileData + fmt.Sprintf("ENV %s %s\n", k, v)
	}

	if ioutil.WriteFile(dockerfilePath, []byte(newFileData), filePerm); err != nil {
		return err
	}

	return nil
}

// dockerBuild performs a docker build on the source that has been retrieved
func (d *DockerBuilder) dockerBuild(dir string) error {
	var noCache bool
	if d.build.Parameters.Strategy.DockerStrategy != nil {
		if d.build.Parameters.Strategy.DockerStrategy.ContextDir != "" {
			dir = filepath.Join(dir, d.build.Parameters.Strategy.DockerStrategy.ContextDir)
		}
		noCache = d.build.Parameters.Strategy.DockerStrategy.NoCache
	}
	return buildImage(d.dockerClient, dir, noCache, d.build.Parameters.Output.DockerImageReference, d.tar)
}

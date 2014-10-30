package builder

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/source-to-image/pkg/sti/git"
	"github.com/openshift/source-to-image/pkg/sti/tar"
)

// urlCheckTimeout is the timeout used to check the source URL
// If fetching the URL exceeeds the timeout, then the build will
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
		git:          git.NewGit(),
		tar:          tar.NewTar(false),
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
	if err = d.dockerBuild(buildDir); err != nil {
		return err
	}
	if err = d.addImageVars(); err != nil {
		return err
	}
	if d.build.Parameters.Output.Registry != "" || d.authPresent {
		return pushImage(d.dockerClient, imageTag(d.build), d.auth)
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

// dockerBuild performs a docker build on the source that has been retrieved
func (d *DockerBuilder) dockerBuild(dir string) error {
	if d.build.Parameters.Strategy.DockerStrategy != nil &&
		d.build.Parameters.Strategy.DockerStrategy.ContextDir != "" {
		dir = filepath.Join(dir, d.build.Parameters.Strategy.DockerStrategy.ContextDir)
	}
	return buildImage(d.dockerClient, dir, imageTag(d.build), d.tar)
}

// addImageVars creates a new Dockerfile which adds certain environment
// variables to the previously tagged image
func (d *DockerBuilder) addImageVars() error {
	envVars := getBuildEnvVars(d.build)
	tempDir, err := ioutil.TempDir("", "overlay")
	if err != nil {
		return err
	}
	overlay, err := os.Create(filepath.Join(tempDir, "Dockerfile"))
	if err != nil {
		return err
	}
	overlay.WriteString(fmt.Sprintf("FROM %s\n", imageTag(d.build)))
	for k, v := range envVars {
		overlay.WriteString(fmt.Sprintf("ENV %s %s\n", k, v))
	}
	if err = overlay.Close(); err != nil {
		return err
	}
	return buildImage(d.dockerClient, tempDir, imageTag(d.build), d.tar)
}

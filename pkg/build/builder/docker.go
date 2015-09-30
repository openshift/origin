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

	dockercmd "github.com/docker/docker/builder/command"
	"github.com/docker/docker/builder/parser"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/util/docker/dockerfile"
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
// to make sure that it is valid.  It also optionally tests the connection
// to the source uri.
func (d *DockerBuilder) checkSourceURI(testConnection bool) error {
	rawurl := d.build.Spec.Source.Git.URI
	if !d.git.ValidCloneSpec(rawurl) {
		return fmt.Errorf("Invalid git source url: %s", rawurl)
	}
	if strings.HasPrefix(rawurl, "git@") || strings.HasPrefix(rawurl, "git://") {
		return nil
	}
	srcURL, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	if !testConnection {
		return nil
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
	hasGitSource := false
	// TODO: refactor me into a method
	if gitSource := d.build.Spec.Source.Git; gitSource != nil {
		hasGitSource = true
		revision := d.build.Spec.Revision

		// Set the HTTP and HTTPS proxies to be used by git clone.
		originalProxies := setHTTPProxy(gitSource.HTTPProxy, gitSource.HTTPSProxy)
		defer resetHTTPProxy(originalProxies)

		// Check source URI, trying to connect to the server only if not using a proxy.
		usingProxy := len(originalProxies) > 0
		if err := d.checkSourceURI(!usingProxy); err != nil {
			return err
		}

		glog.V(2).Infof("Cloning source from %s", gitSource.URI)
		if err := d.git.Clone(gitSource.URI, dir, s2iapi.CloneConfig{Recursive: true, Quiet: true}); err != nil {
			return err
		}

		// if we specify a commit, ref, or branch to checkout, do so
		if len(gitSource.Ref) != 0 || (revision != nil && revision.Git != nil && len(revision.Git.Commit) != 0) {
			commit := gitSource.Ref
			if revision != nil && revision.Git != nil && revision.Git.Commit != "" {
				commit = revision.Git.Commit
			}
			if err := d.git.Checkout(dir, commit); err != nil {
				return err
			}
		}
	}

	// a Dockerfile has been specified, create or overwrite into the destination
	if dockerfileSource := d.build.Spec.Source.Dockerfile; dockerfileSource != nil {
		baseDir := dir
		// if a context dir has been defined and we cloned source, overwrite the destination
		if hasGitSource && len(d.build.Spec.Source.ContextDir) != 0 {
			baseDir = filepath.Join(baseDir, d.build.Spec.Source.ContextDir)
		}
		return ioutil.WriteFile(filepath.Join(baseDir, "Dockerfile"), []byte(*dockerfileSource), 0660)
	}

	return nil
}

// addBuildParameters checks if a Image is set to replace the default base image.
// If that's the case then change the Dockerfile to make the build with the given image.
// Also append the environment variables and labels in the Dockerfile.
func (d *DockerBuilder) addBuildParameters(dir string) error {
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if d.build.Spec.Strategy.DockerStrategy != nil && len(d.build.Spec.Source.ContextDir) > 0 {
		dockerfilePath = filepath.Join(dir, d.build.Spec.Source.ContextDir, "Dockerfile")
	}

	f, err := os.Open(dockerfilePath)
	if err != nil {
		return err
	}

	// Parse the Dockerfile.
	node, err := parser.Parse(f)
	if err != nil {
		return err
	}

	// Update base image if build strategy specifies the From field.
	if d.build.Spec.Strategy.DockerStrategy.From != nil && d.build.Spec.Strategy.DockerStrategy.From.Kind == "DockerImage" {
		err := replaceLastFrom(node, d.build.Spec.Strategy.DockerStrategy.From.Name)
		if err != nil {
			return err
		}
	}

	// Append build info as environment variables.
	err = appendEnv(node, d.buildInfo())
	if err != nil {
		return err
	}

	// Append build labels.
	err = appendLabel(node, d.buildLabels(dir))
	if err != nil {
		return err
	}

	// Insert environment variables defined in the build strategy.
	err = insertEnvAfterFrom(node, d.build.Spec.Strategy.DockerStrategy.Env)
	if err != nil {
		return err
	}

	instructions := dockerfile.ParseTreeToDockerfile(node)

	// Overwrite the Dockerfile.
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dockerfilePath, instructions, fi.Mode())
}

// buildInfo converts the buildInfo output to a format that appendEnv can
// consume.
func (d *DockerBuilder) buildInfo() []dockerfile.KeyValue {
	bi := buildInfo(d.build)
	kv := make([]dockerfile.KeyValue, len(bi))
	for i, item := range bi {
		kv[i] = dockerfile.KeyValue{Key: item.Key, Value: item.Value}
	}
	return kv
}

// buildLabels returns a slice of KeyValue pairs in a format that appendEnv can
// consume.
func (d *DockerBuilder) buildLabels(dir string) []dockerfile.KeyValue {
	labels := map[string]string{}
	sourceInfo := &s2iapi.SourceInfo{}
	if d.build.Spec.Source.Git != nil {
		sourceInfo = d.git.GetInfo(dir)
	}
	if len(d.build.Spec.Source.ContextDir) > 0 {
		sourceInfo.ContextDir = d.build.Spec.Source.ContextDir
	}
	labels = util.GenerateLabelsFromSourceInfo(labels, sourceInfo, api.DefaultDockerLabelNamespace)
	kv := make([]dockerfile.KeyValue, 0, len(labels))
	for k, v := range labels {
		kv = append(kv, dockerfile.KeyValue{Key: k, Value: v})
	}
	return kv
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
	auth, err := d.setupPullSecret()
	if err != nil {
		return err
	}
	return buildImage(d.dockerClient, dir, noCache, d.build.Spec.Output.To.Name, d.tar, auth, forcePull)
}

// replaceLastFrom changes the last FROM instruction of node to point to the
// base image image.
func replaceLastFrom(node *parser.Node, image string) error {
	if node == nil {
		return nil
	}
	for i := len(node.Children) - 1; i >= 0; i-- {
		child := node.Children[i]
		if child != nil && child.Value == dockercmd.From {
			from, err := dockerfile.From(image)
			if err != nil {
				return err
			}
			fromTree, err := parser.Parse(strings.NewReader(from))
			if err != nil {
				return err
			}
			node.Children[i] = fromTree.Children[0]
			return nil
		}
	}
	return nil
}

// appendEnv appends an ENV Dockerfile instruction as the last child of node
// with keys and values from m.
func appendEnv(node *parser.Node, m []dockerfile.KeyValue) error {
	return appendKeyValueInstruction(dockerfile.Env, node, m)
}

// appendLabel appends a LABEL Dockerfile instruction as the last child of node
// with keys and values from m.
func appendLabel(node *parser.Node, m []dockerfile.KeyValue) error {
	if len(m) == 0 {
		return nil
	}
	return appendKeyValueInstruction(dockerfile.Label, node, m)
}

// appendKeyValueInstruction is a primitive used to avoid code duplication.
// Callers should use a derivative of this such as appendEnv or appendLabel.
// appendKeyValueInstruction appends a Dockerfile instruction with key-value
// syntax created by f as the last child of node with keys and values from m.
func appendKeyValueInstruction(f func([]dockerfile.KeyValue) (string, error), node *parser.Node, m []dockerfile.KeyValue) error {
	if node == nil {
		return nil
	}
	instruction, err := f(m)
	if err != nil {
		return err
	}
	return dockerfile.InsertInstructions(node, len(node.Children), instruction)
}

// insertEnvAfterFrom inserts an ENV instruction with the environment variables
// from env after every FROM instruction in node.
func insertEnvAfterFrom(node *parser.Node, env []kapi.EnvVar) error {
	if node == nil || len(env) == 0 {
		return nil
	}

	// Build ENV instruction.
	var m []dockerfile.KeyValue
	for _, e := range env {
		m = append(m, dockerfile.KeyValue{Key: e.Name, Value: e.Value})
	}
	buildEnv, err := dockerfile.Env(m)
	if err != nil {
		return err
	}

	// Insert the buildEnv after every FROM instruction.
	// We iterate in reverse order, otherwise indices would have to be
	// recomputed after each step, because we're changing node in-place.
	indices := dockerfile.FindAll(node, dockercmd.From)
	for i := len(indices) - 1; i >= 0; i-- {
		err := dockerfile.InsertInstructions(node, indices[i]+1, buildEnv)
		if err != nil {
			return err
		}
	}

	return nil
}

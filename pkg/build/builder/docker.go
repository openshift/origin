package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	dockercmd "github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	docker "github.com/fsouza/go-dockerclient"
	kapi "k8s.io/kubernetes/pkg/api"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/generate/git"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/docker/dockerfile"
)

// defaultDockerfilePath is the default path of the Dockerfile
const defaultDockerfilePath = "Dockerfile"

// DockerBuilder builds Docker images given a git repository URL
type DockerBuilder struct {
	dockerClient DockerClient
	gitClient    GitClient
	tar          tar.Tar
	build        *api.Build
	urlTimeout   time.Duration
	client       client.BuildInterface
	cgLimits     *s2iapi.CGroupLimits
}

// NewDockerBuilder creates a new instance of DockerBuilder
func NewDockerBuilder(dockerClient DockerClient, buildsClient client.BuildInterface, build *api.Build, gitClient GitClient, cgLimits *s2iapi.CGroupLimits) *DockerBuilder {
	return &DockerBuilder{
		dockerClient: dockerClient,
		build:        build,
		gitClient:    gitClient,
		tar:          tar.New(),
		urlTimeout:   initialURLCheckTimeout,
		client:       buildsClient,
		cgLimits:     cgLimits,
	}
}

// Build executes a Docker build
func (d *DockerBuilder) Build() error {
	if d.build.Spec.Source.Git == nil && d.build.Spec.Source.Binary == nil &&
		d.build.Spec.Source.Dockerfile == nil && d.build.Spec.Source.Images == nil {
		return fmt.Errorf("must provide a value for at least one of source, binary, images, or dockerfile")
	}
	var push bool
	pushTag := d.build.Status.OutputDockerImageReference

	buildDir, err := ioutil.TempDir("", "docker-build")
	if err != nil {
		return err
	}
	sourceInfo, err := fetchSource(d.dockerClient, buildDir, d.build, d.urlTimeout, os.Stdin, d.gitClient)
	if err != nil {
		return err
	}
	if sourceInfo != nil {
		updateBuildRevision(d.client, d.build, sourceInfo)
	}
	if err := d.addBuildParameters(buildDir); err != nil {
		return err
	}

	glog.V(4).Infof("Starting Docker build from build config %s ...", d.build.Name)
	// if there is no output target, set one up so the docker build logic
	// (which requires a tag) will still work, but we won't push it at the end.
	if d.build.Spec.Output.To == nil || len(d.build.Spec.Output.To.Name) == 0 {
		d.build.Status.OutputDockerImageReference = d.build.Name
	} else {
		push = true
	}

	buildTag := randomBuildTag(d.build.Namespace, d.build.Name)
	dockerfilePath := d.getDockerfilePath(buildDir)
	imageNames := getDockerfileFrom(dockerfilePath)
	if len(imageNames) == 0 {
		return fmt.Errorf("no from image in dockerfile.")
	}
	for _, imageName := range imageNames {
		var imageExists bool = true
		_, err = d.dockerClient.InspectImage(imageName)
		if err != nil {
			if err != docker.ErrNoSuchImage {
				return err
			}
			imageExists = false
		}
		// if forcePull or the image not exists on the node we should pull the image first
		if d.build.Spec.Strategy.DockerStrategy.ForcePull || !imageExists {
			pullAuthConfig, _ := dockercfg.NewHelper().GetDockerAuth(
				imageName,
				dockercfg.PullAuthType,
			)
			glog.V(0).Infof("\nPulling image %s ...", imageName)
			if err := pullImage(d.dockerClient, imageName, pullAuthConfig); err != nil {
				return fmt.Errorf("failed to pull image: %v", err)
			}
		}
	}

	if err := d.dockerBuild(buildDir, buildTag, d.build.Spec.Source.Secrets); err != nil {
		return err
	}

	cname := containerName("docker", d.build.Name, d.build.Namespace, "post-commit")
	if err := execPostCommitHook(d.dockerClient, d.build.Spec.PostCommit, buildTag, cname); err != nil {
		return err
	}

	if push {
		if err := tagImage(d.dockerClient, buildTag, pushTag); err != nil {
			return err
		}
	}

	if err := removeImage(d.dockerClient, buildTag); err != nil {
		glog.V(0).Infof("warning: Failed to remove temporary build tag %v: %v", buildTag, err)
	}

	if push {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			pushTag,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.V(4).Infof("Authenticating Docker push with user %q", pushAuthConfig.Username)
		}
		glog.V(0).Infof("\nPushing image %s ...", pushTag)
		if err := pushImage(d.dockerClient, pushTag, pushAuthConfig); err != nil {
			return reportPushFailure(err, authPresent, pushAuthConfig)
		}
		glog.V(0).Infof("Push successful")
	}
	return nil
}

// copySecrets copies all files from the directory where the secret is
// mounted in the builder pod to a directory where the is the Dockerfile, so
// users can ADD or COPY the files inside their Dockerfile.
func (d *DockerBuilder) copySecrets(secrets []api.SecretBuildSource, buildDir string) error {
	for _, s := range secrets {
		dstDir := filepath.Join(buildDir, s.DestinationDir)
		if err := os.MkdirAll(dstDir, 0777); err != nil {
			return err
		}
		srcDir := filepath.Join(strategy.SecretBuildSourceBaseMountPath, s.Secret.Name)
		glog.V(3).Infof("Copying files from the build secret %q to %q", s.Secret.Name, filepath.Clean(s.DestinationDir))
		out, err := exec.Command("cp", "-vrf", srcDir+"/.", dstDir+"/").Output()
		if err != nil {
			glog.V(4).Infof("Secret %q failed to copy: %q", s.Secret.Name, string(out))
			return err
		}
		// See what is copied where when debugging.
		glog.V(5).Infof(string(out))
	}
	return nil
}

// addBuildParameters checks if a Image is set to replace the default base image.
// If that's the case then change the Dockerfile to make the build with the given image.
// Also append the environment variables and labels in the Dockerfile.
func (d *DockerBuilder) addBuildParameters(dir string) error {
	dockerfilePath := d.getDockerfilePath(dir)
	node, err := parseDockerfile(dockerfilePath)
	if err != nil {
		return err
	}

	// Update base image if build strategy specifies the From field.
	if d.build.Spec.Strategy.DockerStrategy.From != nil && d.build.Spec.Strategy.DockerStrategy.From.Kind == "DockerImage" {
		// Reduce the name to a minimal canonical form for the daemon
		name := d.build.Spec.Strategy.DockerStrategy.From.Name
		if ref, err := imageapi.ParseDockerImageReference(name); err == nil {
			name = ref.DaemonMinimal().Exact()
		}
		err := replaceLastFrom(node, name)
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
	fi, err := os.Stat(dockerfilePath)
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
	// TODO: allow source info to be overridden by build
	sourceInfo := &git.SourceInfo{}
	if d.build.Spec.Source.Git != nil {
		var errors []error
		sourceInfo, errors = d.gitClient.GetInfo(dir)
		if len(errors) > 0 {
			for _, e := range errors {
				glog.V(0).Infof("warning: Unable to retrieve Git info: %v", e.Error())
			}
		}
	}
	if len(d.build.Spec.Source.ContextDir) > 0 {
		sourceInfo.ContextDir = d.build.Spec.Source.ContextDir
	}
	labels = util.GenerateLabelsFromSourceInfo(labels, &sourceInfo.SourceInfo, api.DefaultDockerLabelNamespace)
	kv := make([]dockerfile.KeyValue, 0, len(labels)+len(d.build.Spec.Output.ImageLabels))
	for k, v := range labels {
		kv = append(kv, dockerfile.KeyValue{Key: k, Value: v})
	}
	// override autogenerated labels
	for _, lbl := range d.build.Spec.Output.ImageLabels {
		kv = append(kv, dockerfile.KeyValue{Key: lbl.Name, Value: lbl.Value})
	}
	return kv
}

// setupPullSecret provides a Docker authentication configuration when the
// PullSecret is specified.
func (d *DockerBuilder) setupPullSecret() (*docker.AuthConfigurations, error) {
	if len(os.Getenv(dockercfg.PullAuthType)) == 0 {
		return nil, nil
	}
	glog.V(0).Infof("Checking for Docker config file for %s in path %s", dockercfg.PullAuthType, os.Getenv(dockercfg.PullAuthType))
	dockercfgPath := dockercfg.GetDockercfgFile(os.Getenv(dockercfg.PullAuthType))
	if len(dockercfgPath) == 0 {
		return nil, fmt.Errorf("no docker config file found in '%s'", os.Getenv(dockercfg.PullAuthType))
	}
	glog.V(0).Infof("Using Docker config file %s", dockercfgPath)
	r, err := os.Open(dockercfgPath)
	if err != nil {
		return nil, fmt.Errorf("'%s': %s", dockercfgPath, err)
	}
	return docker.NewAuthConfigurations(r)

}

// dockerBuild performs a docker build on the source that has been retrieved
func (d *DockerBuilder) dockerBuild(dir string, tag string, secrets []api.SecretBuildSource) error {
	var noCache bool
	var forcePull bool
	dockerfilePath := defaultDockerfilePath
	if d.build.Spec.Strategy.DockerStrategy != nil {
		if d.build.Spec.Source.ContextDir != "" {
			dir = filepath.Join(dir, d.build.Spec.Source.ContextDir)
		}
		if d.build.Spec.Strategy.DockerStrategy.DockerfilePath != "" {
			dockerfilePath = d.build.Spec.Strategy.DockerStrategy.DockerfilePath
		}
		noCache = d.build.Spec.Strategy.DockerStrategy.NoCache
		forcePull = d.build.Spec.Strategy.DockerStrategy.ForcePull
	}
	auth, err := d.setupPullSecret()
	if err != nil {
		return err
	}
	if err := d.copySecrets(secrets, dir); err != nil {
		return err
	}

	opts := docker.BuildImageOptions{
		Name:           tag,
		RmTmpContainer: true,
		OutputStream:   os.Stdout,
		Dockerfile:     dockerfilePath,
		NoCache:        noCache,
		Pull:           forcePull,
	}
	if d.cgLimits != nil {
		opts.Memory = d.cgLimits.MemoryLimitBytes
		opts.Memswap = d.cgLimits.MemorySwap
		opts.CPUShares = d.cgLimits.CPUShares
		opts.CPUPeriod = d.cgLimits.CPUPeriod
		opts.CPUQuota = d.cgLimits.CPUQuota
	}
	if auth != nil {
		opts.AuthConfigs = *auth
	}

	return buildImage(d.dockerClient, dir, d.tar, &opts)
}

func (d *DockerBuilder) getDockerfilePath(dir string) string {
	var contextDirPath string
	if d.build.Spec.Strategy.DockerStrategy != nil && len(d.build.Spec.Source.ContextDir) > 0 {
		contextDirPath = filepath.Join(dir, d.build.Spec.Source.ContextDir)
	} else {
		contextDirPath = dir
	}

	var dockerfilePath string
	if d.build.Spec.Strategy.DockerStrategy != nil && len(d.build.Spec.Strategy.DockerStrategy.DockerfilePath) > 0 {
		dockerfilePath = filepath.Join(contextDirPath, d.build.Spec.Strategy.DockerStrategy.DockerfilePath)
	} else {
		dockerfilePath = filepath.Join(contextDirPath, defaultDockerfilePath)
	}
	return dockerfilePath
}
func parseDockerfile(dockerfilePath string) (*parser.Node, error) {
	f, err := os.Open(dockerfilePath)
	defer f.Close()
	if err != nil {
		return nil, err
	}

	// Parse the Dockerfile.
	node, err := parser.Parse(f)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// replaceLastFrom changes the last FROM instruction of node to point to the
// base image.
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

// getDockerfilefrom returns all the images behind "FROM" instruction in the dockerfile
func getDockerfileFrom(dockerfilePath string) []string {
	var froms []string
	if "" == dockerfilePath {
		return froms
	}
	node, err := parseDockerfile(dockerfilePath)
	if err != nil {
		return froms
	}
	for i := 0; i < len(node.Children); i++ {
		child := node.Children[i]
		if child == nil || child.Value != dockercmd.From {
			continue
		}
		from := child.Next.Value
		if len(from) > 0 {
			froms = append(froms, from)
		}
	}
	return froms
}

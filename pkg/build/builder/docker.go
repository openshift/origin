package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	dockercmd "github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	docker "github.com/fsouza/go-dockerclient"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/tar"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/builder/timing"
	builderutil "github.com/openshift/origin/pkg/build/builder/util"
	"github.com/openshift/origin/pkg/build/builder/util/dockerfile"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// defaultDockerfilePath is the default path of the Dockerfile
const defaultDockerfilePath = "Dockerfile"

// DockerBuilder builds Docker images given a git repository URL
type DockerBuilder struct {
	dockerClient DockerClient
	tar          tar.Tar
	build        *buildapiv1.Build
	client       buildclientv1.BuildInterface
	cgLimits     *s2iapi.CGroupLimits
	inputDir     string
}

// NewDockerBuilder creates a new instance of DockerBuilder
func NewDockerBuilder(dockerClient DockerClient, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) *DockerBuilder {
	return &DockerBuilder{
		dockerClient: dockerClient,
		build:        build,
		tar:          tar.New(s2ifs.NewFileSystem()),
		client:       buildsClient,
		cgLimits:     cgLimits,
		inputDir:     buildutil.InputContentPath,
	}
}

// Build executes a Docker build
func (d *DockerBuilder) Build() error {

	var err error
	ctx := timing.NewContext(context.Background())
	defer func() {
		d.build.Status.Stages = timing.AppendStageAndStepInfo(d.build.Status.Stages, timing.GetStages(ctx))
		HandleBuildStatusUpdate(d.build, d.client, nil)
	}()

	if d.build.Spec.Source.Git == nil && d.build.Spec.Source.Binary == nil &&
		d.build.Spec.Source.Dockerfile == nil && d.build.Spec.Source.Images == nil {
		return fmt.Errorf("must provide a value for at least one of source, binary, images, or dockerfile")
	}
	var push bool
	pushTag := d.build.Status.OutputDockerImageReference

	// this is where the git-fetch container put the code during the clone operation
	buildDir := d.inputDir

	glog.V(4).Infof("Starting Docker build from build config %s ...", d.build.Name)
	// if there is no output target, set one up so the docker build logic
	// (which requires a tag) will still work, but we won't push it at the end.
	if d.build.Spec.Output.To == nil || len(d.build.Spec.Output.To.Name) == 0 {
		d.build.Status.OutputDockerImageReference = d.build.Name
	} else {
		push = true
	}

	buildTag := randomBuildTag(d.build.Namespace, d.build.Name)
	dockerfilePath := getDockerfilePath(buildDir, d.build)

	imageNames, multiStage, err := findReferencedImages(dockerfilePath)
	if err != nil {
		return err
	}
	if len(imageNames) == 0 {
		return fmt.Errorf("no FROM image in Dockerfile")
	}
	for _, imageName := range imageNames {
		if imageName == "scratch" {
			glog.V(4).Infof("\nSkipping image \"scratch\"")
			continue
		}
		imageExists := true
		_, err = d.dockerClient.InspectImage(imageName)
		if err != nil {
			if err != docker.ErrNoSuchImage {
				return err
			}
			imageExists = false
		}
		// if forcePull or the image does not exist on the node we should pull the image first
		if d.build.Spec.Strategy.DockerStrategy.ForcePull || !imageExists {
			pullAuthConfig, _ := dockercfg.NewHelper().GetDockerAuth(
				imageName,
				dockercfg.PullAuthType,
			)
			glog.V(0).Infof("\nPulling image %s ...", imageName)
			startTime := metav1.Now()
			err = pullImage(d.dockerClient, imageName, pullAuthConfig)

			timing.RecordNewStep(ctx, buildapiv1.StagePullImages, buildapiv1.StepPullBaseImage, startTime, metav1.Now())

			if err != nil {
				d.build.Status.Phase = buildapiv1.BuildPhaseFailed
				d.build.Status.Reason = buildapiv1.StatusReasonPullBuilderImageFailed
				d.build.Status.Message = builderutil.StatusMessagePullBuilderImageFailed
				HandleBuildStatusUpdate(d.build, d.client, nil)
				return fmt.Errorf("failed to pull image: %v", err)
			}

		}
	}

	if s := d.build.Spec.Strategy.DockerStrategy; s != nil && multiStage {
		if s.ImageOptimizationPolicy == nil {
			policy := buildapiv1.ImageOptimizationSkipLayers
			s.ImageOptimizationPolicy = &policy
			glog.V(2).Infof("Detected multi-stage Dockerfile, image will be built with imageOptimizationPolicy set to SkipLayers")
		}
	}

	startTime := metav1.Now()
	err = d.dockerBuild(buildDir, buildTag, d.build.Spec.Source.Secrets)

	timing.RecordNewStep(ctx, buildapiv1.StageBuild, buildapiv1.StepDockerBuild, startTime, metav1.Now())

	if err != nil {
		d.build.Status.Phase = buildapiv1.BuildPhaseFailed
		d.build.Status.Reason = buildapiv1.StatusReasonDockerBuildFailed
		d.build.Status.Message = builderutil.StatusMessageDockerBuildFailed
		HandleBuildStatusUpdate(d.build, d.client, nil)
		return err
	}

	cname := containerName("docker", d.build.Name, d.build.Namespace, "post-commit")

	err = execPostCommitHook(ctx, d.dockerClient, d.build.Spec.PostCommit, buildTag, cname)

	if err != nil {
		d.build.Status.Phase = buildapiv1.BuildPhaseFailed
		d.build.Status.Reason = buildapiv1.StatusReasonPostCommitHookFailed
		d.build.Status.Message = builderutil.StatusMessagePostCommitHookFailed
		HandleBuildStatusUpdate(d.build, d.client, nil)
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
		startTime = metav1.Now()
		digest, err := pushImage(d.dockerClient, pushTag, pushAuthConfig)

		timing.RecordNewStep(ctx, buildapiv1.StagePushImage, buildapiv1.StepPushDockerImage, startTime, metav1.Now())

		if err != nil {
			d.build.Status.Phase = buildapiv1.BuildPhaseFailed
			d.build.Status.Reason = buildapiv1.StatusReasonPushImageToRegistryFailed
			d.build.Status.Message = builderutil.StatusMessagePushImageToRegistryFailed
			HandleBuildStatusUpdate(d.build, d.client, nil)
			return reportPushFailure(err, authPresent, pushAuthConfig)
		}

		if len(digest) > 0 {
			d.build.Status.Output.To = &buildapiv1.BuildStatusOutputTo{
				ImageDigest: digest,
			}
			HandleBuildStatusUpdate(d.build, d.client, nil)
		}
		glog.V(0).Infof("Push successful")
	}
	return nil
}

// copySecrets copies all files from the directory where the secret is
// mounted in the builder pod to a directory where the is the Dockerfile, so
// users can ADD or COPY the files inside their Dockerfile.
func (d *DockerBuilder) copySecrets(secrets []buildapiv1.SecretBuildSource, buildDir string) error {
	for _, s := range secrets {
		dstDir := filepath.Join(buildDir, s.DestinationDir)
		if err := os.MkdirAll(dstDir, 0777); err != nil {
			return err
		}
		glog.V(3).Infof("Copying files from the build secret %q to %q", s.Secret.Name, dstDir)

		// Secrets contain nested directories and fairly baroque links. To prevent extra data being
		// copied, perform the following steps:
		//
		// 1. Only top level files and directories within the secret directory are candidates
		// 2. Any item starting with '..' is ignored
		// 3. Destination directories are created first with 0777
		// 4. Use the '-L' option to cp to copy only contents.
		//
		srcDir := filepath.Join(strategy.SecretBuildSourceBaseMountPath, s.Secret.Name)
		if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if srcDir == path {
				return nil
			}

			// skip any contents that begin with ".."
			if strings.HasPrefix(filepath.Base(path), "..") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// ensure all directories are traversable
			if info.IsDir() {
				if err := os.MkdirAll(dstDir, 0777); err != nil {
					return err
				}
			}
			out, err := exec.Command("cp", "-vLRf", path, dstDir+"/").Output()
			if err != nil {
				glog.V(4).Infof("Secret %q failed to copy: %q", s.Secret.Name, string(out))
				return err
			}
			// See what is copied when debugging.
			glog.V(5).Infof("Result of secret copy %s\n%s", s.Secret.Name, string(out))
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
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
func (d *DockerBuilder) dockerBuild(dir string, tag string, secrets []buildapiv1.SecretBuildSource) error {
	var noCache bool
	var forcePull bool
	var buildArgs []docker.BuildArg
	dockerfilePath := defaultDockerfilePath
	if d.build.Spec.Strategy.DockerStrategy != nil {
		if d.build.Spec.Source.ContextDir != "" {
			dir = filepath.Join(dir, d.build.Spec.Source.ContextDir)
		}
		if d.build.Spec.Strategy.DockerStrategy.DockerfilePath != "" {
			dockerfilePath = d.build.Spec.Strategy.DockerStrategy.DockerfilePath
		}
		for _, ba := range d.build.Spec.Strategy.DockerStrategy.BuildArgs {
			buildArgs = append(buildArgs, docker.BuildArg{Name: ba.Name, Value: ba.Value})
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
		Name:                tag,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		OutputStream:        os.Stdout,
		Dockerfile:          dockerfilePath,
		NoCache:             noCache,
		Pull:                forcePull,
		BuildArgs:           buildArgs,
	}
	network, resolvConfHostPath, err := getContainerNetworkConfig()
	if err != nil {
		return err
	}
	opts.NetworkMode = network
	if len(resolvConfHostPath) != 0 {
		cmd := exec.Command("chcon", "system_u:object_r:svirt_sandbox_file_t:s0", "/etc/resolv.conf")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("unable to set permissions on /etc/resolv.conf: %v", err)
		}
		opts.BuildBinds = fmt.Sprintf("[\"%s:/etc/resolv.conf\"]", resolvConfHostPath)
	}
	// Though we are capped on memory and cpu at the cgroup parent level,
	// some build containers care what their memory limit is so they can
	// adapt, thus we need to set the memory limit at the container level
	// too, so that information is available to them.
	if d.cgLimits != nil {
		opts.Memory = d.cgLimits.MemoryLimitBytes
		opts.Memswap = d.cgLimits.MemorySwap
		opts.CgroupParent = d.cgLimits.Parent
	}

	if auth != nil {
		opts.AuthConfigs = *auth
	}

	if s := d.build.Spec.Strategy.DockerStrategy; s != nil {
		if policy := s.ImageOptimizationPolicy; policy != nil {
			switch *policy {
			case buildapiv1.ImageOptimizationSkipLayers:
				return buildDirectImage(dir, false, &opts)
			case buildapiv1.ImageOptimizationSkipLayersAndWarn:
				return buildDirectImage(dir, true, &opts)
			}
		}
	}

	return buildImage(d.dockerClient, dir, d.tar, &opts)
}

func getDockerfilePath(dir string, build *buildapiv1.Build) string {
	var contextDirPath string
	if build.Spec.Strategy.DockerStrategy != nil && len(build.Spec.Source.ContextDir) > 0 {
		contextDirPath = filepath.Join(dir, build.Spec.Source.ContextDir)
	} else {
		contextDirPath = dir
	}

	var dockerfilePath string
	if build.Spec.Strategy.DockerStrategy != nil && len(build.Spec.Strategy.DockerStrategy.DockerfilePath) > 0 {
		dockerfilePath = filepath.Join(contextDirPath, build.Spec.Strategy.DockerStrategy.DockerfilePath)
	} else {
		dockerfilePath = filepath.Join(contextDirPath, defaultDockerfilePath)
	}
	return dockerfilePath
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
			if child.Next == nil {
				child.Next = &parser.Node{}
			}
			child.Next.Value = image
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
func insertEnvAfterFrom(node *parser.Node, env []corev1.EnvVar) error {
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

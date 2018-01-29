package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	bld "github.com/openshift/origin/pkg/build/builder"
	"github.com/openshift/origin/pkg/build/builder/cmd/scmauth"
	"github.com/openshift/origin/pkg/build/builder/timing"
	builderutil "github.com/openshift/origin/pkg/build/builder/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/git"
	"github.com/openshift/origin/pkg/version"
)

type builder interface {
	Build(dockerClient bld.DockerClient, sock string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) error
}

type builderConfig struct {
	out             io.Writer
	build           *buildapiv1.Build
	sourceSecretDir string
	dockerClient    *docker.Client
	dockerEndpoint  string
	buildsClient    buildclientv1.BuildInterface
}

func newBuilderConfigFromEnvironment(out io.Writer, needsDocker bool) (*builderConfig, error) {
	cfg := &builderConfig{}
	var err error

	cfg.out = out

	buildStr := os.Getenv("BUILD")

	cfg.build = &buildapiv1.Build{}

	obj, groupVersionKind, err := legacyscheme.Codecs.UniversalDecoder().Decode([]byte(buildStr), nil, cfg.build)
	if err != nil {
		return nil, fmt.Errorf("unable to parse build string: %v", err)
	}
	ok := false
	cfg.build, ok = obj.(*buildapiv1.Build)
	if !ok {
		return nil, fmt.Errorf("build string %s is not a build: %#v", buildStr, obj)
	}
	if glog.V(4) {
		redactedBuild := buildutil.SafeForLoggingBuild(cfg.build)
		if err != nil {
			return nil, fmt.Errorf("unable to strip proxy credentials from build: %v", err)
		}
		bytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(groupVersionKind.GroupVersion()), redactedBuild)
		if err != nil {
			return nil, fmt.Errorf("unable to serialize build: %v", err)
		}
		glog.V(4).Infof("redacted build: %v", string(bytes))
	}

	masterVersion := os.Getenv(builderutil.OriginVersion)
	thisVersion := version.Get().String()
	if len(masterVersion) != 0 && masterVersion != thisVersion {
		glog.V(3).Infof("warning: OpenShift server version %q differs from this image %q\n", masterVersion, thisVersion)
	} else {
		glog.V(4).Infof("Master version %q, Builder version %q", masterVersion, thisVersion)
	}

	// sourceSecretsDir (SOURCE_SECRET_PATH)
	cfg.sourceSecretDir = os.Getenv("SOURCE_SECRET_PATH")

	if needsDocker {
		// dockerClient and dockerEndpoint (DOCKER_HOST)
		// usually not set, defaults to docker socket
		cfg.dockerClient, cfg.dockerEndpoint, err = bld.GetDockerClient()
		if err != nil {
			return nil, fmt.Errorf("no Docker configuration defined: %v", err)
		}
	}

	// buildsClient (KUBERNETES_SERVICE_HOST, KUBERNETES_SERVICE_PORT)
	clientConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to the server: %v", err)
	}
	buildsClient, err := buildclientv1.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}
	cfg.buildsClient = buildsClient.Builds(cfg.build.Namespace)

	return cfg, nil
}

func (c *builderConfig) setupGitEnvironment() (string, []string, error) {

	// For now, we only handle git. If not specified, we're done
	gitSource := c.build.Spec.Source.Git
	if gitSource == nil {
		return "", []string{}, nil
	}

	sourceSecret := c.build.Spec.Source.SourceSecret
	gitEnv := []string{"GIT_ASKPASS=true"}
	// If a source secret is present, set it up and add its environment variables
	if sourceSecret != nil {
		// TODO: this should be refactored to let each source type manage which secrets
		// it accepts
		sourceURL, err := s2igit.Parse(gitSource.URI)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse build URL: %s", gitSource.URI)
		}
		scmAuths := scmauth.GitAuths(sourceURL)

		secretsEnv, overrideURL, err := scmAuths.Setup(c.sourceSecretDir)
		if err != nil {
			return c.sourceSecretDir, nil, fmt.Errorf("cannot setup source secret: %v", err)
		}
		if overrideURL != nil {
			gitSource.URI = overrideURL.String()
		}
		gitEnv = append(gitEnv, secretsEnv...)
	}
	if gitSource.HTTPProxy != nil && len(*gitSource.HTTPProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("HTTP_PROXY=%s", *gitSource.HTTPProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("http_proxy=%s", *gitSource.HTTPProxy))
	}
	if gitSource.HTTPSProxy != nil && len(*gitSource.HTTPSProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("HTTPS_PROXY=%s", *gitSource.HTTPSProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("https_proxy=%s", *gitSource.HTTPSProxy))
	}
	if gitSource.NoProxy != nil && len(*gitSource.NoProxy) > 0 {
		gitEnv = append(gitEnv, fmt.Sprintf("NO_PROXY=%s", *gitSource.NoProxy))
		gitEnv = append(gitEnv, fmt.Sprintf("no_proxy=%s", *gitSource.NoProxy))
	}
	return c.sourceSecretDir, bld.MergeEnv(os.Environ(), gitEnv), nil
}

// clone is responsible for cloning the source referenced in the buildconfig
func (c *builderConfig) clone() error {
	ctx := timing.NewContext(context.Background())
	var sourceRev *buildapiv1.SourceRevision
	defer func() {
		c.build.Status.Stages = timing.GetStages(ctx)
		bld.HandleBuildStatusUpdate(c.build, c.buildsClient, sourceRev)
	}()
	secretTmpDir, gitEnv, err := c.setupGitEnvironment()
	if err != nil {
		return err
	}
	defer os.RemoveAll(secretTmpDir)

	gitClient := git.NewRepositoryWithEnv(gitEnv)

	buildDir := buildutil.InputContentPath
	sourceInfo, err := bld.GitClone(ctx, gitClient, c.build.Spec.Source.Git, c.build.Spec.Revision, buildDir)
	if err != nil {
		c.build.Status.Phase = buildapiv1.BuildPhaseFailed
		c.build.Status.Reason = buildapiv1.StatusReasonFetchSourceFailed
		c.build.Status.Message = builderutil.StatusMessageFetchSourceFailed
		return err
	}

	if sourceInfo != nil {
		sourceRev = bld.GetSourceRevision(c.build, sourceInfo)
	}

	err = bld.ExtractInputBinary(os.Stdin, c.build.Spec.Source.Binary, buildDir)
	if err != nil {
		c.build.Status.Phase = buildapiv1.BuildPhaseFailed
		c.build.Status.Reason = buildapiv1.StatusReasonFetchSourceFailed
		c.build.Status.Message = builderutil.StatusMessageFetchSourceFailed
		return err
	}

	if len(c.build.Spec.Source.ContextDir) > 0 {
		if _, err := os.Stat(filepath.Join(buildDir, c.build.Spec.Source.ContextDir)); os.IsNotExist(err) {
			err = fmt.Errorf("provided context directory does not exist: %s", c.build.Spec.Source.ContextDir)
			c.build.Status.Phase = buildapiv1.BuildPhaseFailed
			c.build.Status.Reason = buildapiv1.StatusReasonInvalidContextDirectory
			c.build.Status.Message = builderutil.StatusMessageInvalidContextDirectory
			return err
		}
	}

	return nil
}

func (c *builderConfig) extractImageContent() error {
	ctx := timing.NewContext(context.Background())
	defer func() {
		c.build.Status.Stages = timing.GetStages(ctx)
		bld.HandleBuildStatusUpdate(c.build, c.buildsClient, nil)
	}()

	buildDir := buildutil.InputContentPath
	return bld.ExtractImageContent(ctx, c.dockerClient, buildDir, c.build)
}

// execute is responsible for running a build
func (c *builderConfig) execute(b builder) error {
	cgLimits, err := bld.GetCGroupLimits()
	if err != nil {
		return fmt.Errorf("failed to retrieve cgroup limits: %v", err)
	}
	glog.V(4).Infof("Running build with cgroup limits: %#v", *cgLimits)

	if err := b.Build(c.dockerClient, c.dockerEndpoint, c.buildsClient, c.build, cgLimits); err != nil {
		return fmt.Errorf("build error: %v", err)
	}

	if c.build.Spec.Output.To == nil || len(c.build.Spec.Output.To.Name) == 0 {
		fmt.Fprintf(c.out, "Build complete, no image push requested\n")
	}

	return nil
}

type dockerBuilder struct{}

// Build starts a Docker build.
func (dockerBuilder) Build(dockerClient bld.DockerClient, sock string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) error {
	return bld.NewDockerBuilder(dockerClient, buildsClient, build, cgLimits).Build()
}

type s2iBuilder struct{}

// Build starts an S2I build.
func (s2iBuilder) Build(dockerClient bld.DockerClient, sock string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build, cgLimits *s2iapi.CGroupLimits) error {
	return bld.NewS2IBuilder(dockerClient, sock, buildsClient, build, cgLimits).Build()
}

func runBuild(out io.Writer, builder builder) error {
	cfg, err := newBuilderConfigFromEnvironment(out, true)
	if err != nil {
		return err
	}
	return cfg.execute(builder)
}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild(out io.Writer) error {
	return runBuild(out, dockerBuilder{})
}

// RunS2IBuild creates a S2I builder and runs its build
func RunS2IBuild(out io.Writer) error {
	return runBuild(out, s2iBuilder{})
}

// RunGitClone performs a git clone using the build defined in the environment
func RunGitClone(out io.Writer) error {
	cfg, err := newBuilderConfigFromEnvironment(out, false)
	if err != nil {
		return err
	}
	return cfg.clone()
}

// RunManageDockerfile manipulates the dockerfile for docker builds.
// It will write the inline dockerfile to the working directory (possibly
// overwriting an existing dockerfile) and then update the dockerfile
// in the working directory (accounting for contextdir+dockerfilepath)
// with new FROM image information based on the imagestream/imagetrigger
// and also adds some env and label values to the dockerfile based on
// the build information.
func RunManageDockerfile(out io.Writer) error {
	cfg, err := newBuilderConfigFromEnvironment(out, false)
	if err != nil {
		return err
	}
	return bld.ManageDockerfile(buildutil.InputContentPath, cfg.build)
}

// RunExtractImageContent extracts files from existing images
// into the build working directory.
func RunExtractImageContent(out io.Writer) error {
	cfg, err := newBuilderConfigFromEnvironment(out, true)
	if err != nil {
		return err
	}
	return cfg.extractImageContent()
}

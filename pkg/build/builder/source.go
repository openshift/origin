package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/builder/timing"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/git"
	"github.com/openshift/source-to-image/pkg/tar"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// initialURLCheckTimeout is the initial timeout used to check the
	// source URL.  If fetching the URL exceeds the timeout, then a longer
	// timeout will be tried until the fetch either succeeds or the build
	// itself times out.
	initialURLCheckTimeout = 16 * time.Second

	// timeoutIncrementFactor is the factor to use when increasing
	// the timeout after each unsuccessful try
	timeoutIncrementFactor = 4
)

type gitAuthError string
type gitNotFoundError string

func (e gitAuthError) Error() string {
	return fmt.Sprintf("failed to fetch requested repository %q with provided credentials", string(e))
}

func (e gitNotFoundError) Error() string {
	return fmt.Sprintf("requested repository %q not found", string(e))
}

// GitClone clones the source associated with a build(if any) into the specified directory
func GitClone(ctx context.Context, gitClient GitClient, gitSource *buildapiv1.GitBuildSource, revision *buildapiv1.SourceRevision, dir string) (*git.SourceInfo, error) {

	// It is possible for the initcontainer to get restarted, thus we must wipe out the directory if it already exists.
	err := os.RemoveAll(dir)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(dir, 0777)

	hasGitSource, err := extractGitSource(ctx, gitClient, gitSource, revision, dir, initialURLCheckTimeout)

	if err != nil {
		return nil, err
	}

	var sourceInfo *git.SourceInfo
	if hasGitSource {
		var errs []error
		sourceInfo, errs = gitClient.GetInfo(dir)
		if len(errs) > 0 {
			for _, e := range errs {
				glog.V(0).Infof("error: Unable to retrieve Git info: %v", e)
			}
		}
		if sourceInfo != nil {
			sourceInfoJson, err := json.Marshal(*sourceInfo)
			if err != nil {
				glog.V(0).Infof("error: Unable to serialized git source info: %v", err)
				return sourceInfo, nil
			}
			err = ioutil.WriteFile(filepath.Join(buildutil.BuildWorkDirMount, "sourceinfo.json"), sourceInfoJson, 0644)
			if err != nil {
				glog.V(0).Infof("error: Unable to serialized git source info: %v", err)
				return sourceInfo, nil
			}
		}
	}
	return sourceInfo, nil
}

// ManageDockerfile manipulates the dockerfile for docker builds.
// It will write the inline dockerfile to the working directory (possibly
// overwriting an existing dockerfile) and then update the dockerfile
// in the working directory (accounting for contextdir+dockerfilepath)
// with new FROM image information based on the imagestream/imagetrigger
// and also adds some env and label values to the dockerfile based on
// the build information.
func ManageDockerfile(dir string, build *buildapiv1.Build) error {
	os.MkdirAll(dir, 0777)
	glog.V(5).Infof("Checking for presence of a Dockerfile")
	// a Dockerfile has been specified, create or overwrite into the destination
	if dockerfileSource := build.Spec.Source.Dockerfile; dockerfileSource != nil {
		baseDir := dir
		if len(build.Spec.Source.ContextDir) != 0 {
			baseDir = filepath.Join(baseDir, build.Spec.Source.ContextDir)
		}
		if err := ioutil.WriteFile(filepath.Join(baseDir, "Dockerfile"), []byte(*dockerfileSource), 0660); err != nil {
			return err
		}
	}

	// We only mutate the dockerfile if this is a docker strategy build, otherwise
	// we leave it as it was provided.
	if build.Spec.Strategy.DockerStrategy != nil {
		sourceInfo, err := readSourceInfo()
		if err != nil {
			return fmt.Errorf("error reading git source info: %v", err)
		}
		return addBuildParameters(dir, build, sourceInfo)
	}
	return nil
}

func ExtractImageContent(ctx context.Context, dockerClient DockerClient, dir string, build *buildapiv1.Build) error {
	os.MkdirAll(dir, 0777)
	forcePull := false
	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		forcePull = build.Spec.Strategy.SourceStrategy.ForcePull
	case build.Spec.Strategy.DockerStrategy != nil:
		forcePull = build.Spec.Strategy.DockerStrategy.ForcePull
	case build.Spec.Strategy.CustomStrategy != nil:
		forcePull = build.Spec.Strategy.CustomStrategy.ForcePull
	}
	// extract source from an Image if specified
	for i, image := range build.Spec.Source.Images {
		if len(image.Paths) == 0 {
			continue
		}
		imageSecretIndex := i
		if image.PullSecret == nil {
			imageSecretIndex = -1
		}
		err := extractSourceFromImage(ctx, dockerClient, image.From.Name, dir, imageSecretIndex, image.Paths, forcePull)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkRemoteGit validates the specified Git URL. It returns GitNotFoundError
// when the remote repository not found and GitAuthenticationError when the
// remote repository failed to authenticate.
// Since this is calling the 'git' binary, the proxy settings should be
// available for this command.
func checkRemoteGit(gitClient GitClient, url string, initialTimeout time.Duration) error {

	var (
		out    string
		errOut string
		err    error
	)

	timeout := initialTimeout
	for {
		glog.V(4).Infof("git ls-remote --heads %s", url)
		out, errOut, err = gitClient.TimedListRemote(timeout, url, "--heads")
		if len(out) != 0 {
			glog.V(4).Infof(out)
		}
		if len(errOut) != 0 {
			glog.V(4).Infof(errOut)
		}
		if err != nil {
			if _, ok := err.(*git.TimeoutError); ok {
				timeout = timeout * timeoutIncrementFactor
				glog.Infof("WARNING: timed out waiting for git server, will wait %s", timeout)
				continue
			}
		}
		break
	}
	if err != nil {
		combinedOut := out + errOut
		switch {
		case strings.Contains(combinedOut, "Authentication failed"):
			return gitAuthError(url)
		case strings.Contains(combinedOut, "not found"):
			return gitNotFoundError(url)
		}
	}
	return err
}

// checkSourceURI performs a check on the URI associated with the build
// to make sure that it is valid.
func checkSourceURI(gitClient GitClient, rawurl string, timeout time.Duration) error {
	_, err := s2igit.Parse(rawurl)
	if err != nil {
		return fmt.Errorf("Invalid git source url %q: %v", rawurl, err)
	}
	return checkRemoteGit(gitClient, rawurl, timeout)
}

// ExtractInputBinary processes the provided input stream as directed by BinaryBuildSource
// into dir.
func ExtractInputBinary(in io.Reader, source *buildapiv1.BinaryBuildSource, dir string) error {
	os.MkdirAll(dir, 0777)
	if source == nil {
		return nil
	}

	var path string
	if len(source.AsFile) > 0 {
		glog.V(0).Infof("Receiving source from STDIN as file %s", source.AsFile)
		path = filepath.Join(dir, source.AsFile)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0664)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(f, os.Stdin)
		if err != nil {
			return err
		}
		glog.V(4).Infof("Received %d bytes into %s", n, path)
		return nil
	}

	glog.V(0).Infof("Receiving source from STDIN as archive ...")

	cmd := exec.Command("bsdtar", "-x", "-o", "-m", "-f", "-", "-C", dir)
	cmd.Stdin = in
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.V(0).Infof("Extracting...\n%s", string(out))
		return fmt.Errorf("unable to extract binary build input, must be a zip, tar, or gzipped tar, or specified as a file: %v", err)
	}

	return nil
}

func extractGitSource(ctx context.Context, gitClient GitClient, gitSource *buildapiv1.GitBuildSource, revision *buildapiv1.SourceRevision, dir string, timeout time.Duration) (bool, error) {
	if gitSource == nil {
		return false, nil
	}

	glog.V(0).Infof("Cloning %q ...", gitSource.URI)

	// Check source URI by trying to connect to the server
	if err := checkSourceURI(gitClient, gitSource.URI, timeout); err != nil {
		return true, err
	}

	cloneOptions := []string{}
	usingRevision := revision != nil && revision.Git != nil && len(revision.Git.Commit) != 0
	usingRef := len(gitSource.Ref) != 0 || usingRevision

	// check if we specify a commit, ref, or branch to check out
	// Recursive clone if we're not going to checkout a ref and submodule update later
	if !usingRef {
		cloneOptions = append(cloneOptions, "--recursive")
		cloneOptions = append(cloneOptions, git.Shallow)
	}

	glog.V(3).Infof("Cloning source from %s", gitSource.URI)

	// Only use the quiet flag if Verbosity is not 5 or greater
	if !glog.Is(5) {
		cloneOptions = append(cloneOptions, "--quiet")
	}
	startTime := metav1.Now()
	if err := gitClient.CloneWithOptions(dir, gitSource.URI, cloneOptions...); err != nil {
		return true, err
	}

	timing.RecordNewStep(ctx, buildapiv1.StageFetchInputs, buildapiv1.StepFetchGitSource, startTime, metav1.Now())

	// if we specify a commit, ref, or branch to checkout, do so, and update submodules
	if usingRef {
		commit := gitSource.Ref

		if usingRevision {
			commit = revision.Git.Commit
		}

		if err := gitClient.Checkout(dir, commit); err != nil {
			err = gitClient.PotentialPRRetryAsFetch(dir, gitSource.URI, commit, err)
			if err != nil {
				return true, err
			}
		}

		// Recursively update --init
		if err := gitClient.SubmoduleUpdate(dir, true, true); err != nil {
			return true, err
		}
	}

	if glog.Is(0) {
		if information, gitErr := gitClient.GetInfo(dir); len(gitErr) == 0 {
			glog.Infof("\tCommit:\t%s (%s)\n", information.CommitID, information.Message)
			glog.Infof("\tAuthor:\t%s <%s>\n", information.AuthorName, information.AuthorEmail)
			glog.Infof("\tDate:\t%s\n", information.Date)
		}
	}

	return true, nil
}

func copyImageSource(dockerClient DockerClient, containerID, sourceDir, destDir string, tarHelper tar.Tar) error {
	// Setup destination directory
	fi, err := os.Stat(destDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		glog.V(4).Infof("Creating image destination directory: %s", destDir)
		err := os.MkdirAll(destDir, 0755)
		if err != nil {
			return err
		}
	} else {
		if !fi.IsDir() {
			return fmt.Errorf("destination %s must be a directory", destDir)
		}
	}

	tempFile, err := ioutil.TempFile("", "imgsrc")
	if err != nil {
		return err
	}
	glog.V(4).Infof("Downloading source from path %s in container %s to temporary archive %s", sourceDir, containerID, tempFile.Name())
	err = dockerClient.DownloadFromContainer(containerID, docker.DownloadFromContainerOptions{
		OutputStream: tempFile,
		Path:         sourceDir,
	})
	if err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	// Extract the created tar file to the destination directory
	file, err := os.Open(tempFile.Name())
	if err != nil {
		return err
	}
	defer file.Close()

	glog.V(4).Infof("Extracting temporary tar %s to directory %s", tempFile.Name(), destDir)
	var tarOutput io.Writer
	if glog.Is(4) {
		tarOutput = os.Stdout
	}
	return tarHelper.ExtractTarStreamWithLogging(destDir, file, tarOutput)
}

func extractSourceFromImage(ctx context.Context, dockerClient DockerClient, image, buildDir string, imageSecretIndex int, paths []buildapiv1.ImageSourcePath, forcePull bool) error {
	glog.V(4).Infof("Extracting image source from %s", image)
	dockerAuth := docker.AuthConfiguration{}
	if imageSecretIndex != -1 {
		pullSecret := os.Getenv(fmt.Sprintf("%s%d", dockercfg.PullSourceAuthType, imageSecretIndex))
		if len(pullSecret) > 0 {
			authPresent := false
			dockerAuth, authPresent = dockercfg.NewHelper().GetDockerAuth(image, fmt.Sprintf("%s%d", dockercfg.PullSourceAuthType, imageSecretIndex))
			if authPresent {
				glog.V(5).Infof("Registry server Address: %s", dockerAuth.ServerAddress)
				glog.V(5).Infof("Registry server User Name: %s", dockerAuth.Username)
				glog.V(5).Infof("Registry server Email: %s", dockerAuth.Email)
				passwordPresent := "<<empty>>"
				if len(dockerAuth.Password) > 0 {
					passwordPresent = "<<non-empty>>"
				}
				glog.V(5).Infof("Registry server Password: %s", passwordPresent)
			}
		}
	}

	exists := true
	if !forcePull {
		_, err := dockerClient.InspectImage(image)
		if err == docker.ErrNoSuchImage {
			exists = false
		} else if err != nil {
			return err
		}
	}

	if !exists || forcePull {
		glog.V(0).Infof("Pulling image %q ...", image)
		startTime := metav1.Now()
		if err := dockerClient.PullImage(docker.PullImageOptions{Repository: image}, dockerAuth); err != nil {
			return fmt.Errorf("error pulling image %v: %v", image, err)
		}

		timing.RecordNewStep(ctx, buildapiv1.StagePullImages, buildapiv1.StepPullInputImage, startTime, metav1.Now())

	}

	containerConfig := &docker.Config{Image: image}
	if inspect, err := dockerClient.InspectImage(image); err != nil {
		return err
	} else {
		// In case the Docker image does not specify the entrypoint
		if len(inspect.Config.Entrypoint) == 0 && len(inspect.Config.Cmd) == 0 {
			containerConfig.Entrypoint = []string{"/fake-entrypoint"}
		}
	}

	// Create container to copy from
	container, err := dockerClient.CreateContainer(docker.CreateContainerOptions{Config: containerConfig})
	if err != nil {
		return fmt.Errorf("error creating source image container: %v", err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID})

	tarHelper := tar.New(s2ifs.NewFileSystem())
	tarHelper.SetExclusionPattern(nil)

	for _, path := range paths {
		glog.V(4).Infof("Extracting path %s from container %s to %s", path.SourcePath, container.ID, path.DestinationDir)
		err := copyImageSource(dockerClient, container.ID, path.SourcePath, filepath.Join(buildDir, path.DestinationDir), tarHelper)
		if err != nil {
			return fmt.Errorf("error copying source path %s to %s: %v", path.SourcePath, path.DestinationDir, err)
		}
	}

	return nil
}

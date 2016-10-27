package images

import (
	"bytes"
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	dockerclient "github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/api"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	// There are coefficients used to multiply layer data size to get a rough size of uploaded blob.
	layerSizeMultiplierForDocker18     = 2.0
	layerSizeMultiplierForLatestDocker = 0.8
)

var (
	pushDeniedErrorMessages []string = []string{
		// docker < 1.10
		`requested access to the resource is denied`,
		// docker client command >= 1.10
		`failed to push image: denied`,
		// docker daemon output >= 1.10
		`^denied$`,
	}
	// reExpectedDeniedError matches the output from `docker push` command when the push is denied. The output
	// differs based on Docker version.
	reExpectedDeniedError *regexp.Regexp = regexp.MustCompile(`(?i)` + strings.Join(pushDeniedErrorMessages, "|"))
	rePushedImageDigest   *regexp.Regexp = regexp.MustCompile(`(?i)digest\s*:\s*(sha\S+)`)
	reSuccessfulBuild     *regexp.Regexp = regexp.MustCompile(`(?i)Successfully built\s+(\S+)`)
)

// GetImageLabels retrieves Docker labels from image from image repository name and
// image reference
func GetImageLabels(c client.ImageStreamImageInterface, imageRepoName, imageRef string) (map[string]string, error) {
	_, imageID, err := api.ParseImageStreamImageName(imageRef)
	image, err := c.Get(imageRepoName, imageID)

	if err != nil {
		return map[string]string{}, err
	}
	return image.Image.DockerImageMetadata.Config.Labels, nil
}

// BuildAndPushImageOfSizeWithBuilder tries to build an image of wanted size and number of layers. Built image
// is stored as an image stream tag <name>:<tag>. If shouldSucceed is false, a build is expected to fail with
// a denied error. Note the size is only approximate. Resulting image size will be different depending on used
// compression algorithm and metadata overhead.
func BuildAndPushImageOfSizeWithBuilder(
	oc *exutil.CLI,
	dClient *dockerclient.Client,
	namespace, name, tag string,
	size uint64,
	numberOfLayers int,
	shouldSucceed bool,
) error {
	istName := name
	if tag != "" {
		istName += ":" + tag
	}

	bc, err := oc.REST().BuildConfigs(namespace).Get(name)
	if err == nil {
		if bc.Spec.CommonSpec.Output.To.Kind != "ImageStreamTag" {
			return fmt.Errorf("Unexpected kind of buildspec's output (%s != %s)", bc.Spec.CommonSpec.Output.To.Kind, "ImageStreamTag")
		}
		bc.Spec.CommonSpec.Output.To.Name = istName
		if _, err = oc.REST().BuildConfigs(namespace).Update(bc); err != nil {
			return err
		}
	} else {
		err = oc.Run("new-build").Args("--binary", "--name", name, "--to", istName).Execute()
		if err != nil {
			return err
		}
	}

	tempDir, err := ioutil.TempDir("", "name-build")
	if err != nil {
		return err
	}

	dataSize := calculateRoughDataSize(oc.Stdout(), size, numberOfLayers)

	lines := make([]string, numberOfLayers+1)
	lines[0] = "FROM scratch"
	for i := 1; i <= numberOfLayers; i++ {
		blobName := fmt.Sprintf("data%d", i)
		if err := createRandomBlob(path.Join(tempDir, blobName), dataSize); err != nil {
			return err
		}
		lines[i] = fmt.Sprintf("COPY %s /%s", blobName, blobName)
	}
	if err := ioutil.WriteFile(path.Join(tempDir, "Dockerfile"), []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return err
	}

	br, _ := exutil.StartBuildAndWait(oc, name, "--from-dir", tempDir)
	if shouldSucceed {
		br.AssertSuccess()
	} else {
		br.AssertFailure()
	}
	buildLog, logsErr := br.Logs()

	if match := reSuccessfulBuild.FindStringSubmatch(buildLog); len(match) > 1 {
		defer dClient.RemoveImageExtended(match[1], dockerclient.RemoveImageOptions{Force: true})
	}

	if !shouldSucceed {
		if logsErr != nil {
			return fmt.Errorf("Failed to show log of build config %s: %v", name, err)
		}
		if !reExpectedDeniedError.MatchString(buildLog) {
			return fmt.Errorf("Failed to match expected %q in: %q", reExpectedDeniedError.String(), buildLog)
		}
	}

	return nil
}

// BuildAndPushImageOfSizeWithDocker tries to build an image of wanted size and number of layers. It instructs
// Docker daemon directly. Built image is stored as an image stream tag <name>:<tag>. If shouldSucceed is
// false, a push is expected to fail with a denied error. Note the size is only approximate. Resulting image
// size will be different depending on used compression algorithm and metadata overhead.
func BuildAndPushImageOfSizeWithDocker(
	oc *exutil.CLI,
	dClient *dockerclient.Client,
	name, tag string,
	size uint64,
	numberOfLayers int,
	outSink io.Writer,
	shouldSucceed bool,
) (imageDigest string, err error) {
	registryURL, err := GetDockerRegistryURL(oc)
	if err != nil {
		return "", err
	}
	tempDir, err := ioutil.TempDir("", "name-build")
	if err != nil {
		return "", err
	}

	dataSize := calculateRoughDataSize(oc.Stdout(), size, numberOfLayers)

	lines := make([]string, numberOfLayers+1)
	lines[0] = "FROM scratch"
	for i := 1; i <= numberOfLayers; i++ {
		blobName := fmt.Sprintf("data%d", i)
		if err := createRandomBlob(path.Join(tempDir, blobName), dataSize); err != nil {
			return "", err
		}
		lines[i] = fmt.Sprintf("COPY %s /%s", blobName, blobName)
	}
	if err := ioutil.WriteFile(path.Join(tempDir, "Dockerfile"), []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return "", err
	}

	imageName := fmt.Sprintf("%s/%s/%s", registryURL, oc.Namespace(), name)
	taggedName := fmt.Sprintf("%s:%s", imageName, tag)

	err = dClient.BuildImage(dockerclient.BuildImageOptions{
		Name:                taggedName,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		ContextDir:          tempDir,
		OutputStream:        outSink,
	})
	if err != nil {
		return "", fmt.Errorf("failed to build %q image: %v", taggedName, err)
	}

	image, err := dClient.InspectImage(taggedName)
	if err != nil {
		return
	}

	defer dClient.RemoveImageExtended(image.ID, dockerclient.RemoveImageOptions{Force: true})
	if len(image.RepoDigests) == 1 {
		imageDigest = image.RepoDigests[0]
	}

	out, err := oc.Run("whoami").Args("-t").Output()
	if err != nil {
		return
	}
	token := strings.TrimSpace(out)

	var buf bytes.Buffer
	err = dClient.PushImage(dockerclient.PushImageOptions{
		Name:         imageName,
		Tag:          tag,
		Registry:     registryURL,
		OutputStream: &buf,
	}, dockerclient.AuthConfiguration{
		Username:      "test",
		Password:      token,
		Email:         "test@test.com",
		ServerAddress: registryURL,
	})
	out = buf.String()
	outSink.Write([]byte(out))

	if shouldSucceed {
		if err != nil {
			return "", fmt.Errorf("Got unexpected push error: %v", err)
		}
		if len(imageDigest) == 0 {
			outSink.Write([]byte("matching digest string\n"))
			match := rePushedImageDigest.FindStringSubmatch(out)
			if len(match) < 2 {
				return imageDigest, fmt.Errorf("Failed to parse digest")
			}
			imageDigest = match[1]
		}
		return
	}

	if err == nil {
		return "", fmt.Errorf("Push unexpectedly succeeded")
	}
	if !reExpectedDeniedError.MatchString(err.Error()) {
		return "", fmt.Errorf("Failed to match expected %q in: %q", reExpectedDeniedError.String(), err.Error())
	}

	return "", nil
}

// GetDockerRegistryURL returns a cluster URL of internal docker registry if available.
func GetDockerRegistryURL(oc *exutil.CLI) (string, error) {
	svc, err := oc.AdminKubeREST().Services("default").Get("docker-registry")
	if err != nil {
		return "", err
	}
	url := svc.Spec.ClusterIP
	for _, p := range svc.Spec.Ports {
		url = fmt.Sprintf("%s:%d", url, p.Port)
		break
	}
	return url, nil
}

// createRandomBlob creates a random data with bytes from `letters` in order to let docker take advantage of
// compression. Resulting layer size will be different due to file metadata overhead and compression.
func createRandomBlob(dest string, size uint64) error {
	var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	data := make([]byte, size)
	if _, err = cryptorand.Read(data); err != nil {
		return err
	}

	for i := range data {
		data[i] = letters[uint(data[i])%uint(len(letters))]
	}

	f.Write(data)
	return nil
}

// dockerVersion is a cached version string of Docker daemon
var dockerVersion = ""

// getDockerVersion returns a version of running Docker daemon which is questioned only during the first
// invocation.
func getDockerVersion(logger io.Writer) (major, minor int, version string, err error) {
	reVersion := regexp.MustCompile(`^(\d+)\.(\d+)`)

	if dockerVersion == "" {
		client, err2 := testutil.NewDockerClient()
		if err = err2; err != nil {
			return
		}
		env, err2 := client.Version()
		if err = err2; err != nil {
			return
		}
		dockerVersion = env.Get("Version")
		if logger != nil {
			logger.Write([]byte(fmt.Sprintf("Using docker version %s\n", version)))
		}
	}
	version = dockerVersion

	matches := reVersion.FindStringSubmatch(version)
	if len(matches) < 3 {
		return 0, 0, "", fmt.Errorf("failed to parse version string %s", version)
	}
	major, _ = strconv.Atoi(matches[1])
	minor, _ = strconv.Atoi(matches[2])
	return
}

// calculateRoughDataSize returns a rough size of data blob to generate in order to build an image of wanted
// size. Image is comprised of numberOfLayers layers of the same size.
func calculateRoughDataSize(logger io.Writer, wantedImageSize uint64, numberOfLayers int) uint64 {
	major, minor, version, err := getDockerVersion(logger)
	if err != nil {
		// TODO(miminar): shall we use some better logging mechanism?
		logger.Write([]byte(fmt.Sprintf("Failed to get docker version: %v\n", err)))
	}
	if (major >= 1 && minor >= 9) || version == "" {
		// running Docker version 1.9+
		return uint64(float64(wantedImageSize) / (float64(numberOfLayers) * layerSizeMultiplierForLatestDocker))
	}

	// running Docker daemon < 1.9
	return uint64(float64(wantedImageSize) / (float64(numberOfLayers) * layerSizeMultiplierForDocker18))
}

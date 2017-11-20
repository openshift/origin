package images

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
	g "github.com/onsi/ginkgo"

	godigest "github.com/opencontainers/go-digest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagetypedclientset "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	// There are coefficients used to multiply layer data size to get a rough size of uploaded blob.
	layerSizeMultiplierForDocker18     = 2.0
	layerSizeMultiplierForLatestDocker = 0.8
	defaultLayerSize                   = 1024
	digestSHA256GzippedEmptyTar        = godigest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")
	digestSha256EmptyTar               = godigest.Digest("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")

	dockerRegistryBinary     = "dockerregistry"
	registryGCLauncherScript = `#!/bin/sh
bin="$(which %[1]s 2>/dev/null)"
if [ -z "${bin}" -a -e "/usr/bin/%[1]s" ]; then
	bin="/usr/bin/%[1]s"
elif [ -z "${bin}" -a -e "/%[1]s" ]; then
	bin="/%[1]s"
fi
export REGISTRY_LOG_LEVEL=info
exec "${bin}" -prune=%[2]s
`
)

// RepoLinks maps digests of layer links to a repository
type RepoLinks map[string][]string

func (rl RepoLinks) Add(repo string, links ...string) {
	rl[repo] = append(rl[repo], links...)
}

type RegistryStorageFiles struct {
	Repos         []string
	ManifestLinks RepoLinks
	LayerLinks    RepoLinks
	Blobs         []string
}

// ToPaths returns a list of paths of files contained in _sfs_ corresponding to their location in registry
// pod's storage under _root_ directory.
func (sfs *RegistryStorageFiles) ToPaths(root string) []string {
	result := []string{}
	if sfs == nil {
		return result
	}
	for _, repo := range sfs.Repos {
		result = append(result, repoToPath(root, repo))
	}
	for repo, links := range sfs.ManifestLinks {
		for _, link := range links {
			result = append(result, repoLinkToPath(root, "manifest", repo, link))
		}
	}
	for repo, links := range sfs.LayerLinks {
		for _, link := range links {
			result = append(result, repoLinkToPath(root, "layer", repo, link))
		}
	}
	for _, blob := range sfs.Blobs {
		result = append(result, blobToPath(root, blob))
	}
	return result
}

// Len returns a number of files contained in the sfs container.
func (sfs *RegistryStorageFiles) Len() int {
	if sfs == nil {
		return 0
	}
	count := len(sfs.Blobs) + len(sfs.Repos)
	for _, links := range sfs.ManifestLinks {
		count += len(links)
	}
	for _, links := range sfs.LayerLinks {
		count += len(links)
	}
	return count
}

func repoToPath(root, repository string) string {
	return path.Join(root, fmt.Sprintf("repositories/%s", repository))
}
func repoLinkToPath(root, fileType, repository, dgst string) string {
	d := godigest.Digest(dgst)
	return path.Join(root, fmt.Sprintf("repositories/%s/_%ss/%s/%s/link",
		repository, fileType, d.Algorithm(), d.Hex()))
}
func blobToPath(root, dgst string) string {
	d := godigest.Digest(dgst)
	return path.Join(root, fmt.Sprintf("blobs/%s/%s/%s/data",
		d.Algorithm(), d.Hex()[0:2], d.Hex()))
}

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
func GetImageLabels(c imagetypedclientset.ImageStreamImageInterface, imageRepoName, imageRef string) (map[string]string, error) {
	_, imageID, err := imageapi.ParseImageStreamImageName(imageRef)
	image, err := c.Get(imageapi.JoinImageStreamImage(imageRepoName, imageID), metav1.GetOptions{})

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

	bc, err := oc.BuildClient().Build().BuildConfigs(namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		if bc.Spec.CommonSpec.Output.To.Kind != "ImageStreamTag" {
			return fmt.Errorf("Unexpected kind of buildspec's output (%s != %s)", bc.Spec.CommonSpec.Output.To.Kind, "ImageStreamTag")
		}
		bc.Spec.CommonSpec.Output.To.Name = istName
		if _, err = oc.BuildClient().Build().BuildConfigs(namespace).Update(bc); err != nil {
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
// Returned is an image digest, its ID (docker daemon's internal representation) and an error if any.
func BuildAndPushImageOfSizeWithDocker(
	oc *exutil.CLI,
	dClient *dockerclient.Client,
	name, tag string,
	size uint64,
	numberOfLayers int,
	outSink io.Writer,
	shouldSucceed bool,
	removeBuiltImage bool,
) (string, string, error) {
	imageName, image, err := buildImageOfSizeWithDocker(
		oc,
		dClient,
		"scratch",
		name, tag,
		size,
		numberOfLayers,
		outSink)
	if err != nil {
		return "", "", err
	}

	digest, err := pushImageWithDocker(
		oc,
		dClient,
		image,
		imageName, tag,
		outSink,
		shouldSucceed,
		removeBuiltImage)
	if err != nil {
		return "", "", err
	}

	return digest, image.ID, nil
}

// BuildAndPushChildImage tries to build and push an image of given name and number of layers. It instructs
// Docker daemon directly. Built image is stored as an image stream tag <name>:<tag>.
// Returned is an image digest, its ID (docker daemon's internal representation) and an error if any.
func BuildAndPushChildImage(
	oc *exutil.CLI,
	dClient *dockerclient.Client,
	parent string,
	name, tag string,
	numberOfNewLayers int,
	outSink io.Writer,
	removeBuiltImage bool,
) (string, string, error) {
	imageName, image, err := buildImageOfSizeWithDocker(
		oc, dClient,
		parent, name, tag,
		defaultLayerSize,
		numberOfNewLayers,
		outSink)
	if err != nil {
		return "", "", err
	}

	digest, err := pushImageWithDocker(
		oc, dClient,
		image, imageName, tag,
		outSink,
		true,
		removeBuiltImage)
	if err != nil {
		return "", "", err
	}

	return digest, image.ID, nil
}

func buildImageOfSizeWithDocker(
	oc *exutil.CLI,
	dClient *dockerclient.Client,
	parent, name, tag string,
	size uint64,
	numberOfLayers int,
	outSink io.Writer,
) (string, *dockerclient.Image, error) {
	registryURL, err := GetDockerRegistryURL(oc)
	if err != nil {
		return "", nil, err
	}
	tempDir, err := ioutil.TempDir("", "name-build")
	if err != nil {
		return "", nil, err
	}

	dataSize := calculateRoughDataSize(oc.Stdout(), size, numberOfLayers)

	lines := make([]string, numberOfLayers+1)
	lines[0] = "FROM scratch"
	for i := 1; i <= numberOfLayers; i++ {
		blobName := fmt.Sprintf("data%d", i)
		if err := createRandomBlob(path.Join(tempDir, blobName), dataSize); err != nil {
			return "", nil, err
		}
		lines[i] = fmt.Sprintf("COPY %s /%s", blobName, blobName)
	}
	if err := ioutil.WriteFile(path.Join(tempDir, "Dockerfile"), []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return "", nil, err
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
		return "", nil, fmt.Errorf("failed to build %q image: %v", taggedName, err)
	}

	image, err := dClient.InspectImage(taggedName)
	if err != nil {
		return "", nil, err
	}

	return imageName, image, nil
}

func pushImageWithDocker(
	oc *exutil.CLI,
	dClient *dockerclient.Client,
	image *dockerclient.Image,
	name, tag string,
	outSink io.Writer,
	shouldSucceed bool,
	removeBuiltImage bool,
) (string, error) {
	if removeBuiltImage {
		defer dClient.RemoveImageExtended(image.ID, dockerclient.RemoveImageOptions{Force: true})
	}

	var imageDigest string
	if len(image.RepoDigests) == 1 {
		imageDigest = image.RepoDigests[0]
	}

	out, err := oc.Run("whoami").Args("-t").Output()
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(out)

	registryURL, err := GetDockerRegistryURL(oc)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = dClient.PushImage(dockerclient.PushImageOptions{
		Name:         name,
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

	if !shouldSucceed {
		if err == nil {
			return "", fmt.Errorf("Push unexpectedly succeeded")
		}
		if !reExpectedDeniedError.MatchString(err.Error()) {
			return "", fmt.Errorf("Failed to match expected %q in: %q", reExpectedDeniedError.String(), err.Error())
		}

		// push failed with expected error -> no results
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("Got unexpected push error: %v", err)
	}
	if len(imageDigest) == 0 {
		match := rePushedImageDigest.FindStringSubmatch(out)
		if len(match) < 2 {
			return imageDigest, fmt.Errorf("Failed to parse digest")
		}
		imageDigest = match[1]
	}

	return imageDigest, nil
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
	if major > 1 || major == 1 && minor >= 9 || version == "" {
		// running Docker version 1.9+
		return uint64(float64(wantedImageSize) / (float64(numberOfLayers) * layerSizeMultiplierForLatestDocker))
	}

	// running Docker daemon < 1.9
	return uint64(float64(wantedImageSize) / (float64(numberOfLayers) * layerSizeMultiplierForDocker18))
}

// MirrorBlobInRegistry forces a blob of external image to be mirrored in the registry. The function expects
// the blob not to exist before a GET request is issued. The function blocks until the blob is mirrored or the
// given timeout passes.
func MirrorBlobInRegistry(oc *exutil.CLI, dgst godigest.Digest, repository string, timeout time.Duration) error {
	presentGlobally, inRepository, err := IsBlobStoredInRegistry(oc, dgst, repository)
	if err != nil {
		return err
	}
	if presentGlobally || inRepository {
		return fmt.Errorf("blob %q is already present in the registry", dgst.String())
	}
	registryURL, err := GetDockerRegistryURL(oc)
	if err != nil {
		return err
	}
	token, err := oc.Run("whoami").Args("-t").Output()
	if err != nil {
		return err
	}

	c := http.Client{
		Transport: knet.SetTransportDefaults(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}),
	}

	peekAtBlob := func(schema string) (*http.Request, *http.Response, error) {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s/v2/%s/blobs/%s", schema, registryURL, repository, dgst.String()), nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("range", "bytes=0-1")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.Do(req)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "failed to %s %s: %v (%#+v)\n", req.Method, req.URL, err, err)
			return nil, nil, err
		}
		return req, resp, nil
	}

	var (
		req    *http.Request
		resp   *http.Response
		getErr error
	)
	if req, resp, getErr = peekAtBlob("https"); getErr != nil {
		if req, resp, getErr = peekAtBlob("http"); getErr != nil {
			return getErr
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status %d for request '%s %s', got %d", http.StatusOK, req.Method, req.URL.String(), resp.StatusCode)
	}

	return wait.Poll(time.Second, timeout, func() (bool, error) {
		globally, inRepo, err := IsBlobStoredInRegistry(oc, dgst, repository)
		return globally || inRepo, err
	})
}

// IsEmptyDigest returns true if the given digest matches one of empty blobs.
func IsEmptyDigest(dgst godigest.Digest) bool {
	return dgst == digestSha256EmptyTar || dgst == digestSHA256GzippedEmptyTar
}

func pathExistsInRegistry(oc *exutil.CLI, pthComponents ...string) (bool, error) {
	pth := path.Join(append([]string{"/registry/docker/registry/v2"}, pthComponents...)...)
	cmd := fmt.Sprintf("test -e '%s' && echo exists || echo missing", pth)
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	out, err := oc.SetNamespace(metav1.NamespaceDefault).AsAdmin().Run("rsh").Args(
		"dc/docker-registry", "/bin/sh", "-c", cmd).Output()
	if err != nil {
		return false, fmt.Errorf("failed to check for blob existence: %v", err)
	}
	return strings.HasPrefix(out, "exists"), nil
}

// IsBlobStoredInRegistry verifies a presence of the given blob on registry's storage. The registry must be
// deployed with a filesystem storage driver. If repository is given, the presence will be verified also for
// layer link inside the ${repository}/_layers directory. First returned bool says whether the blob is present
// globally in the registry's storage. The second says whether the blob is linked in the given repository.
func IsBlobStoredInRegistry(
	oc *exutil.CLI,
	dgst godigest.Digest,
	repository string,
) (bool, bool, error) {
	present, err := pathExistsInRegistry(
		oc,
		"blobs",
		string(dgst.Algorithm()),
		dgst.Hex()[0:2],
		dgst.Hex(),
		"data")
	if err != nil || !present {
		return false, false, err
	}

	presentInRepository := false
	if len(repository) > 0 {
		presentInRepository, err = pathExistsInRegistry(oc,
			"repositories",
			repository,
			"_layers",
			string(dgst.Algorithm()),
			dgst.Hex(),
			"link")
	}
	return present, presentInRepository, err
}

// RunHardPrune executes into a docker-registry pod and runs a garbage collector. The docker-registry is
// assumed to be in a read-only mode and using filesystem as a storage driver. It returns lists of deleted
// files.
func RunHardPrune(oc *exutil.CLI, dryRun bool) (*RegistryStorageFiles, error) {
	pod, err := GetRegistryPod(oc.AsAdmin().KubeClient().Core())
	if err != nil {
		return nil, err
	}

	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	output, err := oc.AsAdmin().SetNamespace(metav1.NamespaceDefault).Run("env").Args("--list", "dc/docker-registry").Output()
	if err != nil {
		return nil, err
	}

	deleted := &RegistryStorageFiles{
		Repos:         []string{},
		ManifestLinks: make(RepoLinks),
		LayerLinks:    make(RepoLinks),
		Blobs:         []string{},
	}

	err = wait.ExponentialBackoff(retry.DefaultBackoff, func() (bool, error) {
		pruneType := "delete"
		if dryRun {
			pruneType = "check"
		}
		out, err := oc.SetNamespace(metav1.NamespaceDefault).AsAdmin().
			Run("exec").Args("--stdin", pod.Name, "--", "/bin/sh", "-s").
			InputString(fmt.Sprintf(registryGCLauncherScript, dockerRegistryBinary, pruneType)).Output()
		if exitError, ok := err.(*exutil.ExitError); ok && strings.Contains(exitError.StdErr, "unable to upgrade connection") {
			fmt.Fprintf(g.GinkgoWriter, "failed to execute into registry pod %s: %v\n", pod.Name, err)
			return false, nil
		}
		output = out
		return true, err
	})
	if len(output) > 0 {
		fmt.Fprintf(g.GinkgoWriter, "prune output: \n%s\n\n", output)
	}

	if err != nil {
		return nil, err
	}

	const reCommon = `(?im)\bmsg="(?:would\s+)?delet(?:e|ing)\s+(?:the\s+)?`
	var reDeleteRepository = regexp.MustCompile(reCommon + `repo(?:sitory)?:\s+` +
		`(?:[^"]+/)?` /* root path of the repository */ +
		`([^"/]+/[^"/]+)/?"` /* repository */)
	var reDeleteRepositoryLink = regexp.MustCompile(reCommon +
		`(manifest|layer)(?:\s+link)?:\s+` /* type of link's destination file */ +
		`([^#@]+)[#@]` /* repository */ +
		`([^"]+)"` /* digest */)
	var reDeleteBlob = regexp.MustCompile(reCommon + `blob:\s+` +
		`(?:[^"]+/)?` /* root path to the blob */ +
		`([^":/]+)` /* digest algorithm */ +
		`(?:/[^"/]{2}/|:)` /* directory whose name matches the first two characters of digest hex */ +
		`([^":/]+)"` /* digest hex */)
	var reDeletedBlobs = regexp.MustCompile(`(?im)^(?:would\s+)?deleted?\s+(\d+)\s+blobs$`)

	for _, match := range reDeleteRepository.FindAllStringSubmatch(output, -1) {
		deleted.Repos = append(deleted.Repos, match[1])
	}
	for _, match := range reDeleteRepositoryLink.FindAllStringSubmatch(output, -1) {
		fileType, repository, digest := match[1], match[2], match[3]

		switch strings.ToLower(fileType) {
		case "manifest":
			deleted.ManifestLinks.Add(repository, digest)
		case "link":
			deleted.LayerLinks.Add(repository, digest)
		default:
			fmt.Fprintf(g.GinkgoWriter, "unrecognized type of deleted file: %s\n", match[1])
			continue
		}
	}
	for _, match := range reDeleteBlob.FindAllStringSubmatch(output, -1) {
		deleted.Blobs = append(deleted.Blobs, fmt.Sprintf("%s:%s", match[1], match[2]))
	}

	match := reDeletedBlobs.FindStringSubmatch(output)
	if match == nil {
		return nil, fmt.Errorf("missing the number of deleted blobs in the output")
	}

	deletedBlobCount, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse deleted number of blobs %q: %v", match[1], err)
	}
	if deletedBlobCount != len(deleted.Blobs) {
		return nil, fmt.Errorf("numbers of deleted blobs doesn't match %d != %d", len(deleted.Blobs), deletedBlobCount)
	}

	return deleted, nil
}

// AssertDeletedStorageFiles compares lists of deleted files against expected. An error will be generated for
// each entry present in just one of these sets.
func AssertDeletedStorageFiles(deleted, expected *RegistryStorageFiles) error {
	var errors []error
	deletedSet := sets.NewString(deleted.ToPaths("")...)
	expectedPaths := sets.NewString(expected.ToPaths("")...)
	verifiedSet := sets.NewString()

	for pth := range expectedPaths {
		if deletedSet.Has(pth) {
			verifiedSet.Insert(pth)
		} else {
			errors = append(errors, fmt.Errorf("expected path %s was not deleted", pth))
		}
	}
	for pth := range deletedSet {
		if !expectedPaths.Has(pth) {
			errors = append(errors, fmt.Errorf("path %s got unexpectedly deleted", pth))
		}
	}

	return kerrors.NewAggregate(errors)
}

// CleanUpContainer holds names of image names, docker image IDs, imagestreamtags and imagestreams that shall
// be deleted at the end of the test.
type CleanUpContainer struct {
	OC *exutil.CLI

	imageNames sets.String
	imageIDs   sets.String
	isTags     sets.String
	isNames    sets.String
}

// NewCleanUpContainer creates a new instance of CleanUpContainer.
func NewCleanUpContainer(oc *exutil.CLI) *CleanUpContainer {
	return &CleanUpContainer{
		OC:         oc,
		imageNames: sets.NewString(),
		imageIDs:   sets.NewString(),
		isTags:     sets.NewString(),
		isNames:    sets.NewString(),
	}
}

// AddImage marks given image name, docker image id and imagestreamtag as candidates for deletion.
func (c *CleanUpContainer) AddImage(name, id, isTag string) {
	if len(name) > 0 {
		c.imageNames.Insert(name)
	}
	if len(id) > 0 {
		c.imageIDs.Insert(id)
	}
	if len(isTag) > 0 {
		c.isNames.Insert(isTag)
	}
}

// AddImageStream marks the given image stream name for removal.
func (c *CleanUpContainer) AddImageStream(isName string) {
	c.isNames.Insert(isName)
}

// Run deletes all the marked objects.
func (c *CleanUpContainer) Run() {
	for image := range c.imageNames {
		err := c.OC.AsAdmin().ImageClient().Image().Images().Delete(image, nil)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "clean up of image %q failed: %v\n", image, err)
		}
	}
	for isName := range c.isNames {
		err := c.OC.AsAdmin().ImageClient().Image().ImageStreams(c.OC.Namespace()).Delete(isName, nil)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "clean up of image stream %q failed: %v\n", isName, err)
		}
	}
	for isTag := range c.isTags {
		err := c.OC.ImageClient().Image().ImageStreamTags(c.OC.Namespace()).Delete(isTag, nil)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "clean up of image stream tag %q failed: %v\n", isTag, err)
		}
	}

	// Remove registry database between tests to avoid the influence of one test on another.
	// TODO: replace this with removals of individual blobs used in the test case.
	out, err := c.OC.SetNamespace(metav1.NamespaceDefault).AsAdmin().
		Run("rsh").Args("dc/docker-registry", "find", "/registry", "-mindepth", "1", "-delete").Output()
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "clean up registry failed: %v\n", err)
		fmt.Fprintf(g.GinkgoWriter, "%s\n", out)
	}

	if len(c.imageIDs) == 0 {
		return
	}

	dClient, err := testutil.NewDockerClient()
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "failed to create a new docker client: %v\n", err)
		return
	}

	for id := range c.imageIDs {
		err := dClient.RemoveImageExtended(id, dockerclient.RemoveImageOptions{Force: true})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "failed to remove image %q: %v\n", id, err)
		}
	}
}

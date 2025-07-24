package extensions

import (
	"compress/gzip"
	"context"
	"crypto/sha1"
	"debug/elf"
	"encoding/json"
	"fmt"
	"io"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
	"github.com/sirupsen/logrus"

	imagev1 "github.com/openshift/api/image/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/test/extended/util"
)

func Time(t *dbtime.DBTime) time.Time {
	if t == nil {
		return time.Time{}
	}
	return time.Time(*t)
}

// ungzipFile checks if a binary is gzipped (ends with .gz) and decompresses it.
// Returns the new filename of the decompressed file (original is deleted), or original filename if it was not gzipped.
func ungzipFile(extractedBinary string) (string, error) {

	if strings.HasSuffix(extractedBinary, ".gz") {

		gzFile, err := os.Open(extractedBinary)
		if err != nil {
			return "", fmt.Errorf("failed to open gzip file: %w", err)
		}
		defer gzFile.Close()

		gzipReader, err := gzip.NewReader(gzFile)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()

		newFilePath := strings.TrimSuffix(extractedBinary, ".gz")
		outFile, err := os.Create(newFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, gzipReader); err != nil {
			return "", fmt.Errorf("failed to write to output file: %w", err)
		}

		// Attempt to delete the original .gz file
		if err := os.Remove(extractedBinary); err != nil {
			return "", fmt.Errorf("failed to delete original .gz file: %w", err)
		}

		return newFilePath, nil
	}

	// Return the original path if the file was not decompressed
	return extractedBinary, nil
}

// Checks whether the binary has a compatible CPU architecture  to the
// host.
func checkCompatibleArchitecture(executablePath string) error {
	file, err := os.Open(executablePath)
	if err != nil {
		return fmt.Errorf("failed to open ELF file: %w", err)
	}
	defer file.Close()

	elfFile, err := elf.NewFile(file)
	if err != nil {
		return fmt.Errorf("failed to parse ELF file: %w", err)
	}

	// Determine the architecture of the ELF file
	elfArch := elfFile.Machine
	var expectedArch elf.Machine

	// Determine the host architecture
	switch runtime.GOARCH {
	case "amd64":
		expectedArch = elf.EM_X86_64
	case "arm64":
		expectedArch = elf.EM_AARCH64
	case "s390x":
		expectedArch = elf.EM_S390
	case "ppc64le":
		expectedArch = elf.EM_PPC64
	default:
		return fmt.Errorf("unsupported host architecture: %s", runtime.GOARCH)
	}

	if elfArch != expectedArch {
		return fmt.Errorf("binary architecture %q doesn't matched expected architecture %q", elfArch, expectedArch)
	}

	return nil
}

// runImageExtract extracts src from specified image to dst
func runImageExtract(image, src, dst string, dockerConfigJsonPath string) error {
	var err error
	var out []byte
	maxRetries := 6
	startTime := time.Now()
	logrus.Infof("Run image extract for release image %q and src %q", image, src)
	for i := 1; i <= maxRetries; i++ {
		args := []string{"--kubeconfig=" + util.KubeConfigPath(), "image", "extract", image, fmt.Sprintf("--path=%s:%s", src, dst), "--confirm"}
		if len(dockerConfigJsonPath) > 0 {
			args = append(args, fmt.Sprintf("--registry-config=%s", dockerConfigJsonPath))
		}
		cmd := exec.Command("oc", args...)
		out, err = cmd.CombinedOutput()
		if err != nil {
			// Allow retries for up to one minute. The openshift internal registry
			// occasionally reports "manifest unknown" when a new image has just
			// been exposed through an imagestream.
			time.Sleep(10 * time.Second)
			continue
		}
		logrus.Infof("Completed image extract for release image %q in %+v", image, time.Since(startTime))
		return nil
	}
	return fmt.Errorf("error during image extract: %w (%v)", err, string(out))
}

// pullSpecToDirName converts a release pullspec to a directory, for use with caching.
func pullSpecToDirName(input string) string {
	// Remove any non-alphanumeric characters (except '-') and replace them with '_'.
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	safeName := re.ReplaceAllString(input, "_")

	// Truncate long names
	if len(safeName) > 249 {
		safeName = safeName[:249]
	}

	// Add suffix to avoid collision when truncating
	hash := sha1.Sum([]byte(input))
	safeName += fmt.Sprintf("_%x", hash[:6])

	// Return a clean, safe directory path.
	return filepath.Clean(safeName)
}

func DetermineReleasePayloadImage() (string, error) {
	var releaseImage string

	// Highest priority override is EXTENSIONS_PAYLOAD_OVERRIDE
	overrideReleaseImage := os.Getenv("EXTENSIONS_PAYLOAD_OVERRIDE")
	if len(overrideReleaseImage) != 0 {
		// if "cluster" is specified, prefer target cluster payload even if RELEASE_IMAGE_LATEST is set.
		if overrideReleaseImage != "cluster" {
			releaseImage = overrideReleaseImage
			logrus.Infof("Using env EXTENSIONS_PAYLOAD_OVERRIDE for release image %q", releaseImage)
		}
	} else {
		// Allow testing using an overridden source for external tests.
		envReleaseImage := os.Getenv("RELEASE_IMAGE_LATEST")
		if len(envReleaseImage) != 0 {
			releaseImage = envReleaseImage
			logrus.Infof("Using env RELEASE_IMAGE_LATEST for release image %q", releaseImage)
		}
	}

	if len(releaseImage) == 0 {
		// Note that MicroShift does not have this resource. The test driver must use ENV vars.
		oc := util.NewCLIWithoutNamespace("default")
		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.TODO(), "version",
			metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed reading ClusterVersion/version: %w", err)
		}

		releaseImage = cv.Status.Desired.Image
		if len(releaseImage) == 0 {
			return "", fmt.Errorf("cannot determine release image from ClusterVersion resource")
		}
		logrus.WithField("release_image", releaseImage).Infof("Using target cluster release image")
	}

	return releaseImage, nil
}

// createBinPath ensures the given path exists, is writable, and allows executing binaries.
func createBinPath(path string) error {
	// Create the directory if it doesn't exist.
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", path, err)
	}

	// Create a simple shell script to test executability.
	testFile := filepath.Join(path, "cache_test.sh")
	scriptContent := "#!/bin/sh\necho 'Executable test passed'"

	// Write the script to the cache directory.
	if err := os.WriteFile(testFile, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write test file in cache path %s: %w", path, err)
	}
	defer os.Remove(testFile)

	// Attempt to execute the test script.
	cmd := exec.Command(testFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute test file in cache path %s: %w", path, err)
	}

	// Check if the output is as expected.
	if string(output) != "Executable test passed\n" {
		return fmt.Errorf("unexpected output from executable test in cache path %s: %s", path, output)
	}

	return nil
}

// extractReleaseImageStream extracts image references from the given releaseImage and returns
// an ImageStream object with tags associated with image-references from that payload.
func extractReleaseImageStream(extractPath, releaseImage string,
	registryAuthFilePath string) (*imagev1.ImageStream, string, error) {

	if _, err := os.Stat(path.Join(extractPath, "image-references")); err != nil {
		if err := runImageExtract(releaseImage, "/release-manifests/image-references", extractPath, registryAuthFilePath); err != nil {
			return nil, "", fmt.Errorf("failed extracting image-references from %q: %w", releaseImage, err)
		}
	}
	jsonFile, err := os.Open(filepath.Join(extractPath, "image-references"))
	if err != nil {
		return nil, "", fmt.Errorf("failed reading image-references from %q: %w", releaseImage, err)
	}
	defer jsonFile.Close()
	data, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, "", fmt.Errorf("unable to load release image-references from %q: %w", releaseImage, err)
	}
	is := &imagev1.ImageStream{}
	if err := json.Unmarshal(data, &is); err != nil {
		return nil, "", fmt.Errorf("unable to load release image-references from %q: %w", releaseImage, err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return nil, "", fmt.Errorf("unrecognized image-references in release payload %q", releaseImage)
	}

	// Allow environmental overrides for individual component images.
	for _, tag := range is.Spec.Tags {
		componentEnvName := "EXTENSIONS_PAYLOAD_OVERRIDE_" + tag.Name
		componentOverrideImage := os.Getenv(componentEnvName)
		if len(componentOverrideImage) != 0 {
			tag.From.Name = componentOverrideImage
			logrus.Infof("Overrode release image tag %q for with env %s value %q", tag.Name, componentEnvName, componentOverrideImage)
		}
	}

	return is, releaseImage, nil
}

// ExtractImageFromReleasePayload extracts the image pull spec for a specific tag from a release payload.
// It returns the image pull spec for the specified tag or an error if the tag is not found.
func ExtractImageFromReleasePayload(releaseImage, imageTag string, oc *util.CLI) (string, error) {
	// Create a temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "release-extract")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	registryAuthFilePath, err := DetermineRegistryAuthFilePath(tmpDir, oc)
	if err != nil {
		return "", fmt.Errorf("failed to determine registry auth file path: %w", err)
	}

	// Extract the ImageStream from the release payload
	imageStream, _, err := extractReleaseImageStream(tmpDir, releaseImage, registryAuthFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to extract image references from release payload: %w", err)
	}

	// Find the specified tag in the ImageStream
	for _, tag := range imageStream.Spec.Tags {
		if tag.Name == imageTag {
			return tag.From.Name, nil
		}
	}

	return "", fmt.Errorf("image tag %q not found in release payload %q", imageTag, releaseImage)
}

func DetermineRegistryAuthFilePath(tmpDir string, oc *util.CLI) (string, error) {
	// To extract binaries bearing external tests, we must inspect the release
	// payload under tests as well as extract content from component images
	// referenced by that payload.
	// openshift-tests is frequently run in the context of a CI job, within a pod.
	// CI sets $RELEASE_IMAGE_LATEST to a pullspec for the release payload under test. This
	// pull spec resolve to:
	// 1. A build farm ci-op-* namespace / imagestream location (anonymous access permitted).
	// 2. A quay.io/openshift-release-dev location (for tests against promoted ART payloads -- anonymous access permitted).
	// 3. A registry.ci.openshift.org/ocp-<arch>/release:<tag> (request registry.ci.openshift.org token).
	// Within the pod, we don't necessarily have a pull-secret for #3 OR the component images
	// a payload references (which are private, unless in a ci-op-* imagestream).
	// We try the following options:
	// 1. If set, use the REGISTRY_AUTH_FILE environment variable to an auths file with
	//    pull secrets capable of reading appropriate payload & component image
	//    information.
	// 2. If it exists, use a file /run/secrets/ci.openshift.io/cluster-profile/pull-secret
	//    (conventional location for pull-secret information for CI cluster profile).
	// 3. Use openshift-config secret/pull-secret from the cluster-under-test, if it exists
	//    (Microshift does not).
	// 4. Use unauthenticated access to the payload image and component images.
	registryAuthFilePath := os.Getenv("REGISTRY_AUTH_FILE")

	// if the environment variable is not set, extract the target cluster's
	// platform pull secret.
	if len(registryAuthFilePath) != 0 {
		logrus.Infof("Using REGISTRY_AUTH_FILE environment variable: %v", registryAuthFilePath)
	} else {

		// See if the cluster-profile has stored a pull-secret at the conventional location.
		ciProfilePullSecretPath := "/run/secrets/ci.openshift.io/cluster-profile/pull-secret"
		_, err := os.Stat(ciProfilePullSecretPath)
		if !os.IsNotExist(err) {
			logrus.Infof("Detected %v; using cluster profile for image access", ciProfilePullSecretPath)
			registryAuthFilePath = ciProfilePullSecretPath
		} else {
			// Inspect the cluster-under-test and read its cluster pull-secret dockerconfigjson value.
			clusterPullSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
			if err != nil {
				if kapierrs.IsNotFound(err) {
					logrus.Warningf("Cluster has no openshift-config secret/pull-secret; falling back to unauthenticated image access")
				} else {
					return "", fmt.Errorf("unable to read ephemeral cluster pull secret: %w", err)
				}
			} else {
				clusterDockerConfig := clusterPullSecret.Data[".dockerconfigjson"]
				registryAuthFilePath = filepath.Join(tmpDir, ".dockerconfigjson")
				err = os.WriteFile(registryAuthFilePath, clusterDockerConfig, 0600)
				if err != nil {
					return "", fmt.Errorf("unable to serialize target cluster pull-secret locally: %w", err)
				}
				logrus.Infof("Using target cluster pull-secrets for registry auth")
			}
		}
	}

	return registryAuthFilePath, nil
}

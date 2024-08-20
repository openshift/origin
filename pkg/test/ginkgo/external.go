package ginkgo

import (
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/origin/test/extended/util"
)

type serializedTest struct {
	Name   string
	Labels string
}

// externalTestsForSuite reads tests available from external testing binaries
// carried in the release payload.
func externalTestsForSuite(ctx context.Context, releaseReferences *imagev1.ImageStream, tag string, binaryPath string) ([]*testCase, error) {
	var tests []*testCase

	testBinary, err := extractBinaryFromReleaseImage(releaseReferences, tag, binaryPath)
	if err != nil {
		return nil, fmt.Errorf("unable to extract %q binary from tag %q: %w", binaryPath, tag, err)
	}

	compat, err := checkCompatibleArchitecture(testBinary)
	if err != nil {
		return nil, fmt.Errorf("unable to check compatibility external binary %q from tag %q: %w", binaryPath, tag, err)
	}

	if !compat {
		return nil, fmt.Errorf("external binary %q from tag %q was compiled for incompatible architecture", binaryPath, tag)
	}

	command := exec.Command(testBinary, "list")
	testList, err := runWithTimeout(ctx, command, 1*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed running '%s list': %w", testBinary, err)
	}
	buf := bytes.NewBuffer(testList)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if !strings.HasPrefix(line, "[{") {
			continue
		}

		var serializedTests []serializedTest
		err = json.Unmarshal([]byte(line), &serializedTests)
		if err != nil {
			return nil, err
		}
		for _, test := range serializedTests {
			tests = append(tests, &testCase{
				name:       test.Name + test.Labels,
				rawName:    test.Name,
				binaryName: testBinary,
			})
		}
	}
	return tests, nil
}

// extractReleaseImageStream extracts image references from the current
// cluster's release payload (or image specified by EXTERNAL_BINARY_RELEASE_OVERRIDE)
// and returns an ImageStream object with tags associated with image-references
// from that payload.
func extractReleaseImageStream() (*imagev1.ImageStream, error) {
	tmpDir, err := os.MkdirTemp("", "release")
	if err != nil {
		return nil, fmt.Errorf("cannot create temporary directory for extracted binary: %w", err)
	}

	oc := util.NewCLIWithoutNamespace("default")

	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed reading ClusterVersion/version: %w", err)
	}
	releaseImage := cv.Status.Desired.Image
	if len(releaseImage) == 0 {
		return nil, fmt.Errorf("cannot determine release image from ClusterVersion resource")
	}

	// Allow testing using an overridden source for external tests.
	overrideReleaseImage := os.Getenv("EXTERNAL_BINARY_RELEASE_OVERRIDE")
	if overrideReleaseImage != "" {
		releaseImage = overrideReleaseImage
	}

	if err := runImageExtract(releaseImage, "/release-manifests/image-references", tmpDir, ""); err != nil {
		return nil, fmt.Errorf("failed extracting image-references from %q: %w", releaseImage, err)
	}
	jsonFile, err := os.Open(filepath.Join(tmpDir, "image-references"))
	if err != nil {
		return nil, fmt.Errorf("failed reading image-references from %q: %w", releaseImage, err)
	}
	defer jsonFile.Close()
	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("unable to load release image-references from %q: %w", releaseImage, err)
	}
	is := &imagev1.ImageStream{}
	if err := json.Unmarshal(data, &is); err != nil {
		return nil, fmt.Errorf("unable to load release image-references from %q: %w", releaseImage, err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return nil, fmt.Errorf("unrecognized image-references in release payload %q", releaseImage)
	}

	return is, nil
}

// extractBinaryFromReleaseImage is responsible for resolving the tag from
// release image and extracting binary, returns path to the binary or error
func extractBinaryFromReleaseImage(releaseImageReferences *imagev1.ImageStream, tag, binary string) (string, error) {

	tmpDir, err := os.MkdirTemp("", "external-binary")

	image := ""
	for _, t := range releaseImageReferences.Spec.Tags {
		if t.Name == tag {
			image = t.From.Name
			break
		}
	}

	oc := util.NewCLIWithoutNamespace("default")

	// The preceding runImageExtract was against a release payload that was created in the local
	// ci-operator namespace. Our process was free to access it. The release payload, however,
	// may be referencing images in other registries that we don't have access to (e.g. quay.io,
	// registry.ci.openshift.org). One location that does have access is in the cluster under test.
	// The cluster-wide pull-secret must have pull access to these images in order for it to have
	// installed. Read its dockerconfigjson value and use it when extracting external test binaries
	// from images referenced by the release payload.
	clusterPullSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to read ephemeral cluster pull secret: %w", err)
	}

	clusterDockerConfig := clusterPullSecret.Data[".dockerconfigjson"]
	dockerConfigJsonPath := filepath.Join(tmpDir, ".dockerconfigjson")
	err = os.WriteFile(dockerConfigJsonPath, clusterDockerConfig, 0644)
	if err != nil {
		return "", fmt.Errorf("unable to serialize ephemeral cluster pull secret locally: %w", err)
	}

	if len(image) == 0 {
		return "", fmt.Errorf("%s not found", tag)
	}
	if err := runImageExtract(image, binary, tmpDir, dockerConfigJsonPath); err != nil {
		return "", fmt.Errorf("failed extracting %q from %q: %w", binary, image, err)
	}

	extractedBinary := filepath.Join(tmpDir, filepath.Base(binary))
	// Support gzipped external binaries as they will not be flagged by FIPS scan
	// for being statically compiled.
	extractedBinary, err = ungzipFile(extractedBinary)
	if err != nil {
		return "", fmt.Errorf("failed to decompress external binary %q: %w", binary, err)
	}

	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return "", fmt.Errorf("failed making the extracted binary executable: %w", err)
	}
	return extractedBinary, nil
}

// runImageExtract extracts src from specified image to dst
func runImageExtract(image, src, dst string, dockerConfigJsonPath string) error {
	args := []string{"--kubeconfig=" + util.KubeConfigPath(), "image", "extract", image, fmt.Sprintf("--path=%s:%s", src, dst), "--confirm"}
	if len(dockerConfigJsonPath) > 0 {
		args = append(args, fmt.Sprintf("--registry-config=%s", dockerConfigJsonPath))
	}
	cmd := exec.Command("oc", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error during image extract: %w (%v)", err, string(out))
	}
	return nil
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
func checkCompatibleArchitecture(executablePath string) (bool, error) {

	file, err := os.Open(executablePath)
	if err != nil {
		return false, fmt.Errorf("failed to open ELF file: %w", err)
	}
	defer file.Close()

	elfFile, err := elf.NewFile(file)
	if err != nil {
		return false, fmt.Errorf("failed to parse ELF file: %w", err)
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
		return false, fmt.Errorf("unsupported host architecture: %s", runtime.GOARCH)
	}

	if elfArch == expectedArch {
		return true, nil
	}

	return false, nil
}

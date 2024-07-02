package ginkgo

import (
	"bytes"
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/origin/test/extended/util"
)

type serializedTest struct {
	Name   string
	Labels string
}

// externalTestsForSuite reads tests from external binary, currently only
// k8s-tests is supported
func externalTestsForSuite(ctx context.Context) ([]*testCase, error) {
	var tests []*testCase

	// TODO: add support for binaries from other images
	testBinary, err := extractBinaryFromReleaseImage("hyperkube", "/usr/bin/k8s-tests")
	if err != nil {
		return nil, fmt.Errorf("unable to extract k8s-tests binary: %w", err)
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
		serializedTests := []serializedTest{}
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

// extractBinaryFromReleaseImage is responsible for resolving the tag from
// release image and extracting binary, returns path to the binary or error
func extractBinaryFromReleaseImage(tag, binary string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "release")
	if err != nil {
		return "", fmt.Errorf("cannot create temporary directory for extracted binary: %w", err)
	}

	oc := util.NewCLIWithoutNamespace("default")
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed reading ClusterVersion/version: %w", err)
	}
	releaseImage := cv.Status.Desired.Image
	if len(releaseImage) == 0 {
		return "", fmt.Errorf("cannot determine release image from ClusterVersion resource")
	}

	if err := runImageExtract(releaseImage, "/release-manifests/image-references", tmpDir, ""); err != nil {
		return "", fmt.Errorf("failed extracting image-references: %w", err)
	}
	jsonFile, err := os.Open(filepath.Join(tmpDir, "image-references"))
	if err != nil {
		return "", fmt.Errorf("failed reading image-references: %w", err)
	}
	defer jsonFile.Close()
	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return "", fmt.Errorf("unable to load release image-references: %w", err)
	}
	is := &imagev1.ImageStream{}
	if err := json.Unmarshal(data, &is); err != nil {
		return "", fmt.Errorf("unable to load release image-references: %w", err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return "", fmt.Errorf("unrecognized image-references in release payload")
	}

	image := ""
	for _, t := range is.Spec.Tags {
		if t.Name == tag {
			image = t.From.Name
			break
		}
	}

	// The preceding runImageExtract was against a release payload that was created in the local
	// ci-operator namespace. Our process was free to access it. The release payload, however,
	// may be referencing images in other registries that we don't have access to (e.g. quay.io,
	// registry.ci.openshift.org). One location that does have access is in the cluster under test.
	// The cluster-wide pull-secret must have pull access to these images in order for it to have
	// installed. Read its dockerconfigjson value and use it when extracting external test binaries
	// from images referenced by the release payload.
	clusterPullSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to read ephemeral cluster pull secret: %v", err)
	}

	clusterDockerConfig := clusterPullSecret.Data[".dockerconfigjson"]
	dockerConfigJsonPath := filepath.Join(tmpDir, ".dockerconfigjson")
	err = os.WriteFile(dockerConfigJsonPath, clusterDockerConfig, 0644)
	if err != nil {
		return "", fmt.Errorf("unable to serialize ephemeral cluster pull secret locally: %v", err)
	}

	if len(image) == 0 {
		return "", fmt.Errorf("%s not found", tag)
	}
	if err := runImageExtract(image, binary, tmpDir, dockerConfigJsonPath); err != nil {
		return "", fmt.Errorf("failed extracting %q from %q: %w", binary, image, err)
	}

	extractedBinary := filepath.Join(tmpDir, filepath.Base(binary))
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

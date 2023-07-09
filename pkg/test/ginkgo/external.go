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

	if err := runImageExtract(releaseImage, "/release-manifests/image-references", tmpDir); err != nil {
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
	if len(image) == 0 {
		return "", fmt.Errorf("%s not found", tag)
	}
	if err := runImageExtract(image, binary, tmpDir); err != nil {
		return "", fmt.Errorf("failed extracting %q from %q: %w", binary, image, err)
	}

	extractedBinary := filepath.Join(tmpDir, filepath.Base(binary))
	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return "", fmt.Errorf("failed making the extracted binary executable: %w", err)
	}
	return extractedBinary, nil
}

// runImageExtract extracts src from specified image to dst
func runImageExtract(image, src, dst string) error {
	cmd := exec.Command("oc", "--kubeconfig="+util.KubeConfigPath(), "image", "extract", image, fmt.Sprintf("--path=%s:%s", src, dst), "--confirm")
	return cmd.Run()
}

package compat_otp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/tidwall/gjson"

	"github.com/coreos/stream-metadata-go/stream"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type CoreOSImageArch string

const (
	CoreOSBootImagesFile                      = "0000_50_installer_coreos-bootimages.yaml"
	CoreOSBootImageArchX86_64 CoreOSImageArch = "x86_64"

	ReleaseImageLatestEnv = "RELEASE_IMAGE_LATEST"
)

func (a CoreOSImageArch) String() string {
	return string(a)
}

// ExtractCoreOSBootImagesConfigMap extracts the CoreOS boot images ConfigMap from the given release image
func ExtractCoreOSBootImagesConfigMap(oc *exutil.CLI, releaseImage, pullSecretFile string) (*corev1.ConfigMap, error) {
	stdout, _, err := oc.AsAdmin().WithoutNamespace().Run("adm", "release", "extract").Args(releaseImage, "--file", CoreOSBootImagesFile, "-a", pullSecretFile).Outputs()
	if err != nil {
		return nil, fmt.Errorf("failed to extract CoreOS boot images from release image: %v", err)
	}

	var coreOSBootImagesCM corev1.ConfigMap
	if err = yaml.Unmarshal([]byte(stdout), &coreOSBootImagesCM); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CoreOS boot images file content: %v", err)
	}

	return &coreOSBootImagesCM, nil
}

// GetRHCOSImageURLForAzureDisk retrieves the RHCOS URL for the specified architecture's Azure disk image
func GetRHCOSImageURLForAzureDisk(oc *exutil.CLI, releaseImage string, pullSecretFile string, arch CoreOSImageArch) (string, error) {
	coreOSBootImagesCM, err := ExtractCoreOSBootImagesConfigMap(oc, releaseImage, pullSecretFile)
	if err != nil {
		return "", fmt.Errorf("failed to extract CoreOS boot images ConfigMap: %v", err)
	}

	var coreOSBootImagesStream stream.Stream
	if err = json.Unmarshal([]byte(coreOSBootImagesCM.Data["stream"]), &coreOSBootImagesStream); err != nil {
		return "", fmt.Errorf("failed to unmarshal CoreOS bootimages stream data: %v", err)
	}

	return coreOSBootImagesStream.Architectures[arch.String()].RHELCoreOSExtensions.AzureDisk.URL, nil
}

func GetLatestReleaseImageFromEnv() string {
	return os.Getenv(ReleaseImageLatestEnv)
}

// GetLatest4StableImage to get the latest 4-stable OCP image from releasestream link
// Return OCP image for sample quay.io/openshift-release-dev/ocp-release:4.11.0-fc.0-x86_64
func GetLatest4StableImage() (string, error) {
	outputCmd, err := exec.Command("bash", "-c", "curl -s -k https://amd64.ocp.releases.ci.openshift.org/api/v1/releasestream/4-stable/latest").Output()
	if err != nil {
		e2e.Logf("Encountered err: %v when trying to curl the releasestream page", err)
		return "", err
	}
	latestImage := gjson.Get(string(outputCmd), `pullSpec`).String()
	e2e.Logf("The latest 4-stable OCP image is %s", latestImage)
	return latestImage, nil
}

// GetLatest4StableImageByStream to get the latest 4-stable OCP image from a specifed releasestream link
// GetLatest4StableImageByStream("multi", "4-stable-multi/latest?in=>4.16.0-0+<4.17.0-0")
// GetLatest4StableImageByStream("amd64", "4-stable/latest")
func GetLatest4StableImageByStream(arch string, stream string) (latestImage string, err error) {
	url := fmt.Sprintf("https://%s.ocp.releases.ci.openshift.org/api/v1/releasestream/%s", arch, stream)
	var resp *http.Response
	var body []byte
	resp, err = http.Get(url)
	if err != nil {
		err = fmt.Errorf("fail to get url %s, error: %v", url, err)
		return "", err
	}
	body, err = io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		err = fmt.Errorf("fail to parse the result, error: %v", err)
		return "", err
	}
	latestImage = gjson.Get(string(body), `pullSpec`).String()
	e2e.Logf("The latest 4-stable OCP image is %s", latestImage)
	return latestImage, err
}

// GetLatestNightlyImage to get the latest nightly OCP image from releasestream link
// Input parameter release: OCP release version such as 4.11, 4.9, ..., 4.6
// Return OCP image
func GetLatestNightlyImage(release string) (string, error) {
	var url string
	switch release {
	case "4.19", "4.18", "4.17", "4.16", "4.15", "4.14", "4.13", "4.12", "4.11", "4.10", "4.9", "4.8", "4.7", "4.6":
		url = "https://amd64.ocp.releases.ci.openshift.org/api/v1/releasestream/" + release + ".0-0.nightly/latest"
	default:
		e2e.Logf("Inputted release version %s is not supported. Only versions from 4.16 to 4.6 are supported.", release)
		return "", errors.New("not supported version of payload")
	}
	outputCmd, err := exec.Command("bash", "-c", "curl -s -k "+url).Output()
	if err != nil {
		e2e.Logf("Encountered err: %v when trying to curl the releasestream page", err)
		return "", err
	}
	latestImage := gjson.Get(string(outputCmd), `pullSpec`).String()
	e2e.Logf("The latest nightly OCP image for %s is: %s", release, latestImage)
	return latestImage, nil
}

// GetLatestImage retrieves the pull spec of the latest image satisfying the arch - product - stream combination.
// arch = "amd64", "arm64", "ppc64le", "s390x", "multi"
// product = "ocp", "origin" (i.e. okd, which only supports the amd64 architecture)
// Possible values for the stream parameter depend on arch and product.
// See https://docs.ci.openshift.org/docs/getting-started/useful-links/#services for relevant release status pages.
//
// Examples:
// GetLatestImage("amd64", "ocp", "4.14.0-0.nightly")
// GetLatestImage("arm64", "ocp", "4.14.0-0.nightly-arm64")
// GetLatestImage("amd64", "origin", "4.14.0-0.okd")
func GetLatestImage(arch, product, stream string) (string, error) {
	switch arch {
	case "amd64", "arm64", "ppc64le", "s390x", "multi":
	default:
		return "", fmt.Errorf("unsupported architecture %v", arch)
	}

	switch product {
	case "ocp", "origin":
	default:
		return "", fmt.Errorf("unsupported product %v", product)
	}

	switch {
	case product == "ocp":
	case product == "origin" && arch == "amd64":
	default:
		return "", fmt.Errorf("the product - architecture combination: %v - %v is not supported", product, arch)
	}

	url := fmt.Sprintf("https://%v.%v.releases.ci.openshift.org/api/v1/releasestream/%v/latest",
		arch, product, stream)
	stdout, err := exec.Command("bash", "-c", "curl -s -k "+url).Output()
	if err != nil {
		return "", err
	}
	if !gjson.ValidBytes(stdout) {
		return "", errors.New("curl does not return a valid json")
	}
	latestImage := gjson.GetBytes(stdout, "pullSpec").String()
	e2e.Logf("Found latest image %v for architecture %v, product %v and stream %v", latestImage, arch, product, stream)
	return latestImage, nil
}

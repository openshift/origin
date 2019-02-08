package release

// This package parses the HTTP API effectively
// created by https://github.com/coreos/coreos-assembler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// BuildMeta is a partial deserialization of the `meta.json` generated
// by coreos-assembler for a build.
type BuildMeta struct {
	AMIs []struct {
		HVM  string `json:"hvm"`
		Name string `json:"name"`
	} `json:"amis"`
	BuildID string `json:"buildid"`
	Images struct {
		QEMU struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"qemu"`
	} `json:"images"`
	OSTreeVersion string `json:"ostree-version"`
	OSContainer struct {
		Digest string `json:"digest"`
		Image string `json:"image"`
	} `json:"oscontainer"`
}

// httpGetAll downloads a URL and gives you a byte array.
func httpGetAll(ctx context.Context, url string) ([]byte, error) {
	var body []byte
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return body, errors.Wrap(err, "failed to build request")
	}

	client := &http.Client{}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return body, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return body, errors.Errorf("fetching %s status %s", url, resp.Status)
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, errors.Wrap(err, "failed to read HTTP response")
	}

	return body, nil
}

// getLatestBuildVersion returns the latest CoreOS build version number
func getLatestBuildVersion(ctx context.Context, baseURL string) (string, error) {
	var builds struct {
		Builds []string `json:"builds"`
	}
	buildsBuf, err := httpGetAll(ctx, baseURL + "/builds.json")
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(buildsBuf, &builds); err != nil {
		return "", errors.Wrap(err, "failed to parse HTTP response")
	}

	if len(builds.Builds) == 0 {
		return "", errors.Errorf("no builds found")
	}

	return builds.Builds[0], nil
}

// GetLatest returns the CoreOS build with target version.  If version
// is the empty string, the latest will be used.
func GetCoreOSBuild(ctx context.Context, baseURL string, version string) (*BuildMeta, error) {
	var err error
	if version == "" {
		version, err = getLatestBuildVersion(ctx, baseURL)
		if err != nil {
			return nil, err
		}
	}
	buildUrl := fmt.Sprintf("%s/%s/meta.json", baseURL, version)
	buildStr, err := httpGetAll(ctx, buildUrl)
	if err != nil {
		return nil, err
	}

	var build BuildMeta
	if err := json.Unmarshal(buildStr, &build); err != nil {
		return nil, errors.Wrap(err, "failed to parse HTTP response")
	}
	return &build, nil
}

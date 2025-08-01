// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package releasesjson

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/internal/httpclient"
)

const defaultBaseURL = "https://releases.hashicorp.com"

// Product is a top-level product like "Consul" or "Nomad". A Product may have
// one or more versions.
type Product struct {
	Name     string             `json:"name"`
	Versions ProductVersionsMap `json:"versions"`
}

type ProductBuilds []*ProductBuild

func (pbs ProductBuilds) FilterBuild(os string, arch string, suffix string) (*ProductBuild, bool) {
	for _, pb := range pbs {
		if pb.OS == os && pb.Arch == arch && strings.HasSuffix(pb.Filename, suffix) {
			return pb, true
		}
	}
	return nil, false
}

// ProductBuild is an OS/arch-specific representation of a product. This is the
// actual file that a user would download, like "consul_0.5.1_linux_amd64".
type ProductBuild struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type Releases struct {
	logger  *log.Logger
	BaseURL string
}

func NewReleases() *Releases {
	return &Releases{
		logger:  log.New(io.Discard, "", 0),
		BaseURL: defaultBaseURL,
	}
}

func (r *Releases) SetLogger(logger *log.Logger) {
	r.logger = logger
}

func (r *Releases) ListProductVersions(ctx context.Context, productName string) (ProductVersionsMap, error) {
	client := httpclient.NewHTTPClient(r.logger)

	productIndexURL := fmt.Sprintf("%s/%s/index.json",
		r.BaseURL,
		url.PathEscape(productName))
	r.logger.Printf("requesting versions from %s", productIndexURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, productIndexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %q: %w", productIndexURL, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to obtain product versions from %q: %s ",
			productIndexURL, resp.Status)
	}

	contentType := resp.Header.Get("content-type")
	if contentType != "application/json" && contentType != "application/vnd+hashicorp.releases-api.v0+json" {
		return nil, fmt.Errorf("unexpected Content-Type: %q", contentType)
	}

	defer resp.Body.Close()

	r.logger.Printf("received %s", resp.Status)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	p := Product{}
	err = json.Unmarshal(body, &p)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal: %q",
			err, string(body))
	}

	for rawVersion := range p.Versions {
		v, err := version.NewVersion(rawVersion)
		if err != nil {
			// remove unparseable version
			delete(p.Versions, rawVersion)
			continue
		}

		p.Versions[rawVersion].Version = v
	}

	return p.Versions, nil
}

func (r *Releases) GetProductVersion(ctx context.Context, product string, version *version.Version) (*ProductVersion, error) {
	client := httpclient.NewHTTPClient(r.logger)

	indexURL := fmt.Sprintf("%s/%s/%s/index.json",
		r.BaseURL,
		url.PathEscape(product),
		url.PathEscape(version.String()))
	r.logger.Printf("requesting version from %s", indexURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %q: %w", indexURL, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to obtain product version from %q: %s ",
			indexURL, resp.Status)
	}

	contentType := resp.Header.Get("content-type")
	if contentType != "application/json" && contentType != "application/vnd+hashicorp.releases-api.v0+json" {
		return nil, fmt.Errorf("unexpected Content-Type: %q", contentType)
	}

	defer resp.Body.Close()

	r.logger.Printf("received %s", resp.Status)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	pv := &ProductVersion{}
	err = json.Unmarshal(body, pv)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal response: %q",
			err, string(body))
	}

	return pv, nil
}

package sampler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// NewNamespaceRequestor returns a new Requestor instance that
// creates a new http.Request to query the default namespace:
//
//	GET namespaces/default
func NewNamespaceRequestor(host string) namespaceGetter {
	return namespaceGetter{host: host}
}

// NewNamespaceRequestor returns a new Requestor instance that
// creates a new http.Request for a given path:
func NewHostPathRequestor(host, path string) hostpath {
	return hostpath{host: host, path: path}
}

type namespaceGetter struct {
	host string
}

// to support legacy BackendSampler
func (n namespaceGetter) GetBaseURL() string {
	return fmt.Sprintf("%s/api/v1/namespaces/default", n.host)
}

func (n namespaceGetter) NewHTTPRequest(ctx context.Context, sampleID uint64) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/default?sample-id=%d", n.host, sampleID)
	return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}

type hostpath struct {
	host string
	path string
}

// to support legacy BackendSampler
func (n hostpath) GetBaseURL() string {
	return fmt.Sprintf("%s%s", n.host, n.path)
}

func (n hostpath) NewHTTPRequest(ctx context.Context, sampleID uint64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.host+n.path, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("sample-id", strconv.FormatUint(sampleID, 10))
	req.URL.RawQuery = q.Encode()
	return req, nil
}

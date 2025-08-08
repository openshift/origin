package nutanix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	libraryVersion         = "v3"
	absolutePath           = "api/nutanix/" + libraryVersion
	defaultV2BaseURL       = "PrismGateway/services/rest/v2.0"
	userAgent              = "nutanix/" + "cmd.Version"
	itemsPerPage     int64 = 500
	mediaTypeJSON          = "application/json"
	mediaTypeUpload        = "application/octet-stream"
)

// ClientOption ...
type ClientOption func(*Client)

// Client Config Configuration of the client
type Client struct {
	baseURL     *url.URL
	credentials *Credentials
	httpClient  *http.Client
	userAgent   string
	skipVerify  bool
	debugWriter io.Writer

	Image            ImageClient
	Cluster          ClusterClient
	Project          ProjectClient
	VM               VMClient
	Subnet           SubnetClient
	Host             HostClient
	Category         CategoryClient
	Task             TaskClient
	Snapshot         SnapshotClient
	AvailabilityZone AvailabilityZoneClient
	VMRecoveryPoint  VMRecoveryPointClient
	VPC              VpcClient
	FlotatingIP      FloatingIPClient
	RoutingPolicy    RoutingPolicyClient
}

// Credentials needed username and password
type Credentials struct {
	Username string
	Password string
}

// WithCredentials configures a Client to use the specified credentials for authentication.
func WithCredentials(cred *Credentials) ClientOption {
	return func(client *Client) {
		client.credentials = cred
	}
}

// WithEndpoint configures a Client to use the specified credentials for authentication.
func WithEndpoint(endpoint string) ClientOption {
	return func(client *Client) {
		passedURL := endpoint

		// Required because url.Parse returns an empty string for the hostname if there was no schema
		if !strings.HasPrefix(passedURL, "https://") && !strings.HasPrefix(passedURL, "http://") {
			passedURL = "https://" + passedURL
		}

		client.baseURL, _ = url.Parse(passedURL)
	}
}

// WithHTTPClient allows to specify a custom http client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// WithSkipVerify returns a ClientOption that configure the client connection to not verify https connectins
func WithSkipVerify() ClientOption {
	return func(client *Client) {
		client.skipVerify = true
	}
}

// WithDebugWriter configure a custom writer to debug request and responses
func WithDebugWriter(debugWriter io.Writer) ClientOption {
	return func(client *Client) {
		client.debugWriter = debugWriter
	}
}

// NewClient creates a new client.
func NewClient(options ...ClientOption) *Client {
	client := &Client{}

	for _, option := range options {
		option(client)
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{}
	}

	client.userAgent = userAgent

	transCfg := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: client.skipVerify},
		MaxConnsPerHost:       1000,
		MaxIdleConns:          100,
		ForceAttemptHTTP2:     true,
		ExpectContinueTimeout: 1 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
	}

	client.httpClient.Transport = transCfg

	client.Image = ImageClient{client: client}
	client.Cluster = ClusterClient{client: client}
	client.Project = ProjectClient{client: client}
	client.VM = VMClient{client: client}
	client.Subnet = SubnetClient{client: client}
	client.Category = CategoryClient{client: client}
	client.Task = TaskClient{client: client}
	client.Host = HostClient{client: client}
	client.Snapshot = SnapshotClient{client: client}
	client.AvailabilityZone = AvailabilityZoneClient{client: client}
	client.VMRecoveryPoint = VMRecoveryPointClient{client: client}
	client.VPC = VpcClient{client: client}
	client.FlotatingIP = FloatingIPClient{client: client}
	client.RoutingPolicy = RoutingPolicyClient{client: client}
	return client
}

// Do performs request passed
func (c *Client) Do(r *http.Request, v interface{}) error {
	if c.debugWriter != nil {
		dumpReq, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Request:\n%s\n\n", dumpReq)
	}

	resp, err := c.httpClient.Do(r)
	if err != nil {
		select {
		case <-r.Context().Done():
			return r.Context().Err()
		default:
		}

		return err
	}

	if c.debugWriter != nil {
		dumpResp, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Response:\n%s\n\n", dumpResp)
	}

	defer func() {
		if rerr := resp.Body.Close(); err == nil {
			err = rerr
		}
	}()

	err = checkResponse(resp)
	if err != nil {
		return err
	}
	if v != nil {
		if w, ok := v.(io.Writer); ok {
			_, err = io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err == io.EOF {
				err = nil // ignore EOF errors caused by empty response body
			}
		}
	}

	return err
}

func checkResponse(r *http.Response) error {
	if c := r.StatusCode; c >= 200 && c <= 299 && r.Request.Method == http.MethodDelete {
		return nil
	}

	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if r.StatusCode >= 500 || r.StatusCode == 401 || r.StatusCode == 404 {
		return fmt.Errorf("statusCode: %d, response: %s", r.StatusCode, string(buf))
	}

	data := io.NopCloser(bytes.NewBuffer(buf))

	r.Body = data

	// if has entities -> return nil
	// if has message_list -> check_error["state"]
	// if has status -> check_error["status.state"]
	if len(buf) == 0 {
		return nil
	}

	var res map[string]interface{}

	err = json.Unmarshal(buf, &res)
	if err != nil {
		return errors.Wrap(err, "unmarshalling error response")
	}

	errRes := &schema.ErrorResponse{}

	if status, ok := res["status"]; ok {
		_, sok := status.(string)
		if sok {
			return nil
		}
		err = fillStruct(status.(map[string]interface{}), errRes)
	} else if _, ok := res["state"]; ok {
		err = fillStruct(res, errRes)
	} else if _, ok := res["entities"]; ok {
		return nil
	}

	if err != nil {
		return err
	}

	if errRes.State != "ERROR" {
		return nil
	}

	pretty, _ := json.MarshalIndent(errRes, "", "  ")
	return fmt.Errorf(string(pretty))
}

func (c *Client) setHeaders(req *http.Request) {
	req.SetBasicAuth(c.credentials.Username, c.credentials.Password)
	req.Header.Set("User-Agent", c.userAgent)
}

// NewV3PCRequest ...
func (c *Client) NewV3PCRequest(ctx context.Context, method string, path string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(absolutePath + path)
	if err != nil {
		return nil, err
	}
	url := c.baseURL.ResolveReference(rel)
	return c.newV3Request(ctx, method, url, body)
}

// NewV3PERequest ...
func (c *Client) NewV3PERequest(ctx context.Context, method string, clusterUUID string, path string, body interface{}) (*http.Request, error) {
	cluster, err := c.Cluster.GetByUUID(ctx, clusterUUID)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(absolutePath + path)
	if err != nil {
		return nil, err
	}

	urlEndpoint, _ := url.Parse(fmt.Sprintf("%s://%s:%s", c.baseURL.Scheme, cluster.Spec.Resources.Network.ExternalIP, c.baseURL.Port()))

	url := urlEndpoint.ResolveReference(rel)
	return c.newV3Request(ctx, method, url, body)
}

func (c *Client) newV3Request(ctx context.Context, method string, url *url.URL, body interface{}) (*http.Request, error) {
	var contentBody io.Reader
	var contentType string

	if body != nil {
		switch b := body.(type) {
		case *schema.File:
			contentType = b.ContentType
			contentBody = b.Body
		default:
			buf, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			contentType = mediaTypeJSON
			contentBody = bytes.NewReader(buf)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url.String(), contentBody)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)

	req = req.WithContext(ctx)

	return req, nil
}

// NewV2PERequest ...
func (c *Client) NewV2PERequest(ctx context.Context, method string, clusterUUID string, path string, body io.Reader) (*http.Request, error) {
	cluster, err := c.Cluster.GetByUUID(ctx, clusterUUID)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(defaultV2BaseURL + path)
	if err != nil {
		return nil, err
	}

	urlEndpoint, _ := url.Parse(fmt.Sprintf("%s://%s:%s", c.baseURL.Scheme, cluster.Spec.Resources.Network.ExternalIP, c.baseURL.Port()))

	url := urlEndpoint.ResolveReference(rel)
	return c.newV2Request(ctx, method, url, body)
}

// NewV2PCRequest ...
func (c *Client) NewV2PCRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	rel, err := url.Parse(defaultV2BaseURL + path)
	if err != nil {
		return nil, err
	}
	url := c.baseURL.ResolveReference(rel)
	return c.newV2Request(ctx, method, url, body)
}

// NewV2PERequest ...
func (c *Client) newV2Request(ctx context.Context, method string, url *url.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	req.Header.Set("Content-Type", mediaTypeJSON)
	req.Header.Set("Accept", mediaTypeJSON)

	req = req.WithContext(ctx)

	return req, nil
}

func fillStruct(data map[string]interface{}, result interface{}) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(j, result)
}

func (c *Client) requestHelper(ctx context.Context, path, method string, request interface{}, output interface{}) error {
	req, err := c.NewV3PCRequest(ctx, method, path, request)
	if err != nil {
		return err
	}

	err = c.Do(req, &output)
	if err != nil {
		return err
	}

	return nil
}

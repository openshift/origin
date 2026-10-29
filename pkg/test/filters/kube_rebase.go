package filters

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/test/extensions"
)

// KubeRebaseTestsFilter filters out tests during k8s rebase
type KubeRebaseTestsFilter struct {
	restConfig  *rest.Config
	retryPolicy serverVersionRetryPolicy
}

type serverVersionRetryPolicy struct {
	backoff wait.Backoff
	timeout time.Duration
}

func NewKubeRebaseTestsFilter(restConfig *rest.Config) *KubeRebaseTestsFilter {
	return &KubeRebaseTestsFilter{
		restConfig: restConfig,
		retryPolicy: serverVersionRetryPolicy{
			backoff: wait.Backoff{
				Duration: time.Second,
				Factor:   2,
				Jitter:   0.1,
				Steps:    3,
			},
			timeout: time.Minute,
		},
	}
}

func (f *KubeRebaseTestsFilter) Name() string {
	return "kube-rebase-tests"
}

func (f *KubeRebaseTestsFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if f.restConfig == nil {
		return tests, nil
	}

	httpClient, err := rest.HTTPClientFor(f.restConfig)
	if err != nil {
		return nil, err
	}
	httpClientCopy := *httpClient
	httpClientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(f.restConfig, &httpClientCopy)
	if err != nil {
		return nil, err
	}
	serverVersion, err := f.serverVersionWithRetry(ctx, discoveryClient)
	if err != nil {
		return nil, err
	}

	// TODO: this version along with below exclusions lists needs to be updated
	// for the rebase in-progress.
	if !strings.HasPrefix(serverVersion.Minor, "31") {
		return tests, nil
	}

	// Below list should only be filled in when we're trying to land k8s rebase.
	// Don't pile them up!
	exclusions := []string{
		// affected by the available controller split https://github.com/kubernetes/kubernetes/pull/126149
		`[sig-api-machinery] health handlers should contain necessary checks`,
	}

	matches := make(extensions.ExtensionTestSpecs, 0, len(tests))
outerLoop:
	for _, test := range tests {
		for _, excl := range exclusions {
			if strings.Contains(test.Name, excl) {
				logrus.Infof("Skipping %q due to rebase in-progress", test.Name)
				continue outerLoop
			}
		}
		matches = append(matches, test)
	}
	return matches, nil
}

func (f *KubeRebaseTestsFilter) ShouldApply() bool {
	return f.restConfig != nil
}

func (f *KubeRebaseTestsFilter) serverVersionWithRetry(ctx context.Context, discoveryClient *discovery.DiscoveryClient) (*version.Info, error) {
	retryCtx, cancel := context.WithTimeout(ctx, f.retryPolicy.timeout)
	defer cancel()

	var serverVersion *version.Info
	var lastErr error
	waitErr := wait.ExponentialBackoffWithContext(retryCtx, f.retryPolicy.backoff, func(attemptCtx context.Context) (bool, error) {
		serverVersion, lastErr = serverVersionOnce(attemptCtx, discoveryClient)
		if lastErr == nil {
			return true, nil
		}
		if attemptCtx.Err() != nil || !isTransientServerVersionError(lastErr) {
			return false, lastErr
		}
		return false, nil
	})
	if waitErr == nil {
		return serverVersion, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, waitErr
}

func serverVersionOnce(ctx context.Context, discoveryClient *discovery.DiscoveryClient) (*version.Info, error) {
	body, err := discoveryClient.RESTClient().Get().AbsPath("/version").MaxRetries(0).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	serverVersion := &version.Info{}
	if err := json.Unmarshal(body, serverVersion); err != nil {
		return nil, fmt.Errorf("unable to parse the server version: %v", err)
	}
	return serverVersion, nil
}

func isTransientServerVersionError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || isTLSError(err) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) {
		return true
	}

	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) && statusErr.Status().Code == http.StatusBadGateway {
		return true
	}

	return utilnet.IsTimeout(err) || utilnet.IsConnectionRefused(err) ||
		utilnet.IsConnectionReset(err) || utilnet.IsHTTP2ConnectionLost(err) ||
		utilnet.IsProbableEOF(err)
}

func isTLSError(err error) bool {
	var certificateInvalid x509.CertificateInvalidError
	var hostnameError x509.HostnameError
	var recordHeaderError tls.RecordHeaderError
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &certificateInvalid) || errors.As(err, &hostnameError) ||
		errors.As(err, &recordHeaderError) || errors.As(err, &unknownAuthority) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "tls:") || strings.Contains(message, "tls handshake") ||
		strings.Contains(message, "x509:")
}

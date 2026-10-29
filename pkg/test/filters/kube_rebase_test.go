package filters

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/test/extensions"
)

func TestKubeRebaseFilterNilConfig(t *testing.T) {
	tests := kubeRebaseTestSpecs()

	result, err := NewKubeRebaseTestsFilter(nil).Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Equal(t, tests, result)
}

func TestKubeRebaseFilterHealthyServerUsesOneRequest(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		return serverVersionResponse(request, http.StatusOK, `{"major":"1","minor":"32"}`), nil
	})

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int32(1), requests.Load())
}

func TestKubeRebaseFilterPreservesUserAgent(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		expected   string
	}{
		{name: "default", expected: rest.DefaultKubernetesUserAgent()},
		{name: "explicit", configured: "kube-rebase-test", expected: "kube-rebase-test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual string
			filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
				actual = request.Header.Get("User-Agent")
				return serverVersionResponse(request, http.StatusOK, `{"major":"1","minor":"32"}`), nil
			})
			filter.restConfig.UserAgent = test.configured

			_, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestKubeRebaseFilterRetriesTimeoutThenSucceeds(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			return nil, context.DeadlineExceeded
		}
		return serverVersionResponse(request, http.StatusOK, `{"major":"1","minor":"32"}`), nil
	})
	filter.retryPolicy.backoff.Duration = 0

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int32(2), requests.Load())
}

func TestKubeRebaseFilterRetriesUnexpectedEOFThenSucceeds(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			response := serverVersionResponse(request, http.StatusOK, "")
			response.Body = &unexpectedEOFBody{payload: []byte(`{"major":"1"`)}
			return response, nil
		}
		return serverVersionResponse(request, http.StatusOK, `{"major":"1","minor":"32"}`), nil
	})
	filter.retryPolicy.backoff.Duration = 0

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int32(2), requests.Load())
}

func TestKubeRebaseFilterRetriesHTTP2StreamErrorThenSucceeds(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			response := serverVersionResponse(request, http.StatusOK, "")
			response.Body = http2StreamErrorBody{}
			return response, nil
		}
		return serverVersionResponse(request, http.StatusOK, `{"major":"1","minor":"32"}`), nil
	})
	filter.retryPolicy.backoff.Duration = 0

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int32(2), requests.Load())
}

func TestKubeRebaseFilterContinuousTimeoutStopsAtAttemptCeiling(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, context.DeadlineExceeded
	})
	filter.retryPolicy.backoff.Duration = 0

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(3), requests.Load())
}

func TestKubeRebaseFilterRetryAfterStopsAtAttemptCeiling(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		response := serverVersionResponse(request, http.StatusServiceUnavailable, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"ServiceUnavailable","code":503}`)
		response.Header.Set("Retry-After", "0")
		return response, nil
	})
	filter.retryPolicy.backoff.Duration = 0

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, int32(3), requests.Load())
}

func TestKubeRebaseFilterDoesNotFollowRedirects(t *testing.T) {
	var requests atomic.Int32
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		response := serverVersionResponse(request, http.StatusTemporaryRedirect, "")
		response.Header.Set("Location", "/version")
		return response, nil
	})

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, int32(1), requests.Load())
}

func TestKubeRebaseFilterDoesNotRetryNonTransientErrors(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTripFunc
	}{
		{
			name: "unauthorized",
			transport: func(request *http.Request) (*http.Response, error) {
				return serverVersionResponse(request, http.StatusUnauthorized, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Unauthorized","code":401}`), nil
			},
		},
		{
			name: "forbidden",
			transport: func(request *http.Request) (*http.Response, error) {
				return serverVersionResponse(request, http.StatusForbidden, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`), nil
			},
		},
		{
			name: "TLS failure",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}
			},
		},
		{
			name: "TLS handshake timeout",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, tlsHandshakeTimeoutError{}
			},
		},
		{
			name: "malformed version response",
			transport: func(request *http.Request) (*http.Response, error) {
				return serverVersionResponse(request, http.StatusOK, `not-json`), nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
				requests.Add(1)
				return test.transport(request)
			})
			filter.retryPolicy.backoff.Duration = 0

			result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Equal(t, int32(1), requests.Load())
		})
	}
}

func TestKubeRebaseFilterMalformedConfigFailsBeforeRequest(t *testing.T) {
	filter := NewKubeRebaseTestsFilter(&rest.Config{Host: "://not-a-url"})

	result, err := filter.Filter(context.Background(), kubeRebaseTestSpecs())

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestKubeRebaseFilterCancellationStopsInFlightRequest(t *testing.T) {
	var requests atomic.Int32
	requestStarted := make(chan struct{})
	filter := newKubeRebaseFilterForTransport(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		close(requestStarted)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-requestStarted
		cancel()
	}()
	result, err := filter.Filter(ctx, kubeRebaseTestSpecs())

	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, result)
	assert.Equal(t, int32(1), requests.Load())
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type tlsHandshakeTimeoutError struct{}

func (tlsHandshakeTimeoutError) Error() string { return "net/http: TLS handshake timeout" }
func (tlsHandshakeTimeoutError) Timeout() bool { return true }

type unexpectedEOFBody struct {
	payload []byte
	read    bool
}

func (b *unexpectedEOFBody) Read(buffer []byte) (int, error) {
	if b.read {
		return 0, io.EOF
	}
	b.read = true
	return copy(buffer, b.payload), io.ErrUnexpectedEOF
}

func (*unexpectedEOFBody) Close() error { return nil }

type http2StreamErrorBody struct{}

func (http2StreamErrorBody) Read([]byte) (int, error) {
	return 0, http2.StreamError{StreamID: 1, Code: http2.ErrCodeCancel}
}

func (http2StreamErrorBody) Close() error { return nil }

func newKubeRebaseFilterForTransport(transport roundTripFunc) *KubeRebaseTestsFilter {
	return NewKubeRebaseTestsFilter(&rest.Config{
		Host: "https://kube-rebase-test.invalid",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return transport
		},
	})
}

func serverVersionResponse(request *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		Status:        http.StatusText(statusCode),
		StatusCode:    statusCode,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}
}

func kubeRebaseTestSpecs() extensions.ExtensionTestSpecs {
	return extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "[sig-api-machinery] health handlers should contain necessary checks"}},
	}
}

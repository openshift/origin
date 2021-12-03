package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/tools/events"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/client-go/transport"
)

// this entire file should be a separate package with disruption_***, but we are entanged because the sampler lives in monitor
// and the things being started by the monitor are coupled into .Start.
// we also got stuck on writing the disruption backends.  We need a way to track which disruption checks we have started,
// so we can properly write out "zero"

type BackendConnectionType string

const (
	NewConnectionType    BackendConnectionType = "new"
	ReusedConnectionType BackendConnectionType = "reused"
)

type BackendSampler struct {
	locator               string
	disruptionBackendName string
	connectionType        BackendConnectionType
	host                  string
	path                  string

	routeCoordinates *routeCoordinates
	initializeHost   sync.Once

	bearerToken     string
	bearerTokenFile string
	timeout         *time.Duration
	tlsConfig       *tls.Config

	eventRecorder events.EventRecorder

	expect       string
	expectRegexp *regexp.Regexp
}

type routeCoordinates struct {
	namespace string
	name      string
}

func NewAPIServerBackend(disruptionBackendName, path string, connectionType BackendConnectionType) *BackendSampler {
	return &BackendSampler{
		connectionType:        connectionType,
		locator:               LocateDisruptionCheck(disruptionBackendName, connectionType),
		disruptionBackendName: disruptionBackendName,
		path:                  path,
	}
}

func NewRouteBackend(namespace, name, disruptionBackendName, path string, connectionType BackendConnectionType) *BackendSampler {
	return &BackendSampler{
		connectionType:        connectionType,
		locator:               LocateRouteForDisruptionCheck(namespace, name, disruptionBackendName, connectionType),
		disruptionBackendName: disruptionBackendName,
		path:                  path,
		routeCoordinates: &routeCoordinates{
			namespace: namespace,
			name:      name,
		},
	}
}

func (b *BackendSampler) WithHost(host string) *BackendSampler {
	b.host = host
	return b
}

func (b *BackendSampler) WithBearerTokenAuth(token, tokenFile string) *BackendSampler {
	b.bearerToken = token
	b.bearerTokenFile = tokenFile
	return b
}

func (b *BackendSampler) WithTLSConfig(tlsConfig *tls.Config) *BackendSampler {
	b.tlsConfig = tlsConfig
	return b
}

func (b *BackendSampler) WithExpectedBody(expectedBody string) *BackendSampler {
	b.expect = expectedBody
	return b
}

func (b *BackendSampler) WithExpectedBodyRegex(expectedBodyRegex string) *BackendSampler {
	b.expectRegexp = regexp.MustCompile(expectedBodyRegex)
	return b
}

func (b *BackendSampler) SetEventRecorder(eventRecorder events.EventRecorder) {
	b.eventRecorder = eventRecorder
}

func (b *BackendSampler) BodyMatches(body []byte) error {
	switch {
	case len(b.expect) != 0 && !bytes.Contains(body, []byte(b.expect)):
		return fmt.Errorf("response did not contain the correct body contents: %q", string(body))
	case b.expectRegexp != nil && !b.expectRegexp.MatchString(string(body)):
		return fmt.Errorf("response did not contain the correct body contents: %q", string(body))
	}

	return nil
}

func (b *BackendSampler) GetDisruptionBackendName() string {
	return b.disruptionBackendName
}

func (b *BackendSampler) GetLocator() string {
	return b.locator
}

func (b *BackendSampler) GetConnectionType() BackendConnectionType {
	return b.connectionType
}

func (b *BackendSampler) getTimeout() time.Duration {
	if b.timeout == nil {
		return 1 * time.Second
	}
	return *b.timeout
}

func (b *BackendSampler) GetURL() (string, error) {
	var hostErr error
	b.initializeHost.Do(func() {
		if len(b.host) > 0 {
			return
		}
		if b.routeCoordinates == nil {
			hostErr = fmt.Errorf("no route coordinates to lookup host")
			return
		}
		config, err := framework.LoadConfig()
		if err != nil {
			hostErr = err
			return
		}
		client, err := routeclientset.NewForConfig(config)
		if err != nil {
			hostErr = err
			return
		}
		route, err := client.RouteV1().Routes(b.routeCoordinates.namespace).Get(context.Background(), b.routeCoordinates.name, metav1.GetOptions{})
		if err != nil {
			hostErr = err
			return
		}
		for _, ingress := range route.Status.Ingress {
			if len(ingress.Host) > 0 {
				b.host = fmt.Sprintf("https://%s", ingress.Host)
				break
			}
		}
	})
	if hostErr != nil {
		return "", hostErr
	}
	if len(b.host) == 0 {
		return "", fmt.Errorf("missing URL")
	}
	return b.host + b.path, nil
}

func (b *BackendSampler) getTLSConfig() *tls.Config {
	if b.tlsConfig == nil {
		return &tls.Config{InsecureSkipVerify: true}
	}
	return b.tlsConfig
}

func (b *BackendSampler) wrapWithAuth(rt http.RoundTripper) (http.RoundTripper, error) {
	if len(b.bearerToken) == 0 && len(b.bearerTokenFile) == 0 {
		return rt, nil
	}

	if b.tlsConfig == nil {
		return nil, fmt.Errorf("WithTLSConfig is required if you are providing a token")
	}

	return transport.NewBearerAuthWithRefreshRoundTripper(b.bearerToken, b.bearerTokenFile, rt)
}

func (b *BackendSampler) GetHTTPClient() (*http.Client, error) {
	var httpTransport *http.Transport
	switch b.GetConnectionType() {
	case NewConnectionType:
		httpTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   b.getTimeout(),
				KeepAlive: -1, // this looks unnecessary to me, but it was set in other code.
			}).Dial,
			TLSClientConfig:     b.getTLSConfig(),
			TLSHandshakeTimeout: b.getTimeout(),
			DisableKeepAlives:   true, // this prevents connections from being reused
			IdleConnTimeout:     b.getTimeout(),
		}

	case ReusedConnectionType:
		httpTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout: b.getTimeout(),
			}).Dial,
			TLSClientConfig:     b.getTLSConfig(),
			TLSHandshakeTimeout: b.getTimeout(),
			IdleConnTimeout:     b.getTimeout(),
		}

	default:
		return nil, fmt.Errorf("unrecognized connection type")
	}

	roundTripper, err := b.wrapWithAuth(http.RoundTripper(httpTransport))
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: roundTripper,
	}

	return httpClient, nil
}

// StartEndpointMonitoring sets up a client for the given BackendSampler and starts a
// new sampler for the given monitor that uses the client to monitor
// connectivity to the BackendSampler and reports any observed disruption.
//
// If disableConnectionReuse is false, the client reuses connections and detects
// abrupt breaks in connectivity.  If disableConnectionReuse is true, the client
// instead creates fresh connections so that it detects failures to establish
// connections.
func (b *BackendSampler) StartEndpointMonitoring(ctx context.Context, m *Monitor) error {
	httpClient, err := b.GetHTTPClient()
	if err != nil {
		return err
	}

	url, err := b.GetURL()
	if err != nil {
		return err
	}

	go NewSampler(m, time.Second, func(previous bool) (condition *monitorapi.Condition, next bool) {
		resp, getErr := httpClient.Get(url)
		var body []byte
		var bodyReadErr, sampleErr error
		if getErr == nil {
			body, bodyReadErr = ioutil.ReadAll(resp.Body)
		}

		// we don't have an error, but the response code was an error, then we have to set an artificial error for the logic below to work.
		switch {
		case getErr != nil:
			sampleErr = getErr
		case bodyReadErr != nil:
			sampleErr = getErr
		case resp.StatusCode < 200 || resp.StatusCode > 399:
			sampleErr = fmt.Errorf("error running request: %v: %v", resp.Status, string(body))
		default:
			if bodyMatchErr := b.BodyMatches(body); bodyMatchErr != nil {
				sampleErr = bodyMatchErr
			}
		}
		currentlyAvailable := sampleErr == nil

		switch {
		case sampleErr == nil && previous:
			// we are continuing to function.  no condition change.
			return nil, currentlyAvailable

		case sampleErr != nil && !previous:
			// we are continuing to fail, no condition change.
			return nil, currentlyAvailable

		case sampleErr == nil && !previous:
			message := DisruptionEndedMessage(b.GetLocator(), b.GetConnectionType())
			framework.Logf(message)
			if b.eventRecorder != nil {
				b.eventRecorder.Eventf(
					&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.disruptionBackendName}, nil,
					v1.EventTypeNormal, "DisruptionEnded", "detected", message)
			}
			return &monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: b.GetLocator(),
				Message: message,
			}, currentlyAvailable

		case sampleErr != nil && previous:
			message := DisruptionBeganMessage(b.GetLocator(), b.GetConnectionType(), sampleErr)
			framework.Logf(message)
			if b.eventRecorder != nil {
				b.eventRecorder.Eventf(
					&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.disruptionBackendName}, nil,
					v1.EventTypeWarning, "DisruptionBegan", "detected", message)
			}
			return &monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: b.GetLocator(),
				Message: message,
			}, currentlyAvailable

		default:
			message := "math broke resulting in this weird error you need to find"
			framework.Logf(message)
			if b.eventRecorder != nil {
				b.eventRecorder.Eventf(
					&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.disruptionBackendName}, nil,
					v1.EventTypeWarning, "DisruptionConfused", "detected", message)
			}
			return &monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: b.GetLocator(),
				Message: message,
			}, currentlyAvailable
		}

	}).WhenFailing(ctx, &monitorapi.Condition{
		Level:   monitorapi.Error,
		Locator: b.GetLocator(),
		Message: DisruptionContinuingMessage(b.GetLocator(), b.GetConnectionType(), fmt.Errorf("missing associated failure")),
	})

	return nil
}

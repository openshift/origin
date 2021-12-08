package monitor

import (
	"bytes"
	"container/list"
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
	path                  string

	routeCoordinates *routeCoordinates
	initializeHost   sync.Once
	host             string
	hostErr          error

	bearerToken     string
	bearerTokenFile string
	timeout         *time.Duration
	tlsConfig       *tls.Config

	expect       string
	expectRegexp *regexp.Regexp

	initHTTPClient sync.Once
	httpClient     *http.Client
	httpClientErr  error
}

type routeCoordinates struct {
	namespace string
	name      string
}

func NewBackend(disruptionBackendName, path string, connectionType BackendConnectionType) *BackendSampler {
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

func (b *BackendSampler) BodyMatches(body []byte) error {
	switch {
	case len(b.expect) != 0 && !bytes.Contains(body, []byte(b.expect)):
		return fmt.Errorf("response did not contain the correct body contents: %q", string(body))
	case b.expectRegexp != nil && !b.expectRegexp.MatchString(string(body)):
		return fmt.Errorf("response did not contain the correct body contents: %q", string(body))
	}

	return nil
}

func (b *BackendSampler) SetHost(host string) {
	b.host = host
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
		return 5 * time.Second
	}
	return *b.timeout
}

func (b *BackendSampler) GetURL() (string, error) {
	b.initializeHost.Do(func() {
		if len(b.host) > 0 {
			return
		}
		if b.routeCoordinates == nil {
			b.hostErr = fmt.Errorf("no route coordinates to lookup host")
			return
		}
		config, err := framework.LoadConfig()
		if err != nil {
			b.hostErr = err
			return
		}
		client, err := routeclientset.NewForConfig(config)
		if err != nil {
			b.hostErr = err
			return
		}
		route, err := client.RouteV1().Routes(b.routeCoordinates.namespace).Get(context.Background(), b.routeCoordinates.name, metav1.GetOptions{})
		if err != nil {
			b.hostErr = err
			return
		}
		for _, ingress := range route.Status.Ingress {
			if len(ingress.Host) > 0 {
				b.host = fmt.Sprintf("https://%s", ingress.Host)
				break
			}
		}
	})
	if b.hostErr != nil {
		return "", b.hostErr
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
	b.initHTTPClient.Do(func() {
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
			b.httpClient = nil
			b.httpClientErr = fmt.Errorf("unrecognized connection type")
			return
		}

		roundTripper, err := b.wrapWithAuth(http.RoundTripper(httpTransport))
		if err != nil {
			b.httpClient = nil
			b.httpClientErr = err
			return
		}

		b.httpClient = &http.Client{
			Transport: roundTripper,
		}
		b.httpClientErr = nil
	})

	return b.httpClient, b.httpClientErr
}

func (b *BackendSampler) CheckConnection(ctx context.Context) error {
	httpClient, err := b.GetHTTPClient()
	if err != nil {
		return err
	}

	url, err := b.GetURL()
	if err != nil {
		return err
	}

	resp, getErr := httpClient.Get(url)
	var body []byte
	var bodyReadErr, sampleErr error
	if getErr == nil {
		body, bodyReadErr = ioutil.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			framework.Logf("error closing body: %v: %v", b.GetLocator(), closeErr)
		}
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

	return sampleErr
}

// StartEndpointMonitoring sets up a client for the given BackendSampler, starts checking the endpoint, and recording
// success/failure edges into the monitorRecorder
func (b *BackendSampler) StartEndpointMonitoring(ctx context.Context, monitorRecorder Recorder, eventRecorder events.EventRecorder) error {
	if monitorRecorder == nil {
		return fmt.Errorf("monitor is required")
	}
	if eventRecorder == nil {
		fakeEventRecorder := events.NewFakeRecorder(100)
		// discard the events
		go func() {
			for {
				select {
				case <-fakeEventRecorder.Events:
				case <-ctx.Done():
					return
				}
			}
		}()
		eventRecorder = fakeEventRecorder
	}

	interval := 1 * time.Second
	disruptionSampler := newDisruptionSampler(b)
	go disruptionSampler.produceSamples(ctx, interval)
	go disruptionSampler.consumeSamples(ctx, interval, monitorRecorder, eventRecorder)

	return nil
}

type disruptionSampler struct {
	backendSampler *BackendSampler

	lock           sync.Mutex
	activeSamplers list.List
}

func newDisruptionSampler(backendSampler *BackendSampler) *disruptionSampler {
	return &disruptionSampler{
		backendSampler: backendSampler,
		lock:           sync.Mutex{},
		activeSamplers: list.List{},
	}
}

// produceSamples only exits when the ctx is closed
func (b *disruptionSampler) produceSamples(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		// the sampleFn may take a significant period of time to run.  In such a case, we want our start interval
		// for when a failure started to be the time when the request was first made, not the time when the call
		// returned.  Imagine a timeout set on a DNS lookup of 30s: when the GET finally fails and returns, the outage
		// was actually 30s before.
		currDisruptionSample := b.newSample(ctx)
		go func() {
			sampleErr := b.backendSampler.CheckConnection(ctx)
			currDisruptionSample.setSampleError(sampleErr)
			close(currDisruptionSample.finished)
		}()

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// consumeSamples only exits when the ctx is closed
func (b *disruptionSampler) consumeSamples(ctx context.Context, interval time.Duration, monitorRecorder Recorder, eventRecorder events.EventRecorder) {
	firstSample := true
	var previousError error
	previousIntervalID := -1
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		currSample := b.getOldestSample(ctx)
		if currSample == nil {
			select {
			case <-time.After(interval):
				continue
			case <-ctx.Done():
				return
			}
		}

		//wait for the current sample to finish
		select {
		case <-currSample.finished:
		case <-ctx.Done():
			return
		}

		previouslyAvailable := previousError == nil
		currentError := currSample.getSampleError()
		currentlyAvailable := currentError == nil

		switch {
		case currentlyAvailable && previouslyAvailable:
			// we are continuing to function.  no condition change.

		case !currentlyAvailable && !previouslyAvailable:
			// we are continuing to fail, check to see if the error is new
			if previousError.Error() == currentError.Error() && !firstSample {
				// if the error is the same and this isn't the first sample we have, skip
				break
			}

			// if the error is new or the first we have seen.
			// end the previous interval if we have one, because we need to start a new interval
			if previousIntervalID != -1 {
				monitorRecorder.EndInterval(previousIntervalID, currSample.startTime)
			}

			// start a new interval with the new error
			message := DisruptionBeganMessage(b.backendSampler.GetLocator(), b.backendSampler.GetConnectionType(), currentError)
			framework.Logf(message)
			eventRecorder.Eventf(
				&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.backendSampler.GetDisruptionBackendName()}, nil,
				v1.EventTypeWarning, "DisruptionBegan", "detected", message)
			currCondition := monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: b.backendSampler.GetLocator(),
				Message: message,
			}
			previousIntervalID = monitorRecorder.StartInterval(currSample.startTime, currCondition)

		case currentlyAvailable && !previouslyAvailable:
			// end the previous interval if we have one because our state changed
			if previousIntervalID != -1 {
				monitorRecorder.EndInterval(previousIntervalID, currSample.startTime)
			}

			message := DisruptionEndedMessage(b.backendSampler.GetLocator(), b.backendSampler.GetConnectionType())
			framework.Logf(message)
			eventRecorder.Eventf(
				&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.backendSampler.GetDisruptionBackendName()}, nil,
				v1.EventTypeNormal, "DisruptionEnded", "detected", message)
			currCondition := monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: b.backendSampler.GetLocator(),
				Message: message,
			}
			previousIntervalID = monitorRecorder.StartInterval(currSample.startTime, currCondition)

		case !currentlyAvailable && previouslyAvailable:
			// end the previous interval if we have one because our state changed
			if previousIntervalID != -1 {
				monitorRecorder.EndInterval(previousIntervalID, currSample.startTime)
			}

			message := DisruptionBeganMessage(b.backendSampler.GetLocator(), b.backendSampler.GetConnectionType(), currentError)
			framework.Logf(message)
			eventRecorder.Eventf(
				&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.backendSampler.GetDisruptionBackendName()}, nil,
				v1.EventTypeWarning, "DisruptionBegan", "detected", message)
			currCondition := monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: b.backendSampler.GetLocator(),
				Message: message,
			}
			previousIntervalID = monitorRecorder.StartInterval(currSample.startTime, currCondition)

		default:
			panic("math broke resulting in this weird error you need to find")
		}

		firstSample = false
		previousError = currentError
	}
}

func (b *disruptionSampler) getOldestSample(ctx context.Context) *disruptionSample {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.activeSamplers.Len() == 0 {
		return nil
	}
	uncast := b.activeSamplers.Front()
	return uncast.Value.(*disruptionSample)
}

func (b *disruptionSampler) newSample(ctx context.Context) *disruptionSample {
	b.lock.Lock()
	defer b.lock.Unlock()
	currentDisruptionSample := newDisruptionSample(time.Now())
	b.activeSamplers.PushBack(currentDisruptionSample)
	return currentDisruptionSample
}

type disruptionSample struct {
	lock      sync.Mutex
	startTime time.Time
	sampleErr error

	finished chan struct{}
}

func newDisruptionSample(startTime time.Time) *disruptionSample {
	return &disruptionSample{
		startTime: startTime,
		finished:  make(chan struct{}),
	}
}
func (s *disruptionSample) setSampleError(sampleErr error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.sampleErr = sampleErr
}
func (s *disruptionSample) getSampleError() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.sampleErr
}

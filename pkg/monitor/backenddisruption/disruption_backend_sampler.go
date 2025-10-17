package backenddisruption

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

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/transport"
	"k8s.io/kubernetes/test/e2e/framework"
)

// this entire file should be a separate package with disruption_***, but we are entanged because the sampler lives in monitor
// and the things being started by the monitor are coupled into .Start.
// we also got stuck on writing the disruption backends.  We need a way to track which disruption checks we have started,
// so we can properly write out "zero"

// BackendSampler is used to monitor an HTTP endpoint and ensure that it is always accessible.
// It records results into the monitorRecorder that is passed to the StartEndpointMonitoring call.
type BackendSampler struct {
	// locator is the string used to identify this in the monitorRecorder later on.  It should always be set
	// by the constructors to ensure a consistent shape for later inspection in higher layers.
	locator monitorapi.Locator
	// connectionType indicates what type of connection is being used.
	connectionType monitorapi.BackendConnectionType
	// is the `/path` part of the url.  It must start with a slash.
	path string

	hostGetter HostGetter

	// bearerToken is the token to be used when contacting a server. Authorization : Bearer XXXXXX
	bearerToken string
	// bearerTokenFile is the file containing a token to be used when contacting a server. Authorization : Bearer XXXXXX
	bearerTokenFile string
	// timeout is the single timeout used for lots of individual phases of the  http request and the overall.
	timeout *time.Duration
	// tlsConfig holds the CA bundle for verifying the server and client cert/key pair for identifying to the server.
	tlsConfig *tls.Config

	// expectedStatusCode allows status codes other than 200-399.
	expectedStatusCode int
	// expect is an exact text match for the expected body.  If expect and expectRegexp are empty, then any 2xx or 3xx
	// http status code is accepted.
	expect string
	// expectRegexp is a regex for matching the expected body.  If expect and expectRegexp are empty, then any 2xx or 3xx
	// http status code is accepted.
	expectRegexp *regexp.Regexp

	// userAgent used to sets the User-Agent HTTP Header for all requests that are sent by this sampler
	userAgent string

	// initHTTPClient ensures we only create the http client once
	initHTTPClient sync.Once
	// httpClient is used to connect to the host+path
	httpClient *http.Client
	// httpClientErr is the error (if we got one) from initHTTPClient.  This easier than retrying if we fail and probably
	//	good enough for CI
	httpClientErr error

	// runningLock
	runningLock sync.Mutex
	// stopRunning is a context cancel for the localContext used to run
	stopRunning context.CancelFunc
	// consumptionFinished is closed when the consumer is done
	consumptionFinished chan struct{}

	samplerHooks []SamplerHook
}

type routeCoordinates struct {
	// namespace containing the route
	namespace string
	// name of the route
	name string
}

const OpenshiftTestsSource = "openshift-tests"

// NewSimpleBackendFromOpenshiftTests constructs a BackendSampler suitable for use against a generic server
func NewSimpleBackendFromOpenshiftTests(host, disruptionBackendName, path string, connectionType monitorapi.BackendConnectionType) *BackendSampler {

	ret := &BackendSampler{
		connectionType:      connectionType,
		locator:             monitorapi.NewLocator().LocateDisruptionCheck(disruptionBackendName, OpenshiftTestsSource, connectionType),
		path:                path,
		hostGetter:          NewSimpleHostGetter(host),
		consumptionFinished: make(chan struct{}),
	}

	// TODO return error?  This is programmer error
	if len(ret.GetDisruptionBackendName()) == 0 {
		panic("missing disruption backend")
	}

	return ret
}

// NewRouteBackend constructs a BackendSampler suitable for use against a routes.route.openshift.io
func NewSimpleBackendWithLocator(locator monitorapi.Locator, host, path string, connectionType monitorapi.BackendConnectionType) *BackendSampler {
	ret := &BackendSampler{
		connectionType:      connectionType,
		locator:             locator,
		path:                path,
		hostGetter:          NewSimpleHostGetter(host),
		consumptionFinished: make(chan struct{}),
	}

	// TODO return error?  This is programmer error
	if len(ret.GetDisruptionBackendName()) == 0 {
		panic("missing disruption backend")
	}

	return ret
}

// NewAPIServerBackend constructs a BackendSampler suitable for use against a kube-like API server
func NewAPIServerBackend(clientConfig *rest.Config, disruptionBackendName, path string, connectionType monitorapi.BackendConnectionType) (*BackendSampler, error) {
	historicalBackendDisruptionDataName := fmt.Sprintf("%s-%v-connections", disruptionBackendName, connectionType)

	kubeTransportConfig, err := clientConfig.TransportConfig()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(kubeTransportConfig)
	if err != nil {
		return nil, err
	}

	// Kube-apiserver allows 34s timeout for api requests. We want to match it here.
	timeout := 34 * time.Second
	ret := &BackendSampler{
		connectionType:      connectionType,
		locator:             monitorapi.NewLocator().LocateDisruptionCheck(historicalBackendDisruptionDataName, OpenshiftTestsSource, connectionType),
		path:                path,
		hostGetter:          NewKubeAPIHostGetter(clientConfig),
		tlsConfig:           tlsConfig,
		bearerToken:         kubeTransportConfig.BearerToken,
		bearerTokenFile:     kubeTransportConfig.BearerTokenFile,
		consumptionFinished: make(chan struct{}),
		timeout:             &timeout,
	}

	// TODO return error?  This is programmer error
	if len(ret.GetDisruptionBackendName()) == 0 {
		panic("missing disruption backend")
	}

	return ret, nil
}

// NewRouteBackend constructs a BackendSampler suitable for use against a routes.route.openshift.io
func NewRouteBackend(clientConfig *rest.Config, namespace, name, disruptionBackendName, path string, connectionType monitorapi.BackendConnectionType) *BackendSampler {
	historicalBackendDisruptionDataName := fmt.Sprintf("%s-%v-connections", disruptionBackendName, connectionType)

	ret := &BackendSampler{
		connectionType:      connectionType,
		locator:             monitorapi.NewLocator().LocateRouteForDisruptionCheck(historicalBackendDisruptionDataName, OpenshiftTestsSource, namespace, name, connectionType),
		path:                path,
		hostGetter:          NewRouteHostGetter(clientConfig, namespace, name),
		consumptionFinished: make(chan struct{}),
	}

	// TODO return error?  This is programmer error
	if len(ret.GetDisruptionBackendName()) == 0 {
		panic("missing disruption backend")
	}

	return ret
}

// WithBearerTokenAuth sets bearer tokens to use
func (b *BackendSampler) WithBearerTokenAuth(token, tokenFile string) *BackendSampler {
	b.bearerToken = token
	b.bearerTokenFile = tokenFile
	return b
}

// WithTLSConfig sets both the CA bundle for trusting the server and the client cert/key pair for identifying to the server
func (b *BackendSampler) WithTLSConfig(tlsConfig *tls.Config) *BackendSampler {
	b.tlsConfig = tlsConfig
	return b
}

// WithExpectedStatusCode sets the expected http status
func (b *BackendSampler) WithExpectedStatusCode(statusCode int) *BackendSampler {
	b.expectedStatusCode = statusCode
	return b
}

// WithExpectedBody allows a specification of specific body to be returned. This useful when passing through proxies and the
// like since a connection may not be the one you expect.  If not specified, then the default behavior is that any 2xx
// or 3xx response is acceptable.
func (b *BackendSampler) WithExpectedBody(expectedBody string) *BackendSampler {
	b.expect = expectedBody
	return b
}

// WithUserAgent sets the User-Agent HTTP Header for all requests that are sent by this sampler
func (b *BackendSampler) WithUserAgent(userAgent string) *BackendSampler {
	b.userAgent = userAgent
	return b
}

// WithExpectedBodyRegex allows a specification of specific body to be returned. This useful when passing through proxies and the
// like since a connection may not be the one you expect.  If not specified, then the default behavior is that any 2xx
// or 3xx response is acceptable.
func (b *BackendSampler) WithExpectedBodyRegex(expectedBodyRegex string) *BackendSampler {
	b.expectRegexp = regexp.MustCompile(expectedBodyRegex)
	return b
}

// WithSamplerHooks adds a list of hooks for the sampler to call at different stages of disruption detection
func (b *BackendSampler) WithSamplerHooks(samplerHooks []SamplerHook) *BackendSampler {
	b.samplerHooks = samplerHooks
	return b
}

// bodyMatches checks the body content and returns an error if it doesn't match the expected.
func (b *BackendSampler) bodyMatches(body []byte) error {
	switch {
	case len(b.expect) != 0 && !bytes.Contains(body, []byte(b.expect)):
		return fmt.Errorf("response did not contain the correct body contents: %q", string(body))
	case b.expectRegexp != nil && !b.expectRegexp.MatchString(string(body)):
		return fmt.Errorf("response did not contain the correct body contents: %q", string(body))
	}

	return nil
}

func (b *BackendSampler) GetDisruptionBackendName() string {
	return monitorapi.BackendDisruptionNameFromLocator(b.locator)
}

func (b *BackendSampler) GetLocator() monitorapi.Locator {
	return b.locator
}

func (b *BackendSampler) GetConnectionType() monitorapi.BackendConnectionType {
	return b.connectionType
}

func (b *BackendSampler) getTimeout() time.Duration {
	if b.timeout == nil {
		return 20 * time.Second
	}
	return *b.timeout
}

func (b *BackendSampler) GetURL() (string, error) {
	host, err := b.hostGetter.GetHost()
	if err != nil {
		return "", err
	}
	if len(host) == 0 {
		return "", fmt.Errorf("missing URL")
	}
	return host + b.path, nil
}

func (b *BackendSampler) getTLSConfig() *tls.Config {
	if b.tlsConfig == nil {
		return &tls.Config{InsecureSkipVerify: true}
	}
	return b.tlsConfig
}

// wrapWithAuth adds a roundtripper for bearertoken auth.  You must have a tlsConfig if you're passing a bearer token
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
	timeoutForEntireRequest := b.getTimeout()
	timeoutForPartOfRequest := timeoutForEntireRequest * 4 / 5 // this is less so that we can see failures for individual portions of the request

	b.initHTTPClient.Do(func() {
		var httpTransport *http.Transport
		switch b.GetConnectionType() {
		case monitorapi.NewConnectionType:
			httpTransport = &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   timeoutForPartOfRequest,
					KeepAlive: -1, // this looks unnecessary to me, but it was set in other code.
				}).Dial,
				TLSClientConfig:       b.getTLSConfig(),
				DisableKeepAlives:     true, // this prevents connections from being reused
				TLSHandshakeTimeout:   timeoutForPartOfRequest,
				IdleConnTimeout:       timeoutForPartOfRequest,
				ResponseHeaderTimeout: timeoutForPartOfRequest,
				ExpectContinueTimeout: timeoutForPartOfRequest,
				Proxy:                 http.ProxyFromEnvironment,
			}

		case monitorapi.ReusedConnectionType:
			httpTransport = &http.Transport{
				Dial: (&net.Dialer{
					Timeout: timeoutForPartOfRequest,
				}).Dial,
				TLSClientConfig:       b.getTLSConfig(),
				TLSHandshakeTimeout:   timeoutForPartOfRequest,
				IdleConnTimeout:       timeoutForPartOfRequest,
				ResponseHeaderTimeout: timeoutForPartOfRequest,
				ExpectContinueTimeout: timeoutForPartOfRequest,
				Proxy:                 http.ProxyFromEnvironment,
			}

		default:
			b.httpClient = nil
			b.httpClientErr = fmt.Errorf("unrecognized connection type")
			return
		}

		var err error
		rt := http.RoundTripper(httpTransport)
		rt, err = b.wrapWithAuth(rt)
		if err != nil {
			b.httpClient = nil
			b.httpClientErr = err
			return
		}
		if len(b.userAgent) > 0 {
			rt = transport.NewUserAgentRoundTripper(b.userAgent, rt)
		}

		b.httpClient = &http.Client{
			Transport: rt,
			Timeout:   timeoutForEntireRequest,
		}
		b.httpClientErr = nil
	})

	return b.httpClient, b.httpClientErr
}

// CheckConnnection returns the audit request UID and an error if there was one.
func (b *BackendSampler) CheckConnection(ctx context.Context) (string, error) {
	httpClient, err := b.GetHTTPClient()
	if err != nil {
		return "", err
	}

	url, err := b.GetURL()
	if err != nil {
		return "", err
	}

	// this is longer than the http client timeout to avoid tripping, but is here to be sure we finish eventually
	backstopContextTimeout := b.getTimeout() * 3 / 2 // (1.5)
	requestContext, requestCancel := context.WithTimeout(ctx, backstopContextTimeout)
	defer requestCancel()
	req, err := http.NewRequestWithContext(requestContext, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	uid := uuid.New().String()
	req.Header.Set(audit.HeaderAuditID, uid)

	resp, getErr := httpClient.Do(req)
	if requestContext.Err() == context.Canceled {
		// this isn't an error, we were simply cancelled
		return uid, nil
	}

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
		sampleErr = bodyReadErr
	case b.expectedStatusCode > 0 && b.expectedStatusCode == resp.StatusCode:
		// don't fail
	case resp.StatusCode < 200 || resp.StatusCode > 399:
		sampleErr = fmt.Errorf("error running request: %v: %v", resp.Status, string(body))
	default:
		if bodyMatchErr := b.bodyMatches(body); bodyMatchErr != nil {
			sampleErr = bodyMatchErr
		}
	}

	return uid, sampleErr
}

// RunEndpointMonitoring sets up a client for the given BackendSampler, starts checking the endpoint, and recording
// success/failure edges into the monitorRecorder, and blocks until the context is closed or the sampler is closed.
func (b *BackendSampler) RunEndpointMonitoring(ctx context.Context, monitorRecorder monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error {
	if b.isRunning() {
		return fmt.Errorf("cannot monitor twice at the same time")
	}

	// the producer is wired from the original context so that a base cancel stops everything
	samplerContext, samplerCancel := context.WithCancel(ctx)
	defer samplerCancel()
	b.setCancelForRun(samplerCancel) // used from .Stop later to stop monitoring

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
				case <-samplerContext.Done():
					return
				}
			}
		}()
		eventRecorder = fakeEventRecorder
	}

	interval := 1 * time.Second
	disruptionSampler := newDisruptionSampler(b)
	go disruptionSampler.produceSamples(samplerContext, interval)
	go disruptionSampler.consumeSamples(samplerContext, b.consumptionFinished, interval, monitorRecorder, eventRecorder)

	<-samplerContext.Done()
	<-b.consumptionFinished

	if disruptionSampler.numberOfSamples(ctx) > 0 {
		return fmt.Errorf("not finished writing all samples (%d remaining), but we're told to close", disruptionSampler.numberOfSamples(ctx))
	}

	return nil
}

func (b *BackendSampler) isRunning() bool {
	b.runningLock.Lock()
	defer b.runningLock.Unlock()
	return b.stopRunning != nil
}

func (b *BackendSampler) setCancelForRun(cancelFunc context.CancelFunc) {
	b.runningLock.Lock()
	defer b.runningLock.Unlock()
	b.stopRunning = cancelFunc
}

// Stop stops the produce and consumer and blocks until the consumer is finished consuming.
func (b *BackendSampler) Stop() {
	b.runningLock.Lock()
	defer b.runningLock.Unlock()
	if b.stopRunning == nil {
		return
	}
	if b.stopRunning != nil {
		b.stopRunning()
	}
	b.stopRunning = nil

	for {
		fmt.Printf("waiting for consumer to finish %v...\n", b.locator)
		select {
		case <-b.consumptionFinished:
			fmt.Printf("consumer finished %v\n", b.locator)
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// StartEndpointMonitoring sets up a client for the given BackendSampler, starts checking the endpoint, and recording
// success/failure edges into the monitorRecorder
func (b *BackendSampler) StartEndpointMonitoring(ctx context.Context, monitorRecorder monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error {
	if monitorRecorder == nil {
		return fmt.Errorf("monitor is required")
	}

	go func() {
		err := b.RunEndpointMonitoring(ctx, monitorRecorder, eventRecorder)
		if err != nil {
			utilruntime.HandleError(err)
		}
	}()

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
			uid, sampleErr := b.backendSampler.CheckConnection(ctx)
			currDisruptionSample.setSampleError(sampleErr)
			currDisruptionSample.setRequestAuditID(uid)
			if sampleErr != nil {
				// We'd like to include these UUIDs in the backend-disruption.json file but this is
				// not possible without some work as we're basing everything off intervals today. There is
				// no place to store request UUIDs without stuffing them into the  interval message, which would break
				// the code that determines when disruption started/stopped based on the similarity of the message.
				// For now we will just log clearly the requests that failed and use this to correlate with the
				// audit log manually.
				logrus.WithFields(logrus.Fields{
					"this-instance": b.backendSampler.locator,
					"backend":       b.backendSampler.GetDisruptionBackendName(),
					"type":          b.backendSampler.connectionType,
					"auditID":       uid,
				}).Errorf("disruption sample failed: %v", sampleErr)
			}
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
func (b *disruptionSampler) consumeSamples(ctx context.Context, consumerDoneCh chan struct{}, interval time.Duration, monitorRecorder monitorapi.RecorderWriter, eventRecorder events.EventRecorder) {
	defer close(consumerDoneCh)

	firstSample := true
	previousError := fmt.Errorf("never checked before")
	previousIntervalID := -1
	var previousSampleTime *time.Time

	// when we exit this function, we want to set a final duration of failure.  We don't actually know whether it ended
	// or how long it took to ask
	defer func() {
		if previousIntervalID != -1 && previousSampleTime != nil {
			monitorRecorder.EndInterval(previousIntervalID, previousSampleTime.Add(interval))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		currSample := b.popOldestSample(ctx)
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
		currSampleTime := currSample.startTime

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

			for _, hook := range b.backendSampler.samplerHooks {
				hook.DisruptionStarted(ctx)
			}
			// start a new interval with the new error
			message, eventReason, level := DisruptionBegan(b.backendSampler.GetLocator().OldLocator(), b.backendSampler.GetConnectionType(), currentError, currSample.getRequestAuditID())
			framework.Logf("%s", message.BuildString())
			eventRecorder.Eventf(
				&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.backendSampler.GetDisruptionBackendName()}, nil,
				v1.EventTypeWarning, string(eventReason), "detected", message.BuildString())
			currInterval := monitorapi.NewInterval(monitorapi.SourceDisruption, level).
				Locator(b.backendSampler.GetLocator()).
				Display().
				Message(message).Build(currSample.startTime, time.Time{})
			previousIntervalID = monitorRecorder.StartInterval(currInterval)

		case currentlyAvailable && !previouslyAvailable:
			// end the previous interval if we have one because our state changed
			if previousIntervalID != -1 {
				monitorRecorder.EndInterval(previousIntervalID, currSample.startTime)
			}

			message := DisruptionEndedMessage(b.backendSampler.GetLocator().OldLocator(), b.backendSampler.GetConnectionType())
			eventRecorder.Eventf(
				&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.backendSampler.GetDisruptionBackendName()}, nil,
				v1.EventTypeNormal, string(monitorapi.DisruptionEndedEventReason), "detected", message.BuildString())
			currInterval := monitorapi.NewInterval(monitorapi.SourceDisruption, monitorapi.Info).
				Locator(b.backendSampler.GetLocator()).
				Message(message).Build(currSample.startTime, time.Time{})
			previousIntervalID = monitorRecorder.StartInterval(currInterval)

		case !currentlyAvailable && previouslyAvailable:
			// end the previous interval if we have one because our state changed
			if previousIntervalID != -1 {
				monitorRecorder.EndInterval(previousIntervalID, currSample.startTime)
			}

			for _, hook := range b.backendSampler.samplerHooks {
				hook.DisruptionStarted(ctx)
			}

			message, eventReason, level := DisruptionBegan(b.backendSampler.GetLocator().OldLocator(), b.backendSampler.GetConnectionType(), currentError, currSample.getRequestAuditID())
			framework.Logf("%s", message.BuildString())
			eventRecorder.Eventf(
				&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: b.backendSampler.GetDisruptionBackendName()}, nil,
				v1.EventTypeWarning, string(eventReason), "detected", message.BuildString())
			currInterval := monitorapi.NewInterval(monitorapi.SourceDisruption, level).
				Locator(b.backendSampler.GetLocator()).
				Message(message).Display().Build(currSample.startTime, time.Time{})
			previousIntervalID = monitorRecorder.StartInterval(currInterval)

		default:
			panic("math broke resulting in this weird error you need to find")
		}

		firstSample = false
		previousError = currentError
		t := currSampleTime // make sure we get a copy
		previousSampleTime = &t
	}
}

func (b *disruptionSampler) popOldestSample(ctx context.Context) *disruptionSample {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.activeSamplers.Len() == 0 {
		return nil
	}
	uncast := b.activeSamplers.Front()
	if uncast != nil {
		b.activeSamplers.Remove(uncast)
	}
	return uncast.Value.(*disruptionSample)
}

func (b *disruptionSampler) newSample(ctx context.Context) *disruptionSample {
	b.lock.Lock()
	defer b.lock.Unlock()
	currentDisruptionSample := newDisruptionSample(time.Now())
	b.activeSamplers.PushBack(currentDisruptionSample)
	return currentDisruptionSample
}

func (b *disruptionSampler) numberOfSamples(ctx context.Context) int {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.activeSamplers.Len()
}

type disruptionSample struct {
	lock           sync.Mutex
	startTime      time.Time
	sampleErr      error
	requestAuditID string

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

func (s *disruptionSample) setRequestAuditID(auditID string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.requestAuditID = auditID
}

func (s *disruptionSample) getRequestAuditID() string {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.requestAuditID
}

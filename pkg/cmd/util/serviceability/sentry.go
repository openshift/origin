package serviceability

import (
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/raven-go"

	"github.com/openshift/origin/pkg/version"
)

// SentryMonitor encapsulates a Sentry client and set of default tags
type SentryMonitor struct {
	client *raven.Client
	tags   map[string]string
}

// NewSentryMonitor creates a class that can capture panics and errors from OpenShift
// and Kubernetes that can roll up to a Sentry server.
func NewSentryMonitor(url string) (*SentryMonitor, error) {
	client, err := raven.NewClient(url, nil)
	if err != nil {
		return nil, err
	}
	client.SetRelease(version.Get().GitCommit)
	return &SentryMonitor{
		client: client,
	}, nil
}

func (m *SentryMonitor) capturePanic(capture interface{}) chan error {
	var packet *raven.Packet
	switch rval := capture.(type) {
	case error:
		packet = raven.NewPacket(rval.Error(), raven.NewException(rval, raven.NewStacktrace(2, 3, nil)))
	default:
		rvalStr := fmt.Sprint(rval)
		packet = raven.NewPacket(rvalStr, raven.NewException(errors.New(rvalStr), raven.NewStacktrace(2, 3, nil)))
	}
	_, ch := m.client.Capture(packet, m.tags)
	return ch
}

// CapturePanic is used by the Sentry client to capture panics
func (m *SentryMonitor) CapturePanic(capture interface{}) {
	m.capturePanic(capture)
}

// CapturePanicAndWait waits until either the Sentry client captures a panic or
// the provided time expires
func (m *SentryMonitor) CapturePanicAndWait(capture interface{}, until time.Duration) {
	select {
	case <-m.capturePanic(capture):
	case <-time.After(until):
	}
}

// CaptureError is used by the Sentry client to capture errors
func (m *SentryMonitor) CaptureError(err error) {
	m.client.CaptureError(err, m.tags)
}

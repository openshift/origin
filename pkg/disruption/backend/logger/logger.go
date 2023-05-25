package logger

import (
	"github.com/openshift/origin/pkg/disruption/backend"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/sirupsen/logrus"
)

// NewLogger returns a new instance of SampleCollector that logs samples
func NewLogger(delegate backendsampler.SampleCollector, descriptor backend.TestDescriptor) *logger {
	return &logger{
		delegate:   delegate,
		descriptor: descriptor,
		logAll:     false,
	}
}

type logger struct {
	delegate   backendsampler.SampleCollector
	descriptor backend.TestDescriptor
	logAll     bool
}

func (l *logger) Collect(s backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if l.delegate != nil {
		l.delegate.Collect(s)
	}
	l.log(s)
}

func (l *logger) log(s backend.SampleResult) {
	if s.Sample == nil {
		return
	}

	err := s.AggregateErr()
	rr := s.RequestResponse
	_, retry := rr.IsRetryAfter()
	// we will log it only when it has interesting data,
	// an error, a retry after, or shutdown window in progress
	if !(l.logAll || (err != nil || rr.ShutdownInProgress() || retry)) {
		return
	}

	entry := logrus.WithFields(logrus.Fields{
		"backend": l.descriptor.Name(),
		"sample":  s.Sample.ID,
	})
	entry = entry.WithFields(s.Fields())

	switch {
	case err != nil:
		entry.WithError(err)
	default:
		entry.Infof("interesting disruption sample")
	}
}

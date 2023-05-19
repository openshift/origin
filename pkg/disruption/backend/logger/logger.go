package logger

import (
	"github.com/openshift/origin/pkg/disruption/backend"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/sirupsen/logrus"
)

// NewLogger returns a new instance of SampleCollector that logs samples
func NewLogger(delegate backendsampler.SampleCollector, name string, connType monitorapi.BackendConnectionType) *logger {
	return &logger{
		delegate: delegate,
		name:     name,
		connType: connType,
		logAll:   false,
	}
}

type logger struct {
	delegate backendsampler.SampleCollector
	name     string
	connType monitorapi.BackendConnectionType
	logAll   bool
}

func (l logger) Collect(s backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if l.delegate != nil {
		l.delegate.Collect(s)
	}
	l.log(s)
}

func (l logger) log(s backend.SampleResult) {
	if s.Sample == nil {
		return
	}

	err := s.AggregateErr()
	rr := s.RequestResponse
	_, retry := rr.IsRetryAfter()
	// we will log it only when it has interesting data,
	// an error, a retry after, or shutdown window in progress
	if !(l.logAll || (err != nil || retry)) {
		return
	}

	entry := logrus.WithFields(logrus.Fields{
		"backend": l.name,
		"type":    l.connType,
		"sample":  s.Sample.ID,
	})
	entry = entry.WithFields(s.Fields())

	switch {
	case err != nil:
		entry.Errorf("failure: %v", err)
	default:
		entry.Infof("received a sample")
	}
}

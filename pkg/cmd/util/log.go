package util

import (
	"io"

	"github.com/golang/glog"

	"github.com/openshift/library-go/pkg/serviceability"
)

// NewGLogWriterV returns a new Writer that delegates to `glog.Info` at the
// desired level of verbosity
func NewGLogWriterV(level int) io.Writer {
	return &gLogWriter{
		level: glog.Level(level),
	}
}

// gLogWriter is a Writer that writes by delegating to `glog.Info`
type gLogWriter struct {
	// level is the default level to log at
	level glog.Level
}

func (w *gLogWriter) Write(p []byte) (n int, err error) {
	if glog.V(w.level) {
		glog.InfoDepth(2, string(p))
	}

	return len(p), nil
}

// InitLogrus sets the logrus trace level based on the glog trace level.
func InitLogrus() {
	switch {
	case bool(glog.V(4)):
		serviceability.InitLogrus("DEBUG")
	case bool(glog.V(2)):
		serviceability.InitLogrus("INFO")
	case bool(glog.V(0)):
		serviceability.InitLogrus("WARN")
	}
}

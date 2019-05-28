package util

import (
	"io"

	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/serviceability"
)

// NewGLogWriterV returns a new Writer that delegates to `klog.Info` at the
// desired level of verbosity
func NewGLogWriterV(level int) io.Writer {
	return &gLogWriter{
		level: klog.Level(level),
	}
}

// gLogWriter is a Writer that writes by delegating to `klog.Info`
type gLogWriter struct {
	// level is the default level to log at
	level klog.Level
}

func (w *gLogWriter) Write(p []byte) (n int, err error) {
	if klog.V(w.level) {
		klog.InfoDepth(2, string(p))
	}

	return len(p), nil
}

// InitLogrus sets the logrus trace level based on the glog trace level.
func InitLogrus() {
	switch {
	case bool(klog.V(4)):
		serviceability.InitLogrus("DEBUG")
	case bool(klog.V(2)):
		serviceability.InitLogrus("INFO")
	case bool(klog.V(0)):
		serviceability.InitLogrus("WARN")
	}
}

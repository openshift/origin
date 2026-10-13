package serviceability

import (
	"os"
	"strings"

	"k8s.io/klog/v2"

	"github.com/sirupsen/logrus"
)

// InitLogrusFromKlog sets the logrus trace level based on the klog trace level.
func InitLogrusFromKlog() {
	switch {
	case klog.V(4).Enabled():
		InitLogrus("DEBUG")
	case klog.V(2).Enabled():
		InitLogrus("INFO")
	case klog.V(0).Enabled():
		InitLogrus("WARN")
	}
}

// InitLogrus initializes logrus by setting a loglevel for it.
func InitLogrus(level string) {
	if len(level) == 0 {
		return
	}
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	case "FATAL":
		logrus.SetLevel(logrus.FatalLevel)
	case "PANIC":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		return
	}

	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)

}

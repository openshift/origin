package serviceability

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

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

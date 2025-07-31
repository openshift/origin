package logext


/*
  author: rioliu@redhat.com
*/

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/rs/zerolog"
)

const (
	// EnableDebugLog env variable to enable debug logging
	EnableDebugLog = "GINKGO_TEST_ENABLE_DEBUG_LOG"
)

// logWrapper wrapper interface for zerolog
type logWrapper struct {
	log zerolog.Logger
}

var logger = newLogger()

// NewLogger initialize log wrapper with zerolog logger
// default log level is INFO, user can enable debug logging by env variable GINKGO_TEST_ENABLE_DEBUG_LOG
func newLogger() *logWrapper {

	// customize time field format to sync with e2e framework
	zerolog.TimeFieldFormat = time.StampMilli
	// initialize customized output to integrate with GinkgoWriter
	output := zerolog.ConsoleWriter{Out: ginkgo.GinkgoWriter, TimeFormat: time.StampMilli}
	// customize level format e.g. INFO, DEBUG, ERROR
	output.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("%s:", i))
	}
	// disable colorful output for timestamp field
	output.FormatTimestamp = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	logger := &logWrapper{log: zerolog.New(output).With().Timestamp().Logger()}
	// set default log level to INFO
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	// if system env var is defined, enable debug logging
	if _, enabled := os.LookupEnv(EnableDebugLog); enabled {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return logger
}

// Infof log info level message
func Infof(format string, v ...interface{}) {
	logger.log.Info().Msgf(format, v...)
}

// Debugf log debug level message
func Debugf(format string, v ...interface{}) {
	logger.log.Debug().Msgf(format, v...)
}

// Errorf log error level message
func Errorf(format string, v ...interface{}) {
	logger.log.Error().Msgf(format, v...)
}

// Warnf log warning level message
func Warnf(format string, v ...interface{}) {
	logger.log.Warn().Msgf(format, v...)
}

package lagertest

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager"
)

type TestLogger struct {
	lager.Logger
	*TestSink
}

type TestSink struct {
	lager.Sink
	buffer *gbytes.Buffer
}

func NewTestLogger(component string) *TestLogger {
	logger := lager.NewLogger(component)

	testSink := NewTestSink()
	logger.RegisterSink(testSink)
	logger.RegisterSink(lager.NewWriterSink(ginkgo.GinkgoWriter, lager.DEBUG))

	return &TestLogger{logger, testSink}
}

func NewTestSink() *TestSink {
	buffer := gbytes.NewBuffer()

	return &TestSink{
		Sink:   lager.NewWriterSink(buffer, lager.DEBUG),
		buffer: buffer,
	}
}

func (s *TestSink) Buffer() *gbytes.Buffer {
	return s.buffer
}

func (s *TestSink) Logs() []lager.LogFormat {
	logs := []lager.LogFormat{}

	decoder := json.NewDecoder(bytes.NewBuffer(s.buffer.Contents()))
	for {
		var log lager.LogFormat
		if err := decoder.Decode(&log); err == io.EOF {
			return logs
		} else if err != nil {
			panic(err)
		}
		logs = append(logs, log)
	}

	return logs
}

func (s *TestSink) LogMessages() []string {
	logs := s.Logs()
	messages := make([]string, 0, len(logs))
	for _, log := range logs {
		messages = append(messages, log.Message)
	}
	return messages
}

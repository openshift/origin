package lagertest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

type TestLogger struct {
	lager.Logger
	*TestSink
}

type TestSink struct {
	writeLock *sync.Mutex
	lager.Sink
	buffer *gbytes.Buffer
	Errors []error
}

func NewTestLogger(component string) *TestLogger {
	logger := lager.NewLogger(component)

	testSink := NewTestSink()
	logger.RegisterSink(testSink)
	logger.RegisterSink(lager.NewWriterSink(ginkgo.GinkgoWriter, lager.DEBUG))

	return &TestLogger{logger, testSink}
}

func NewContext(parent context.Context, name string) context.Context {
	return lagerctx.NewContext(parent, NewTestLogger(name))
}

func NewTestSink() *TestSink {
	buffer := gbytes.NewBuffer()

	return &TestSink{
		writeLock: new(sync.Mutex),
		Sink:      lager.NewWriterSink(buffer, lager.DEBUG),
		buffer:    buffer,
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

func (s *TestSink) Log(log lager.LogFormat) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	if log.Error != nil {
		s.Errors = append(s.Errors, log.Error)
	}
	s.Sink.Log(log)
}

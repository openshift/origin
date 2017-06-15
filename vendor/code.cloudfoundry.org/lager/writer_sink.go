package lager

import (
	"io"
	"sync"
)

// A Sink represents a write destination for a Logger. It provides
// a thread-safe interface for writing logs
type Sink interface {
	//Log to the sink.  Best effort -- no need to worry about errors.
	Log(LogFormat)
}

type writerSink struct {
	writer      io.Writer
	minLogLevel LogLevel
	writeL      *sync.Mutex
}

func NewWriterSink(writer io.Writer, minLogLevel LogLevel) Sink {
	return &writerSink{
		writer:      writer,
		minLogLevel: minLogLevel,
		writeL:      new(sync.Mutex),
	}
}

func (sink *writerSink) Log(log LogFormat) {
	if log.LogLevel < sink.minLogLevel {
		return
	}

	sink.writeL.Lock()
	sink.writer.Write(log.ToJSON())
	sink.writer.Write([]byte("\n"))
	sink.writeL.Unlock()
}

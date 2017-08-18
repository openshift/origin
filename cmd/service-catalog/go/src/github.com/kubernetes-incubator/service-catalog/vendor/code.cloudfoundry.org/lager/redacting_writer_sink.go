package lager

import (
	"io"
	"sync"
)

type redactingWriterSink struct {
	writer      io.Writer
	minLogLevel LogLevel
	writeL      *sync.Mutex
	jsonRedacter *JSONRedacter
}

func NewRedactingWriterSink(writer io.Writer, minLogLevel LogLevel, keyPatterns []string, valuePatterns []string) (Sink, error) {
	jsonRedacter, err := NewJSONRedacter(keyPatterns, valuePatterns)
	if err != nil {
		return nil, err
	}
	return &redactingWriterSink{
		writer:      writer,
		minLogLevel: minLogLevel,
		writeL:      new(sync.Mutex),
		jsonRedacter: jsonRedacter,
	}, nil
}

func (sink *redactingWriterSink) Log(log LogFormat) {
	if log.LogLevel < sink.minLogLevel {
		return
	}

	sink.writeL.Lock()
	v := log.ToJSON()
	rv := sink.jsonRedacter.Redact(v)

	sink.writer.Write(rv)
	sink.writer.Write([]byte("\n"))
	sink.writeL.Unlock()
}




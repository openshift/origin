package lager

import "sync/atomic"

type ReconfigurableSink struct {
	sink Sink

	minLogLevel int32
}

func NewReconfigurableSink(sink Sink, initialMinLogLevel LogLevel) *ReconfigurableSink {
	return &ReconfigurableSink{
		sink: sink,

		minLogLevel: int32(initialMinLogLevel),
	}
}

func (sink *ReconfigurableSink) Log(log LogFormat) {
	minLogLevel := LogLevel(atomic.LoadInt32(&sink.minLogLevel))

	if log.LogLevel < minLogLevel {
		return
	}

	sink.sink.Log(log)
}

func (sink *ReconfigurableSink) SetMinLevel(level LogLevel) {
	atomic.StoreInt32(&sink.minLogLevel, int32(level))
}

func (sink *ReconfigurableSink) GetMinLevel() LogLevel {
	return LogLevel(atomic.LoadInt32(&sink.minLogLevel))
}

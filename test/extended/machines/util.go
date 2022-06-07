package operators

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

// BufferedLogger represents a logger interface which is buffered.
// It only writes to the output when flushed.
// It can be reset at any time by calling Reset().
// This is useful if you want to log only the last iteration of an
// asynchronous assertion.
type BufferedLogger interface {
	Reset()
	Flush()
	Logf(string, ...interface{})
}

// bufferedLogger keeps a track of whatever was written to it last, and writes that out
// to the underlying writer when flushed.
type bufferedLogger struct {
	buffer *bytes.Buffer
	output io.Writer
}

// NewBufferedLogger creates a new BufferedLogger which will flush to the output given.
func NewBufferedLogger(out io.Writer) BufferedLogger {
	return &bufferedLogger{
		buffer: bytes.NewBuffer(nil),
		output: out,
	}
}

// Reset resets the contents of the buffer in the logger.
func (l *bufferedLogger) Reset() {
	l.buffer = bytes.NewBuffer(nil)
}

func (l *bufferedLogger) log(level string, format string, args ...interface{}) {
	fmt.Fprintf(l.buffer, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf logs the info.
func (l *bufferedLogger) Logf(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

// Flush writes the loggers buffer to the output.
func (l *bufferedLogger) Flush() {
	l.output.Write(l.buffer.Bytes())
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

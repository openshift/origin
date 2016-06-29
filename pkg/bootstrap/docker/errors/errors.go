package errors

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/util/prefixwriter"
)

type Error interface {
	error
	WithCause(error) Error
	WithSolution(string, ...interface{}) Error
	WithDetails(string) Error
}

func NewError(msg string, args ...interface{}) Error {
	return &internalError{
		msg: fmt.Sprintf(msg, args...),
	}
}

type internalError struct {
	msg      string
	cause    error
	solution string
	details  string
}

func (e *internalError) Error() string {
	return e.msg
}

func (e *internalError) Cause() error {
	return e.cause
}

func (e *internalError) Solution() string {
	return e.solution
}

func (e *internalError) Details() string {
	return e.details
}

func (e *internalError) WithCause(err error) Error {
	e.cause = err
	return e
}

func (e *internalError) WithDetails(details string) Error {
	e.details = details
	return e
}

func (e *internalError) WithSolution(solution string, args ...interface{}) Error {
	e.solution = fmt.Sprintf(solution, args...)
	return e
}

func LogError(err error) {
	if err == nil {
		return
	}
	glog.V(1).Infof("Unexpected error: %v", err)
	if glog.V(5) {
		debug.PrintStack()
	}
}

func PrintLog(out io.Writer, title string, content []byte) {
	fmt.Fprintf(out, "%s:\n", title)
	w := prefixwriter.New("  ", out)
	w.Write(bytes.TrimSpace(content))
	fmt.Fprintf(out, "\n")
}

package errors

import (
	"fmt"
)

type Error interface {
	error
	WithCause(error) Error
	WithSolution(string) Error
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
	if e.cause != nil && len(e.cause.Error()) > 0 {
		return e.msg + "; caused by: " + e.cause.Error()
	}
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

func (e *internalError) WithSolution(solution string) Error {
	e.solution = solution
	return e
}

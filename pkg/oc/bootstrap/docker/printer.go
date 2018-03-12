package docker

import (
	"bytes"
	"fmt"
	"io"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/util/prefixwriter"
)

const (
	taskNamePrefix = "-- "
	taskIndent     = "   "
)

// TaskPrinter is a helper for start task output
type TaskPrinter struct {
	taskWriter *taskWriter
	out        io.Writer
}

// NewTaskPrinter creates a new TaskPrinter
func NewTaskPrinter(out io.Writer) *TaskPrinter {
	return &TaskPrinter{
		out: out,
	}
}

// StartTask writes out the header for a task
func (p *TaskPrinter) StartTask(name string) {
	fmt.Fprintf(p.out, "%s%s ... ", taskNamePrefix, name)
	if glog.V(1) {
		fmt.Fprintln(p.out)
	}
}

// Success writes out a success marker for a task
func (p *TaskPrinter) Success() {
	if (p.taskWriter != nil && p.taskWriter.used) || bool(glog.V(1)) {
		return
	}
	fmt.Fprintln(p.out, "OK")
}

// TaskWriter is a writer that can be used to write task output
func (p *TaskPrinter) TaskWriter() io.Writer {
	p.taskWriter = &taskWriter{w: p.out}
	return prefixwriter.New(taskIndent, p.taskWriter)
}

// Failure writes out a failure marker for a task and outputs the error
// that caused the failure
func (p *TaskPrinter) Failure(err error) {
	fmt.Fprintln(p.out, "FAIL")
	PrintError(err, prefixwriter.New(taskIndent, p.out))
}

type hasCause interface {
	Cause() error
}

type hasDetails interface {
	Details() string
}

type hasSolution interface {
	Solution() string
}

func PrintError(err error, out io.Writer) {
	fmt.Fprintf(out, "Error: %v\n", err)
	if d, ok := err.(hasDetails); ok && len(d.Details()) > 0 {
		fmt.Fprintln(out, "Details:")
		w := prefixwriter.New("  ", out)
		fmt.Fprintf(w, "%s\n", d.Details())
	}
	if s, ok := err.(hasSolution); ok && len(s.Solution()) > 0 {
		fmt.Fprintln(out, "Solution:")
		w := prefixwriter.New("  ", out)
		fmt.Fprintf(w, "%s\n", s.Solution())
	}
	if c, ok := err.(hasCause); ok && c.Cause() != nil {
		fmt.Fprintln(out, "Caused By:")
		w := prefixwriter.New("  ", out)
		PrintError(c.Cause(), w)
	}
}

type taskWriter struct {
	w    io.Writer
	used bool
}

func (t *taskWriter) Write(p []byte) (n int, err error) {
	if !t.used {
		t.used = true
		t.w.Write([]byte("\n"))
	}
	return t.w.Write(p)
}

// ToError provides a way to return the pretty error output from a task without special casing
// the code to only work in a narrow case while running the command.
func (p *TaskPrinter) ToError(err error) error {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "FAIL")
	PrintError(err, prefixwriter.New(taskIndent, buf))
	return taskError{message: buf.String()}
}

type taskError struct {
	message string
}

func (e taskError) Error() string {
	return e.message
}

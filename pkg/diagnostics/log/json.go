package log

import (
	"encoding/json"
	"fmt"
	"io"
)

type jsonLogger struct {
	out         io.Writer
	logStarted  bool
	logFinished bool
}

func (j *jsonLogger) Write(entry Entry) {
	if j.logStarted {
		fmt.Fprintln(j.out, ",")
	} else {
		fmt.Fprintln(j.out, "[")
	}
	j.logStarted = true
	b, _ := json.MarshalIndent(entry, "  ", "  ")
	fmt.Print("  " + string(b))
}
func (j *jsonLogger) Finish() {
	if j.logStarted {
		fmt.Fprintln(j.out, "\n]")
	} else if !j.logFinished {
		fmt.Fprintln(j.out, "[]")
	}
	j.logFinished = true
}

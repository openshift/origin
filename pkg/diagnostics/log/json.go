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

func (j *jsonLogger) Write(l Level, msg Msg) {
	if j.logStarted {
		fmt.Fprintln(j.out, ",")
	} else {
		fmt.Fprintln(j.out, "[")
	}
	j.logStarted = true
	msg["level"] = l.Name
	b, _ := json.MarshalIndent(msg, "  ", "  ")
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

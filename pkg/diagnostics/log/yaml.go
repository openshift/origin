package log

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
)

type yamlLogger struct {
	out        io.Writer
	logStarted bool
}

func (y *yamlLogger) Write(entry LogEntry) {
	b, _ := yaml.Marshal(&entry)
	fmt.Fprintln(y.out, "---\n"+string(b))
}
func (y *yamlLogger) Finish() {}

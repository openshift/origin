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

func (y *yamlLogger) Write(l Level, msg Msg) {
	msg["level"] = l.Name
	b, _ := yaml.Marshal(&msg)
	fmt.Fprintln(y.out, "---\n"+string(b))
}
func (y *yamlLogger) Finish() {}

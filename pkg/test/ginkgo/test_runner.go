package ginkgo

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

type commandContext struct {
	env []string
}

// construction provided so that if we add anything, we get a compile failure for all callers instead of weird behavior
func newCommandContext(env []string) commandContext {
	return commandContext{
		env: env,
	}
}

func (c commandContext) commandString(test *testCase) string {
	buf := &bytes.Buffer{}
	for _, env := range c.env {
		parts := strings.SplitN(env, "=", 2)
		fmt.Fprintf(buf, "%s=%q ", parts[0], parts[1])
	}
	fmt.Fprintf(buf, "%s %s %q", os.Args[0], "run-test", test.name)
	return buf.String()
}

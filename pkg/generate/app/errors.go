package app

import (
	"bytes"
	"fmt"
)

type ErrNoMatch struct {
	value     string
	qualifier string
}

func (e ErrNoMatch) Error() string {
	if len(e.qualifier) != 0 {
		return fmt.Sprintf("no image matched %q: %s", e.value, e.qualifier)
	}
	return fmt.Sprintf("no image matched %q", e.value)
}

func (e ErrNoMatch) UsageError(commandName string) string {
	return fmt.Sprintf("%[3]s - does a Docker image with that name exist?", e.value, commandName, e.Error())

	/*`
	  %[3]s - you can try to search for images or templates that may match this name with:

	      $ %[2]s -S %[1]q

	  `*/
}

type ErrMultipleMatches struct {
	image   string
	matches []*ComponentMatch
}

func (e ErrMultipleMatches) Error() string {
	return fmt.Sprintf("multiple images matched %q: %d", e.image, len(e.matches))
}

func (e ErrMultipleMatches) UsageError(commandName string) string {
	buf := &bytes.Buffer{}
	for _, match := range e.matches {
		fmt.Fprintf(buf, "* %[1]s (use %[2]s)\n", match.Name, match.Argument)
		fmt.Fprintf(buf, "  %s\n\n", match.Description)
	}
	return fmt.Sprintf(`
The argument %[1]q could apply to the following images or image repositories:

%[2]s
`, e.image, buf.String())
}

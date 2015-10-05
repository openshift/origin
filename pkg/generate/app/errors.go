package app

import (
	"bytes"
	"fmt"
)

// ErrNoMatch is an error returned to new-app users when no match
// is found for a given component
type ErrNoMatch struct {
	value     string
	qualifier string
}

func (e ErrNoMatch) Error() string {
	if len(e.qualifier) != 0 {
		return fmt.Sprintf("no image or template matched %q: %s, specify allow-missing-images to use this image name.", e.value, e.qualifier)
	}
	return fmt.Sprintf("no image or template matched %q, specify --allow-missing-images to use this image name.", e.value)
}

// UsageError is the usage error message returned when no match is found
func (e ErrNoMatch) UsageError(commandName string) string {
	return fmt.Sprintf("%[3]s - does a Docker image with that name exist?", e.value, commandName, e.Error())

	/*`
	  %[3]s - you can try to search for images or templates that may match this name with:

	      $ %[2]s -S %[1]q

	  `*/
}

// ErrMultipleMatches is an error returned to new-app users when multiple
// matches are found for a given component
type ErrMultipleMatches struct {
	Image   string
	Matches []*ComponentMatch
}

func (e ErrMultipleMatches) Error() string {
	return fmt.Sprintf("multiple images or templates matched %q: %d", e.Image, len(e.Matches))
}

// UsageError is the usage error message returned when multiple matches are found
func (e ErrMultipleMatches) UsageError(commandName string) string {
	buf := &bytes.Buffer{}
	for _, match := range e.Matches {
		fmt.Fprintf(buf, "* %s %f\n", match.Description, match.Score)
		fmt.Fprintf(buf, "  Use %[1]s to specify this image or template\n\n", match.Argument)
	}
	return fmt.Sprintf(`
The argument %[1]q could apply to the following Docker images or OpenShift image repositories:

%[2]s
`, e.Image, buf.String())
}

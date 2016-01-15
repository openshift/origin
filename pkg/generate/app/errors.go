package app

import (
	"bytes"
	"fmt"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ErrNoMatch is the error returned by new-app when no match is found for a
// given component.
type ErrNoMatch struct {
	value     string
	qualifier string
}

func (e ErrNoMatch) Error() string {
	if len(e.qualifier) != 0 {
		return fmt.Sprintf("no match for %q: %s, specify --allow-missing-images to use this image name.", e.value, e.qualifier)
	}
	return fmt.Sprintf("no match for %q, specify --allow-missing-images to use this image name.", e.value)
}

// UsageError is the usage error message returned when no match is found.
func (e ErrNoMatch) UsageError(commandName string) string {
	return fmt.Sprintf("%[3]s - does a Docker image with that name exist?", e.value, commandName, e.Error())
}

// ErrPartialMatch is the error returned to new-app users when the
// best match available is only a partial match for a given component.
type ErrPartialMatch struct {
	Image string
	Match *ComponentMatch
}

func (e ErrPartialMatch) Error() string {
	return fmt.Sprintf("only a partial match was found for %q: %q", e.Image, e.Match.Name)
}

// UsageError is the usage error message returned when only a partial match is
// found.
func (e ErrPartialMatch) UsageError(commandName string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "* %s\n", e.Match.Description)
	fmt.Fprintf(buf, "  Use %[1]s to specify this image or template\n\n", e.Match.Argument)

	return fmt.Sprintf(`
The argument %[1]q only partially matched the following Docker image or OpenShift image stream:

%[2]s
`, e.Image, buf.String())
}

// ErrMultipleMatches is the error returned to new-app users when multiple
// matches are found for a given component.
type ErrMultipleMatches struct {
	Image   string
	Matches []*ComponentMatch
}

func (e ErrMultipleMatches) Error() string {
	return fmt.Sprintf("multiple images or templates matched %q: %d", e.Image, len(e.Matches))
}

// UsageError is the usage error message returned when multiple matches are
// found.
func (e ErrMultipleMatches) UsageError(commandName string) string {
	buf := &bytes.Buffer{}
	for _, match := range e.Matches {
		fmt.Fprintf(buf, "* %s\n", match.Description)
		fmt.Fprintf(buf, "  Use %[1]s to specify this image or template\n\n", match.Argument)
	}
	return fmt.Sprintf(`
The argument %[1]q could apply to the following Docker images or OpenShift image streams:

%[2]s
`, e.Image, buf.String())
}

// ErrNameRequired is the error returned by new-app when a name cannot be
// suggested and the user needs to provide one explicitly.
var ErrNameRequired = fmt.Errorf("you must specify a name for your app")

// CircularOutputReferenceError is the error returned by new-app when the input
// and output image stream tags are identical.
type CircularOutputReferenceError struct {
	Reference imageapi.DockerImageReference
}

func (e CircularOutputReferenceError) Error() string {
	return fmt.Sprintf("the input and output image stream tags are identical (%q)", e.Reference.DockerClientDefaults())
}

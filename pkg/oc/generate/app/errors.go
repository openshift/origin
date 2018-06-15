package app

import (
	"bytes"
	"fmt"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// ErrNoMatch is the error returned by new-app when no match is found for a
// given component.
type ErrNoMatch struct {
	Value     string
	Type      string
	Qualifier string
	Errs      []error
}

func (e ErrNoMatch) Error() string {
	if len(e.Qualifier) != 0 {
		return fmt.Sprintf("unable to locate any %s with name %q: %s", e.Type, e.Value, e.Qualifier)
	}
	return fmt.Sprintf("unable to locate any %s with name %q", e.Type, e.Value)
}

// Suggestion is the usage error message returned when no match is found.
func (e ErrNoMatch) Suggestion(commandName string) string {
	return fmt.Sprintf("%[3]s - does a Docker image with that name exist?", e.Value, commandName, e.Error())
}

// ErrPartialMatch is the error returned to new-app users when the
// best match available is only a partial match for a given component.
type ErrPartialMatch struct {
	Value string
	Match *ComponentMatch
	Errs  []error
}

func (e ErrPartialMatch) Error() string {
	return fmt.Sprintf("only a partial match was found for %q: %q", e.Value, e.Match.Name)
}

// Suggestion is the usage error message returned when only a partial match is
// found.
func (e ErrPartialMatch) Suggestion(commandName string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "* %s\n", e.Match.Description)
	fmt.Fprintf(buf, "  Use %[1]s to specify this image or template\n\n", e.Match.Argument)

	return fmt.Sprintf(`%[3]s
The argument %[1]q only partially matched the following Docker image or OpenShift image stream:

%[2]s
`, e.Value, buf.String(), cmdutil.MultipleErrors("error: ", e.Errs))
}

// ErrNoTagsFound is returned when a matching image stream has no tags associated with it
type ErrNoTagsFound struct {
	Value string
	Match *ComponentMatch
	Errs  []error
}

func (e ErrNoTagsFound) Error() string {
	imageStream := fmt.Sprintf("%s/%s", e.Match.ImageStream.Namespace, e.Match.ImageStream.Name)
	return fmt.Sprintf("no tags found on matching image stream: %q", imageStream)
}

// Suggestion is the usage error message returned when no tags are found on matching image stream
func (e ErrNoTagsFound) Suggestion(commandName string) string {

	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "* %s\n", e.Match.Description)
	fmt.Fprintf(buf, "  Use --allow-missing-imagestreamtags to use this image stream\n\n")

	return fmt.Sprintf(`%[3]s
The argument %[1]q matched the following OpenShift image stream which has no tags:

%[2]s
`, e.Value, buf.String(), cmdutil.MultipleErrors("error: ", e.Errs))
}

// ErrMultipleMatches is the error returned to new-app users when multiple
// matches are found for a given component.
type ErrMultipleMatches struct {
	Value   string
	Matches []*ComponentMatch
	Errs    []error
}

func (e ErrMultipleMatches) Error() string {
	return fmt.Sprintf("multiple images or templates matched %q: %d", e.Value, len(e.Matches))
}

// ErrNameRequired is the error returned by new-app when a name cannot be
// suggested and the user needs to provide one explicitly.
var ErrNameRequired = fmt.Errorf("you must specify a name for your app")

// CircularOutputReferenceError is the error returned by new-app when the input
// and output image stream tags are identical.
type CircularOutputReferenceError struct {
	Reference string
}

func (e CircularOutputReferenceError) Error() string {
	return fmt.Sprintf("output image of %q should be different than input", e.Reference)
}

// CircularReferenceError is the error returned by new-app when either the input
// or output image stream tags employ circular loops
type CircularReferenceError struct {
	Reference string
}

func (e CircularReferenceError) Error() string {
	return fmt.Sprintf("image stream tag reference %q is a circular loop of image stream tags", e.Reference)
}

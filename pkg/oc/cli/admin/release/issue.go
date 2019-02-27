package release

import (
	"fmt"
)

type issue struct {
	// Store for the issue, e.g. "rhbz" for https://bugzilla.redhat.com, or "origin" for https://github.com/openshift/origin/issues.
	Store string

	// ID for the issue, e.g. 123.
	ID int

	// URI for the issue, e.g. https://bugzilla.redhat.com/show_bug.cgi?id=123.
	URI string
}

func (i *issue) Markdown() string {
	return fmt.Sprintf("[%s#%d](%s)", i.Store, i.ID, i.URI) // TODO: proper escaping
}

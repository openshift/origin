package release

import (
	"fmt"
)

type issue struct {
	// ID for the issue, e.g. 123.
	ID int

	// URI for the issue, e.g. https://bugzilla.redhat.com/show_bug.cgi?id=123.
	URI string
}

func (i *issue) Markdown() string {
	return fmt.Sprintf("[rhbz#%d](%s)", i.ID, i.URI)
}

package foo

import "github.com/Azure/go-autorest/autorest"

// Gateway ...
type Gateway struct {
	autorest.Response `json:"-"`
	// Field1 ...
	Field1 *int
	// Field2 ...
	Field2 *string
}

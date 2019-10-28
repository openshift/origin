package jspointer

import "errors"

// Errors used in jspointer package
var (
	ErrInvalidPointer        = errors.New("invalid pointer")
	ErrCanNotSet             = errors.New("field cannot be set to")
	ErrSliceIndexOutOfBounds = errors.New("slice index out of bounds")
)

// Consntants used in jspointer package. Mostly for internal usage only
const (
	EncodedTilde = "~0"
	EncodedSlash = "~1"
	Separator    = '/'
)

type ErrNotFound struct {
	Ptr string
}

// JSPointer represents a JSON pointer
type JSPointer struct {
	raw    string
	tokens tokens
}

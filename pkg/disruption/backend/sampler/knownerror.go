package sampler

import (
	"fmt"
)

type KnownError struct {
	category string
	err      error
}

func (ke KnownError) Unwrap() error {
	return ke.err
}

func (ke KnownError) Error() string {
	return fmt.Sprintf("category: %s err: %v", ke.category, ke.err)
}

func (ke KnownError) Category() string {
	return ke.category
}

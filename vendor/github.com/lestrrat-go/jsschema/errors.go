package schema

import "github.com/pkg/errors"

func errInvalidType(s string, v interface{}) error {
	return errors.Errorf("invalid type: expected %s, got %T", s, v)
}

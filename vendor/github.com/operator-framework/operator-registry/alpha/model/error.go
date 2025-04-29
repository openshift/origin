package model

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type validationError struct {
	message   string
	subErrors []error
}

func newValidationError(message string) *validationError {
	return &validationError{message: message}
}

func (v *validationError) orNil() error {
	if len(v.subErrors) == 0 {
		return nil
	}
	return v
}

func (v *validationError) Error() string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(v.errorPrefix(nil, true, nil))
}

func (v *validationError) errorPrefix(prefix []rune, last bool, seen []error) string {
	for _, s := range seen {
		if errors.Is(v, s) {
			return ""
		}
	}
	seen = append(seen, v)
	sep := ":\n"
	if len(v.subErrors) == 0 {
		sep = "\n"
	}
	errMsg := bytes.NewBufferString(fmt.Sprintf("%s%s%s", string(prefix), v.message, sep))
	for i, serr := range v.subErrors {
		subPrefix := prefix
		if len(subPrefix) >= 4 {
			if last {
				subPrefix = append(subPrefix[0:len(subPrefix)-4], []rune("    ")...)
			} else {
				subPrefix = append(subPrefix[0:len(subPrefix)-4], []rune("│   ")...)
			}
		}
		subLast := i == len(v.subErrors)-1
		if subLast {
			subPrefix = append(subPrefix, []rune("└── ")...)
		} else {
			subPrefix = append(subPrefix, []rune("├── ")...)
		}

		var verr *validationError
		if errors.As(serr, &verr) {
			errMsg.WriteString(verr.errorPrefix(subPrefix, subLast, seen))
		} else {
			errMsg.WriteString(fmt.Sprintf("%s%s\n", string(subPrefix), serr))
		}
	}
	return errMsg.String()
}

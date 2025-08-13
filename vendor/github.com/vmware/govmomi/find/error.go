// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package find

import "fmt"

type NotFoundError struct {
	kind string
	path string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s '%s' not found", e.kind, e.path)
}

type MultipleFoundError struct {
	kind string
	path string
}

func (e *MultipleFoundError) Error() string {
	return fmt.Sprintf("path '%s' resolves to multiple %ss", e.path, e.kind)
}

type DefaultNotFoundError struct {
	kind string
}

func (e *DefaultNotFoundError) Error() string {
	return fmt.Sprintf("no default %s found", e.kind)
}

type DefaultMultipleFoundError struct {
	kind string
}

func (e DefaultMultipleFoundError) Error() string {
	return fmt.Sprintf("default %s resolves to multiple instances, please specify", e.kind)
}

func toDefaultError(err error) error {
	switch e := err.(type) {
	case *NotFoundError:
		return &DefaultNotFoundError{e.kind}
	case *MultipleFoundError:
		return &DefaultMultipleFoundError{e.kind}
	default:
		return err
	}
}

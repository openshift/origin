package errors

import kapierrors "k8s.io/kubernetes/pkg/api/errors"

// TolerateNotFoundError tolerates 'not found' errors
func TolerateNotFoundError(err error) error {
	if kapierrors.IsNotFound(err) {
		return nil
	}
	return err
}

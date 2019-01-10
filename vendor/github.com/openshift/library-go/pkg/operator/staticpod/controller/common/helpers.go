package common

import (
	"strings"

	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"k8s.io/apimachinery/pkg/api/equality"
	errutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// UpdateStatusFunc is a func that mutates an operator status.
type UpdateStatusFunc func(status *operatorv1.StaticPodOperatorStatus) error

// UpdateStatus applies the update funcs to the oldStatus abd tries to update via the client.
func UpdateStatus(client OperatorClient, updateFuncs ...UpdateStatusFunc) (bool, error) {
	updated := false
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, oldStatus, resourceVersion, err := client.Get()
		if err != nil {
			return err
		}

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newStatus); err != nil {
				return err
			}
		}

		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			return nil
		}

		_, err = client.UpdateStatus(resourceVersion, newStatus)
		updated = err == nil
		return err
	})

	return updated, err
}

// UpdateConditionFunc returns a func to update a condition.
func UpdateConditionFn(cond operatorv1.OperatorCondition) UpdateStatusFunc {
	return func(oldStatus *operatorv1.StaticPodOperatorStatus) error {
		v1helpers.SetOperatorCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

type aggregate []error

var _ errutils.Aggregate = aggregate{}

// NewMultiLineAggregate returns an aggregate error with multi-line output
func NewMultiLineAggregate(errList []error) error {
	var errs []error
	for _, e := range errList {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return aggregate(errs)
}

// Error is part of the error interface.
func (agg aggregate) Error() string {
	msgs := make([]string, len(agg))
	for i := range agg {
		msgs[i] = agg[i].Error()
	}
	return strings.Join(msgs, "\n")
}

// Errors is part of the Aggregate interface.
func (agg aggregate) Errors() []error {
	return []error(agg)
}

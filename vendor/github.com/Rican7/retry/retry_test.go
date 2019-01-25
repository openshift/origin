package retry

import (
	"errors"
	"testing"
)

func TestRetry(t *testing.T) {
	action := func(attempt uint) error {
		return nil
	}

	err := Retry(action)

	if nil != err {
		t.Error("expected a nil error")
	}
}

func TestRetryRetriesUntilNoErrorReturned(t *testing.T) {
	const errorUntilAttemptNumber = 5

	var attemptsMade uint

	action := func(attempt uint) error {
		attemptsMade = attempt

		if errorUntilAttemptNumber == attempt {
			return nil
		}

		return errors.New("erroring")
	}

	err := Retry(action)

	if nil != err {
		t.Error("expected a nil error")
	}

	if errorUntilAttemptNumber != attemptsMade {
		t.Errorf(
			"expected %d attempts to be made, but %d were made instead",
			errorUntilAttemptNumber,
			attemptsMade,
		)
	}
}

func TestShouldAttempt(t *testing.T) {
	shouldAttempt := shouldAttempt(1)

	if !shouldAttempt {
		t.Error("expected to return true")
	}
}

func TestShouldAttemptWithStrategy(t *testing.T) {
	const attemptNumberShouldReturnFalse = 7

	strategy := func(attempt uint) bool {
		return (attemptNumberShouldReturnFalse != attempt)
	}

	should := shouldAttempt(1, strategy)

	if !should {
		t.Error("expected to return true")
	}

	should = shouldAttempt(1+attemptNumberShouldReturnFalse, strategy)

	if !should {
		t.Error("expected to return true")
	}

	should = shouldAttempt(attemptNumberShouldReturnFalse, strategy)

	if should {
		t.Error("expected to return false")
	}
}

func TestShouldAttemptWithMultipleStrategies(t *testing.T) {
	trueStrategy := func(attempt uint) bool {
		return true
	}

	falseStrategy := func(attempt uint) bool {
		return false
	}

	should := shouldAttempt(1, trueStrategy)

	if !should {
		t.Error("expected to return true")
	}

	should = shouldAttempt(1, falseStrategy)

	if should {
		t.Error("expected to return false")
	}

	should = shouldAttempt(1, trueStrategy, trueStrategy, trueStrategy)

	if !should {
		t.Error("expected to return true")
	}

	should = shouldAttempt(1, falseStrategy, falseStrategy, falseStrategy)

	if should {
		t.Error("expected to return false")
	}

	should = shouldAttempt(1, trueStrategy, trueStrategy, falseStrategy)

	if should {
		t.Error("expected to return false")
	}
}

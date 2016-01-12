package api

import "time"

// SetDuration sets the runtime duration of the test case
func (t *TestCase) SetDuration(duration string) error {
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		return err
	}

	// we round to the millisecond on duration
	t.Duration = float64(int(parsedDuration.Seconds()*1000)) / 1000
	return nil
}

// MarkSkipped marks the test as skipped with the given message
func (t *TestCase) MarkSkipped(message string) {
	t.SkipMessage = &SkipMessage{
		Message: message,
	}
}

// MarkFailed marks the test as failed with the given message and output
func (t *TestCase) MarkFailed(message, output string) {
	t.FailureOutput = &FailureOutput{
		Message: message,
		Output:  output,
	}
}

package api

import "time"

// AddProperty adds a property to the test suite, deduplicating multiple additions of the same property
// by overwriting the previous record to reflect the new values
func (t *TestSuite) AddProperty(name, value string) {
	for _, property := range t.Properties {
		if property.Name == name {
			property.Value = value
			return
		}
	}

	t.Properties = append(t.Properties, &TestSuiteProperty{Name: name, Value: value})
}

// AddTestCase adds a test case to the test suite and updates test suite metrics as necessary
func (t *TestSuite) AddTestCase(testCase *TestCase) {
	t.NumTests += 1

	switch {
	case testCase.SkipMessage != nil:
		t.NumSkipped += 1
	case testCase.FailureOutput != nil:
		t.NumFailed += 1
	default:
		// we do not preserve output on tests that are not failures or skips
		testCase.SystemOut = ""
		testCase.SystemErr = ""
	}

	t.Duration += testCase.Duration
	// we round to the millisecond on duration
	t.Duration = float64(int(t.Duration*1000)) / 1000

	t.TestCases = append(t.TestCases, testCase)
}

// SetDuration sets the duration of the test suite if this value is not calculated by aggregating the durations
// of all of the substituent test cases. This should *not* be used if the total duration of the test suite is
// calculated as that sum, as AddTestCase will handle that case.
func (t *TestSuite) SetDuration(duration string) error {
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		return err
	}

	// we round to the millisecond on duration
	t.Duration = float64(int(parsedDuration.Seconds()*1000)) / 1000
	return nil
}

// ByName implements sort.Interface for []*TestSuite based on the Name field
type ByName []*TestSuite

func (n ByName) Len() int {
	return len(n)
}

func (n ByName) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n ByName) Less(i, j int) bool {
	return n[i].Name < n[j].Name
}

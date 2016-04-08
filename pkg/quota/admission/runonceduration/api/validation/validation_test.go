package validation

import (
	"testing"

	"github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
)

func TestRunOnceDurationConfigValidation(t *testing.T) {
	// Check invalid duration returns an error
	var invalidSecs int64 = -1
	invalidConfig := &api.RunOnceDurationConfig{
		ActiveDeadlineSecondsLimit: &invalidSecs,
	}
	errs := ValidateRunOnceDurationConfig(invalidConfig)
	if len(errs) == 0 {
		t.Errorf("Did not get expected error on invalid config")
	}

	// Check that valid duration returns no error
	var validSecs int64 = 5
	validConfig := &api.RunOnceDurationConfig{
		ActiveDeadlineSecondsLimit: &validSecs,
	}
	errs = ValidateRunOnceDurationConfig(validConfig)
	if len(errs) > 0 {
		t.Errorf("Unexpected error on valid config")
	}
}

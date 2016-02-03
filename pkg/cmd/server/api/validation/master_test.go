package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/api"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func TestFailingAPIServerArgs(t *testing.T) {
	args := configapi.ExtendedArguments{}
	args["port"] = []string{"invalid-value"}
	args["missing-key"] = []string{"value"}

	// [port: invalid value '[invalid-value]': could not be set: strconv.ParseUint: parsing "invalid-value": invalid syntax flag: invalid value 'missing-key': is not a valid flag]

	errs := ValidateAPIServerExtendedArguments(args, nil)

	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, not %v", errs)
	}

	var (
		portErr    *field.Error
		missingErr *field.Error
	)
	for _, err := range errs {
		switch err.Field {
		case "port":
			portErr = err
		case "flag":
			missingErr = err
		}
	}

	if portErr == nil {
		t.Fatalf("missing port")
	}
	if missingErr == nil {
		t.Fatalf("missing missing-key")
	}

	if e, a := "port", portErr.Field; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "invalid-value", portErr.BadValue.(string); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := `could not be set: strconv.ParseInt: parsing "invalid-value": invalid syntax`, portErr.Detail; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	if e, a := "flag", missingErr.Field; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "missing-key", missingErr.BadValue.(string); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := `is not a valid flag`, missingErr.Detail; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestFailingControllerArgs(t *testing.T) {
	args := configapi.ExtendedArguments{}
	args["port"] = []string{"invalid-value"}
	args["missing-key"] = []string{"value"}

	// [port: invalid value '[invalid-value]': could not be set: strconv.ParseUint: parsing "invalid-value": invalid syntax flag: invalid value 'missing-key': is not a valid flag]

	errs := ValidateControllerExtendedArguments(args, nil)

	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, not %v", errs)
	}

	var (
		portErr    *field.Error
		missingErr *field.Error
	)
	for _, err := range errs {
		switch err.Field {
		case "port":
			portErr = err
		case "flag":
			missingErr = err
		}
	}

	if portErr == nil {
		t.Fatalf("missing port")
	}
	if missingErr == nil {
		t.Fatalf("missing missing-key")
	}

	if e, a := "port", portErr.Field; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "invalid-value", portErr.BadValue.(string); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := `could not be set: strconv.ParseInt: parsing "invalid-value": invalid syntax`, portErr.Detail; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	if e, a := "flag", missingErr.Field; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := "missing-key", missingErr.BadValue.(string); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := `is not a valid flag`, missingErr.Detail; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestValidate_ValidateEtcdStorageConfig(t *testing.T) {
	osField := "openShiftStorageVersion"
	kubeField := "kubernetesStorageVersion"
	tests := []struct {
		label                   string
		kubeStorageVersion      string
		openshiftStorageVersion string
		name                    string
		expected                field.ErrorList
	}{
		{
			label:                   "valid levels",
			kubeStorageVersion:      "v1",
			openshiftStorageVersion: "v1",
			expected:                field.ErrorList{},
		},
		{
			label:                   "unknown openshift level",
			kubeStorageVersion:      "v1",
			openshiftStorageVersion: "bogus",
			expected: field.ErrorList{
				field.NotSupported(field.NewPath(osField), "bogus", []string{"v1"}),
			},
		},
		{
			label:                   "unsupported openshift level",
			kubeStorageVersion:      "v1",
			openshiftStorageVersion: "v1beta3",
			expected: field.ErrorList{
				field.NotSupported(field.NewPath(osField), "v1beta3", []string{"v1"}),
			},
		},
		{
			label:                   "missing openshift level",
			kubeStorageVersion:      "v1",
			openshiftStorageVersion: "",
			expected: field.ErrorList{
				field.Required(field.NewPath(osField), ""),
			},
		},
		{
			label:                   "unknown kube level",
			kubeStorageVersion:      "bogus",
			openshiftStorageVersion: "v1",
			expected: field.ErrorList{
				field.NotSupported(field.NewPath(kubeField), "bogus", []string{"v1"}),
			},
		},
		{
			label:                   "unsupported kube level",
			kubeStorageVersion:      "v1beta3",
			openshiftStorageVersion: "v1",
			expected: field.ErrorList{
				field.NotSupported(field.NewPath(kubeField), "v1beta3", []string{"v1"}),
			},
		},
		{
			label:                   "missing kube level",
			kubeStorageVersion:      "",
			openshiftStorageVersion: "v1",
			expected: field.ErrorList{
				field.Required(field.NewPath(kubeField), ""),
			},
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.label)
		config := api.EtcdStorageConfig{
			OpenShiftStorageVersion:  test.openshiftStorageVersion,
			KubernetesStorageVersion: test.kubeStorageVersion,
		}
		results := ValidateEtcdStorageConfig(config, nil)
		if !kapi.Semantic.DeepEqual(test.expected, results) {
			t.Errorf("unexpected validation results; diff:\n%v", util.ObjectDiff(test.expected, results))
			return
		}
	}
}

func TestValidateAdmissionPluginConfig(t *testing.T) {
	locationOnly := configapi.AdmissionPluginConfig{
		Location: "/some/location",
	}
	configOnly := configapi.AdmissionPluginConfig{
		Configuration: runtime.EmbeddedObject{
			Object: &configapi.AdmissionPluginConfig{},
		},
	}
	locationAndConfig := configapi.AdmissionPluginConfig{
		Location: "/some/location",
		Configuration: runtime.EmbeddedObject{
			Object: &configapi.AdmissionPluginConfig{},
		},
	}
	bothEmpty := configapi.AdmissionPluginConfig{}

	tests := []struct {
		config      map[string]configapi.AdmissionPluginConfig
		expectError bool
	}{
		{
			config: map[string]configapi.AdmissionPluginConfig{
				"one": locationOnly,
				"two": configOnly,
			},
		},
		{
			config: map[string]configapi.AdmissionPluginConfig{
				"one": locationOnly,
				"two": locationAndConfig,
			},
			expectError: true,
		},
		{
			config: map[string]configapi.AdmissionPluginConfig{
				"one": configOnly,
				"two": bothEmpty,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		errs := ValidateAdmissionPluginConfig(tc.config, nil)
		if len(errs) > 0 && !tc.expectError {
			t.Errorf("Unexpected error for %#v: %v", tc.config, errs)
		}
		if len(errs) == 0 && tc.expectError {
			t.Errorf("Did not get expected error for: %#v", tc.config)
		}
	}
}

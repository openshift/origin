package test_report

import (
	"reflect"
	"testing"
)

// I hate regexes so much
func Test_featureGatesFromTestName(t *testing.T) {
	type args struct {
		testName string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "more than one",
			args: args{
				testName: `[sig-auth][FeatureGate:ServiceAccountTokenNodeBinding][OCPFeatureGate:Other][OCPFeatureGate:ValidatingAdmissionPolicy] per-node SA tokens can restrict access by-node [Suite:openshift/conformance/parallel]`,
			},
			want: []string{
				"Other",
				"ValidatingAdmissionPolicy",
				"ServiceAccountTokenNodeBinding",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := featureGatesFromTestName(tt.args.testName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("featureGatesFromTestName() = %v, want %v", got, tt.want)
			}
		})
	}
}

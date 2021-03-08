package monitor

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func Test_findOperatorVersionChange(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
		old  []configv1.OperandVersion
		new  []configv1.OperandVersion
		want []string
	}{
		{
			old: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "1.0.1"}},
			new: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "1.0.1"}},
		},
		{
			old:  []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "1.0.1"}},
			new:  []configv1.OperandVersion{{Name: "a", Version: "1.0.1"}, {Name: "b", Version: "1.0.1"}},
			want: []string{"a 1.0.0 -> 1.0.1"},
		},
		{
			old: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "1.0.1"}},
			new: []configv1.OperandVersion{{Name: "b", Version: "1.0.1"}, {Name: "a", Version: "1.0.0"}},
		},
		{
			old:  []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "1.0.1"}},
			new:  []configv1.OperandVersion{{Name: "b", Version: "1.0.1"}, {Name: "a", Version: "1.0.1"}},
			want: []string{"a 1.0.0 -> 1.0.1"},
		},
		{
			old:  []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}},
			new:  []configv1.OperandVersion{{Name: "a", Version: "1.0.1"}},
			want: []string{"a 1.0.0 -> 1.0.1"},
		},
		{
			old: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}},
			new: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}},
		},
		{
			old: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}},
			new: []configv1.OperandVersion{},
		},
		{
			old: []configv1.OperandVersion{},
			new: []configv1.OperandVersion{{Name: "a", Version: "1.0.0"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findOperatorVersionChange(tt.old, tt.new); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findOperatorVersionChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOperatorConditionStatus(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
		want1   bool
		want3   string
	}{
		{
			name:    "simple",
			message: "condition/Degraded status/True reason/DNSDegraded changed: DNS default is degraded",
			want:    "Degraded",
			want1:   true,
			want3:   "DNS default is degraded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got3 := GetOperatorConditionStatus(tt.message)
			if got != tt.want {
				t.Errorf("GetOperatorConditionStatus() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetOperatorConditionStatus() got1 = %v, want %v", got1, tt.want1)
			}
			if got3 != tt.want3 {
				t.Errorf("GetOperatorConditionStatus() got3 = %v, want %v", got3, tt.want3)
			}
		})
	}
}

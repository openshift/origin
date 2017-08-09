package interceptor

import (
	"testing"
)

func TestIsPushImageEndpoint(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "with dashes", args: args{path: "/v1.24/images/openshift-built-image-kpoe6a7h3p5md6vjlh3s32d5tm1qe4mb/push"}, want: true},
		{name: "with slashes", args: args{path: "/v1.24/images/openshift/built/image/kpoe6a7h3p5md6vjlh3s32d5tm1qe4mb/push"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPushImageEndpoint(tt.args.path); got != tt.want {
				t.Errorf("IsPushImageEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

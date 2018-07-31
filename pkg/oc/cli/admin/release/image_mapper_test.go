package release

import (
	"testing"
)

func TestNewImageMapper(t *testing.T) {
	type args struct {
		images map[string]ImageReference
	}
	tests := []struct {
		name    string
		args    args
		input   string
		output  string
		wantErr bool
	}{
		// TODO: Add test cases.
		{name: "empty input"},
		{
			name: "duplicate source repositories",
			args: args{
				images: map[string]ImageReference{
					"etcd": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:1234",
					},
					"etcd-2": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:5678",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "replace repository with tag",
			args: args{
				images: map[string]ImageReference{
					"etcd": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:1234",
					},
				},
			},
			input:  "image: quay.io/coreos/etcd:latest",
			output: "image: quay.io/openshift/origin-etcd@sha256:1234",
		},
		{
			name: "replace repository with digest",
			args: args{
				images: map[string]ImageReference{
					"etcd": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:1234",
					},
				},
			},
			input:  "image: quay.io/coreos/etcd@sha256:5678",
			output: "image: quay.io/openshift/origin-etcd@sha256:1234",
		},
		{
			name: "ignore bare repository - in the future we may fix this",
			args: args{
				images: map[string]ImageReference{
					"etcd": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:1234",
					},
				},
			},
			input:  "image: quay.io/coreos/etcd",
			output: "image: quay.io/coreos/etcd",
		},
		{
			name: "Ignore things that only look like images",
			args: args{
				images: map[string]ImageReference{
					"etcd": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:1234",
					},
				},
			},
			input:  "example_url: https://quay.io/coreos/etcd:8443/test",
			output: "example_url: https://quay.io/coreos/etcd:8443/test",
		},
		{
			name: "replace entire file - just to verify the regex",
			args: args{
				images: map[string]ImageReference{
					"etcd": {
						SourceRepository: "quay.io/coreos/etcd",
						TargetPullSpec:   "quay.io/openshift/origin-etcd@sha256:1234",
					},
				},
			},
			input:  "quay.io/coreos/etcd:latest",
			output: "quay.io/openshift/origin-etcd@sha256:1234",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewImageMapper(tt.args.images)
			if (err != nil) != tt.wantErr {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			out, err := m([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			if string(out) != tt.output {
				t.Errorf("unexpected output, wanted\n%s\ngot\n%s", tt.output, string(out))
			}
		})
	}
}

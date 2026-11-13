package main

import (
	"os"
	"testing"

	"github.com/containerd/nri/pkg/api"
)

func TestParseKeyValues(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []keyValue
		wantErr bool
	}{
		{name: "empty", value: "", wantErr: true},
		{name: "single", value: "A=1", want: []keyValue{{key: "A", value: "1"}}},
		{name: "multiple", value: "A=1,B=2", want: []keyValue{{key: "A", value: "1"}, {key: "B", value: "2"}}},
		{name: "value contains equals", value: "A=b=c", want: []keyValue{{key: "A", value: "b=c"}}},
		{name: "empty value", value: "A=", want: []keyValue{{key: "A", value: ""}}},
		{name: "missing equals", value: "A", wantErr: true},
		{name: "second entry missing equals", value: "A=1,B", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseKeyValues("test", tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestParseMounts(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []*api.Mount
		wantErr bool
	}{
		{
			name:  "read-write by default",
			value: "/host:/ctr",
			want:  []*api.Mount{{Source: "/host", Destination: "/ctr", Type: "bind", Options: []string{"bind", "rw"}}},
		},
		{
			name:  "read-only",
			value: "/host:/ctr:ro",
			want:  []*api.Mount{{Source: "/host", Destination: "/ctr", Type: "bind", Options: []string{"bind", "ro"}}},
		},
		{
			name:  "unknown option is read-write",
			value: "/host:/ctr:z",
			want:  []*api.Mount{{Source: "/host", Destination: "/ctr", Type: "bind", Options: []string{"bind", "rw"}}},
		},
		{
			name:  "multiple",
			value: "/a:/a:ro,/b:/b",
			want: []*api.Mount{
				{Source: "/a", Destination: "/a", Type: "bind", Options: []string{"bind", "ro"}},
				{Source: "/b", Destination: "/b", Type: "bind", Options: []string{"bind", "rw"}},
			},
		},
		{name: "missing destination", value: "/host", wantErr: true},
		{name: "too many fields", value: "/host:/ctr:ro:x", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMounts(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i].Source != tc.want[i].Source || got[i].Destination != tc.want[i].Destination ||
					got[i].Type != tc.want[i].Type || len(got[i].Options) != len(tc.want[i].Options) {
					t.Fatalf("got %+v, want %+v", got[i], tc.want[i])
				}
				for j := range got[i].Options {
					if got[i].Options[j] != tc.want[i].Options[j] {
						t.Fatalf("got options %v, want %v", got[i].Options, tc.want[i].Options)
					}
				}
			}
		})
	}
}

func TestParseDevices(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    *api.LinuxDevice
		wantErr bool
	}{
		{
			name:  "default mode",
			value: "/dev/nri-null:c:1:3",
			want:  &api.LinuxDevice{Path: "/dev/nri-null", Type: "c", Major: 1, Minor: 3, FileMode: api.FileMode(uint32(0o666))},
		},
		{
			name:  "explicit mode",
			value: "/dev/nri-null:c:1:3:664",
			want:  &api.LinuxDevice{Path: "/dev/nri-null", Type: "c", Major: 1, Minor: 3, FileMode: api.FileMode(uint32(0o664))},
		},
		{name: "missing minor", value: "/dev/nri-null:c:1", wantErr: true},
		{name: "invalid major", value: "/dev/nri-null:c:x:3", wantErr: true},
		{name: "invalid minor", value: "/dev/nri-null:c:1:x", wantErr: true},
		{name: "invalid mode", value: "/dev/nri-null:c:1:3:9z", wantErr: true},
		{name: "negative mode", value: "/dev/nri-null:c:1:3:-1", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDevices(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d devices, want 1", len(got))
			}
			if got[0].Path != tc.want.Path || got[0].Type != tc.want.Type ||
				got[0].Major != tc.want.Major || got[0].Minor != tc.want.Minor {
				t.Fatalf("got %+v, want %+v", got[0], tc.want)
			}
			gotMode, wantMode := got[0].FileMode.Get(), tc.want.FileMode.Get()
			if gotMode == nil || wantMode == nil || *gotMode != os.FileMode(*wantMode) {
				t.Fatalf("got mode %v, want %v", gotMode, wantMode)
			}
		})
	}
}

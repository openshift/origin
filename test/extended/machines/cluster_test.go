package operators

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func mustTime(str string) time.Time {
	ret, err := time.Parse(time.RFC3339, str)
	if err != nil {
		panic(err)
	}
	return ret
}

func Test_parseBootInstances(t *testing.T) {
	type args struct {
		listBootsOutput string
	}
	tests := []struct {
		name            string
		args            args
		want            []bootTimelineEntry
		wantDiagnostics []string
		wantErr         bool
	}{
		{
			name: "david's laptop",
			args: args{listBootsOutput: `IDX BOOT ID                          FIRST ENTRY                 LAST ENTRY                 
 -2 ac57799232d2499cbfac9c0e2e6d4d60 Wed 2024-03-13 10:20:26 EDT Sun 2024-04-07 23:27:26 EDT
 -1 a9d9a2901ab94a2f8ff8992565380105 Wed 2024-04-10 08:30:52 EDT Wed 2024-04-24 11:46:08 EDT
  0 b05245fa1b1c4c77a6c1b39f44f90acf Wed 2024-04-24 11:46:29 EDT Thu 2024-06-06 16:32:24 EDT
`},
			want: []bootTimelineEntry{
				{action: "Boot", time: mustTime("2024-03-13T10:20:26-04:00")},
				{action: "Boot", time: mustTime("2024-04-10T08:30:52-04:00")},
				{action: "Boot", time: mustTime("2024-04-24T11:46:29-04:00")},
			},
		},
		{
			name: "journal diagnostics do not hide valid boots",
			args: args{listBootsOutput: `journalctl: warning: skipped unreadable journal data
IDX BOOT ID                          FIRST ENTRY                 LAST ENTRY
 -1 a9d9a2901ab94a2f8ff8992565380105 Wed 2024-04-10 08:30:52 EDT Wed 2024-04-24 11:46:08 EDT
  0 b05245fa1b1c4c77a6c1b39f44f90acf Wed 2024-04-24 11:46:29 EDT Thu 2024-06-06 16:32:24 EDT
`},
			want: []bootTimelineEntry{
				{action: "Boot", time: mustTime("2024-04-10T08:30:52-04:00")},
				{action: "Boot", time: mustTime("2024-04-24T11:46:29-04:00")},
			},
			wantDiagnostics: []string{"journalctl: warning: skipped unreadable journal data"},
		},
		{
			name:    "empty output is not a valid boot timeline",
			args:    args{listBootsOutput: ""},
			wantErr: true,
		},
		{
			name:    "malformed boot record is rejected",
			args:    args{listBootsOutput: "IDX BOOT ID FIRST ENTRY LAST ENTRY\n0 invalid-id Wed 2024-04-24 11:46:29 EDT"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diagnostics, err := parseBootInstances(tt.args.listBootsOutput)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBootInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBootInstances() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(diagnostics, tt.wantDiagnostics) {
				t.Errorf("parseBootInstances() diagnostics = %v, want %v", diagnostics, tt.wantDiagnostics)
			}
		})
	}
}

func Test_bootTimelineEntryString(t *testing.T) {
	tests := []struct {
		name     string
		entry    bootTimelineEntry
		expected string
	}{
		{
			name:     "boot entry formats time as RFC3339",
			entry:    bootTimelineEntry{action: "Boot", time: mustTime("2026-01-20T22:20:50Z")},
			expected: "2026-01-20T22:20:50Z - Boot",
		},
		{
			name:     "reboot request entry formats time as RFC3339",
			entry:    bootTimelineEntry{action: "RebootRequest", time: mustTime("2024-03-13T10:20:01-04:00")},
			expected: "2024-03-13T10:20:01-04:00 - RebootRequest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func Test_bootTimelineSliceFormat(t *testing.T) {
	// Verify that formatting a []bootTimelineEntry with %v calls String() on each element,
	// producing human-readable timestamps instead of raw struct output.
	entries := []bootTimelineEntry{
		{action: "Boot", time: mustTime("2026-01-20T22:20:50Z")},
		{action: "RebootRequest", time: mustTime("2026-01-20T22:01:27Z")},
	}
	formatted := fmt.Sprintf("%v", entries)
	if formatted == fmt.Sprintf("%v", []struct {
		action string
		time   time.Time
	}{{action: "Boot"}, {action: "RebootRequest"}}) {
		t.Error("slice formatting is not using String() method")
	}
	expected := "[2026-01-20T22:20:50Z - Boot 2026-01-20T22:01:27Z - RebootRequest]"
	if formatted != expected {
		t.Errorf("formatted slice = %q, want %q", formatted, expected)
	}
}

func Test_parseRebootInstances(t *testing.T) {
	type args struct {
		rebootsOutput string
	}
	tests := []struct {
		name            string
		args            args
		want            []bootTimelineEntry
		wantDiagnostics []string
		wantErr         bool
	}{
		{
			name: "david's laptop",
			args: args{rebootsOutput: `2024-02-29T14:10:33-0500 fedora systemd-logind[21993]: System is rebooting.
2024-03-13T10:20:01-0400 fedora systemd-logind[1404]: System is rebooting.
2024-04-24T11:45:58-0400 fedora systemd-logind[1460]: System is rebooting.
`},
			want: []bootTimelineEntry{
				{action: "RebootRequest", time: mustTime("2024-02-29T14:10:33-05:00")},
				{action: "RebootRequest", time: mustTime("2024-03-13T10:20:01-04:00")},
				{action: "RebootRequest", time: mustTime("2024-04-24T11:45:58-04:00")},
			},
		},
		{
			name: "journal diagnostics do not hide valid reboot requests",
			args: args{rebootsOutput: `journalctl: warning: skipped unreadable journal data
2024-03-01T12:00:00-0500 journalctl: warning: skipped rotated journal data
2024-03-13T10:20:01-0400 fedora systemd-logind[1404]: System is rebooting.
2024-04-24T11:45:58-0400 fedora systemd-logind[1460]: System is rebooting.
`},
			want: []bootTimelineEntry{
				{action: "RebootRequest", time: mustTime("2024-03-13T10:20:01-04:00")},
				{action: "RebootRequest", time: mustTime("2024-04-24T11:45:58-04:00")},
			},
			wantDiagnostics: []string{
				"journalctl: warning: skipped unreadable journal data",
				"2024-03-01T12:00:00-0500 journalctl: warning: skipped rotated journal data",
			},
		},
		{
			name: "empty output means no reboot requests",
			args: args{rebootsOutput: ""},
			want: []bootTimelineEntry{},
		},
		{
			name:            "diagnostics without records are rejected",
			args:            args{rebootsOutput: "journalctl: warning: skipped unreadable journal data"},
			wantDiagnostics: []string{"journalctl: warning: skipped unreadable journal data"},
			wantErr:         true,
		},
		{
			name:    "malformed reboot record is rejected",
			args:    args{rebootsOutput: "not-a-time fedora systemd-logind[1404]: System is rebooting."},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diagnostics, err := parseRebootInstances(tt.args.rebootsOutput)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRebootInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRebootInstances() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(diagnostics, tt.wantDiagnostics) {
				t.Errorf("parseRebootInstances() diagnostics = %v, want %v", diagnostics, tt.wantDiagnostics)
			}
		})
	}
}

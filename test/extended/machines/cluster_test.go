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
 -2 ac57799232d2499cbfac9c0e2e6d4d60 Wed 2024-03-13 14:20:26 UTC Mon 2024-04-08 03:27:26 UTC
 -1 a9d9a2901ab94a2f8ff8992565380105 Wed 2024-04-10 12:30:52 UTC Wed 2024-04-24 15:46:08 UTC
  0 b05245fa1b1c4c77a6c1b39f44f90acf Wed 2024-04-24 15:46:29 UTC Thu 2024-06-06 20:32:24 UTC
`},
			want: []bootTimelineEntry{
				{action: "Boot", time: mustTime("2024-03-13T14:20:26Z")},
				{action: "Boot", time: mustTime("2024-04-10T12:30:52Z")},
				{action: "Boot", time: mustTime("2024-04-24T15:46:29Z")},
			},
		},
		{
			name: "diagnostics and invalid records are skipped",
			args: args{listBootsOutput: `Journal file /var/log/journal/example/system.journal is truncated, ignoring file.
Journal file /var/log/journal/example/system.journal uses an unsupported feature, ignoring file.
Use SYSTEMD_LOG_LEVEL=debug journalctl --file=/var/log/journal/example/system.journal to see the details.
IDX BOOT ID                          FIRST ENTRY                 LAST ENTRY
invalid a9d9a2901ab94a2f8ff8992565380105 Wed 2024-04-10 08:30:52 UTC Wed 2024-04-24 11:46:08 UTC
  0 b05245fa1b1c4c77a6c1b39f44f90acf Wed 2024-04-24 11:46:29 UTC Thu 2024-06-06 16:32:24 UTC
`},
			want: []bootTimelineEntry{
				{action: "Boot", time: mustTime("2024-04-24T11:46:29Z")},
			},
			wantDiagnostics: []string{
				"Journal file /var/log/journal/example/system.journal is truncated, ignoring file.",
				"Journal file /var/log/journal/example/system.journal uses an unsupported feature, ignoring file.",
				"Use SYSTEMD_LOG_LEVEL=debug journalctl --file=/var/log/journal/example/system.journal to see the details.",
				"invalid a9d9a2901ab94a2f8ff8992565380105 Wed 2024-04-10 08:30:52 UTC Wed 2024-04-24 11:46:08 UTC",
			},
		},
		{
			name: "no valid boot records fails",
			args: args{listBootsOutput: `Journal file /var/log/journal/example/system.journal corrupted, ignoring file.
`},
			wantDiagnostics: []string{"Journal file /var/log/journal/example/system.journal corrupted, ignoring file."},
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diagnostics, err := parseBootInstances(tt.args.listBootsOutput)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBootInstances() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_parseBootInstance(t *testing.T) {
	validFields := []string{"0", "a9d9a2901ab94a2f8ff8992565380105", "Wed", "2024-04-10", "08:30:52", "UTC", "Wed", "2024-04-24", "11:46:08", "UTC"}
	tests := []struct {
		name   string
		fields []string
		wantOK bool
	}{
		{
			name:   "valid boot record",
			fields: validFields,
			wantOK: true,
		},
		{
			name:   "invalid boot index",
			fields: append([]string{"invalid"}, validFields[1:]...),
		},
		{
			name:   "invalid boot ID",
			fields: append([]string{validFields[0], "not-a-boot-id"}, validFields[2:]...),
		},
		{
			name:   "invalid first timestamp",
			fields: append([]string{validFields[0], validFields[1], "Wed", "not-a-date", "08:30:52", "UTC"}, validFields[6:]...),
		},
		{
			name:   "invalid last timestamp",
			fields: []string{"0", "a9d9a2901ab94a2f8ff8992565380105", "Wed", "2024-04-10", "08:30:52", "UTC", "Wed", "not-a-date", "11:46:08", "UTC"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseBootInstance(tt.fields)
			if ok != tt.wantOK {
				t.Errorf("parseBootInstance() ok = %v, want %v", ok, tt.wantOK)
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
			name: "diagnostics and other logind actions are skipped",
			args: args{rebootsOutput: `Journal file /var/log/journal/example/system.journal corrupted, ignoring file.
2024-03-13T10:20:01-0400 fedora systemd-logind[1404]: System is rebooting with kexec.
2024-03-13T10:20:02-0400 fedora systemd-logind[1404]: System userspace is rebooting.
2024-04-24T11:45:58-0400 fedora systemd-logind[1460]: System is rebooting.
`},
			want: []bootTimelineEntry{
				{action: "RebootRequest", time: mustTime("2024-04-24T11:45:58-04:00")},
			},
			wantDiagnostics: []string{
				"Journal file /var/log/journal/example/system.journal corrupted, ignoring file.",
				"2024-03-13T10:20:01-0400 fedora systemd-logind[1404]: System is rebooting with kexec.",
				"2024-03-13T10:20:02-0400 fedora systemd-logind[1404]: System userspace is rebooting.",
			},
		},
		{
			name: "empty output means no reboot requests",
			args: args{rebootsOutput: ""},
			want: []bootTimelineEntry{},
		},
		{
			name: "diagnostics without reboot requests succeed",
			args: args{rebootsOutput: `An error was encountered while opening journal file or directory /var/log/journal, ignoring file: Input/output error
`},
			want: []bootTimelineEntry{},
			wantDiagnostics: []string{
				"An error was encountered while opening journal file or directory /var/log/journal, ignoring file: Input/output error",
			},
		},
		{
			name: "invalid reboot records are skipped",
			args: args{rebootsOutput: `not-a-time fedora systemd-logind[1404]: System is rebooting.
2024-03-13T10:20:01-0400 fedora journalctl[1404]: System is rebooting.
`},
			want: []bootTimelineEntry{},
			wantDiagnostics: []string{
				"not-a-time fedora systemd-logind[1404]: System is rebooting.",
				"2024-03-13T10:20:01-0400 fedora journalctl[1404]: System is rebooting.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diagnostics := parseRebootInstances(tt.args.rebootsOutput)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRebootInstances() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(diagnostics, tt.wantDiagnostics) {
				t.Errorf("parseRebootInstances() diagnostics = %v, want %v", diagnostics, tt.wantDiagnostics)
			}
		})
	}
}

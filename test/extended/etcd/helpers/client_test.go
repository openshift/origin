package helpers

import "testing"

func TestParsePortForwardEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name:   "IPv4 loopback",
			output: "Forwarding from 127.0.0.1:34567 -> 2379",
			want:   "127.0.0.1:34567",
		},
		{
			name:   "IPv6 loopback",
			output: "Forwarding from [::1]:34567 -> 2379",
			want:   "[::1]:34567",
		},
		{
			name:    "empty output",
			wantErr: true,
		},
		{
			name:    "unexpected forwarding prefix",
			output:  "Proxying to 127.0.0.1:34567 -> 2379",
			wantErr: true,
		},
		{
			name:    "wrong destination port",
			output:  "Forwarding from 127.0.0.1:34567 -> 2380",
			wantErr: true,
		},
		{
			name:    "missing local port",
			output:  "Forwarding from internal-host.example -> 2379",
			wantErr: true,
		},
		{
			name:    "missing host",
			output:  "Forwarding from :34567 -> 2379",
			wantErr: true,
		},
		{
			name:    "IPv6 missing brackets",
			output:  "Forwarding from ::1:34567 -> 2379",
			wantErr: true,
		},
		{
			name:    "IPv4 invalid local port",
			output:  "Forwarding from 127.0.0.1:not-a-port -> 2379",
			wantErr: true,
		},
		{
			name:    "IPv6 invalid local port",
			output:  "Forwarding from [::1]:not-a-port -> 2379",
			wantErr: true,
		},
		{
			name:    "empty local port",
			output:  "Forwarding from [::1]: -> 2379",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortForwardEndpoint(tt.output)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePortForwardEndpoint() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err != nil && err.Error() != "port forward output not in expected format" {
				t.Errorf("port-forward error includes raw output: %v", err)
			}
			if got != tt.want {
				t.Errorf("parsePortForwardEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

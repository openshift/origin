package helpers

import "testing"

// TestParsePortForwardEndpoint covers IPv4, IPv6, and malformed port-forward output.
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
			name:    "unexpected forwarding output",
			output:  "Proxying to 127.0.0.1:34567 -> 2379",
			wantErr: true,
		},
		{
			name:    "invalid port",
			output:  "Forwarding from [::1]:not-a-port -> 2379",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortForwardEndpoint(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePortForwardEndpoint() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parsePortForwardEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

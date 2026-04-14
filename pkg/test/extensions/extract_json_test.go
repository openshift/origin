package extensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "clean JSON object",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "clean JSON array",
			input: `[{"index": 1}]`,
			want:  `[{"index": 1}]`,
		},
		{
			name:  "warning lines before JSON object",
			input: "W0414 08:58:39.856273 46367 controller.go:47] some warning\nW0414 08:58:39.865859 46367 feature_gate.go:352] another warning\n{\"key\": \"value\"}\n",
			want:  `{"key": "value"}`,
		},
		{
			name:  "warning lines before JSON array",
			input: "W0414 08:58:39.856273 46367 controller.go:47] some warning\n[{\"index\": 1}]\n",
			want:  `[{"index": 1}]`,
		},
		{
			name:  "log lines after JSON are ignored",
			input: "{\"key\": \"value\"}\nW0414 trailing log with } brace\n",
			want:  `{"key": "value"}`,
		},
		{
			name:    "no JSON in output",
			input:   "W0414 just warnings\nI0414 and info lines\n",
			wantErr: true,
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: true,
		},
		{
			name:    "null is not a JSON object or array",
			input:   "W0414 warning\nnull\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSON([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(got))
		})
	}
}

package release

import (
	"bytes"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func Test_contentStream_Read(t *testing.T) {
	tests := []struct {
		name    string
		parts   [][]byte
		want    string
		wantN   int64
		wantErr bool
	}{
		{
			parts: [][]byte{[]byte("test"), []byte("other"), []byte("a")},
			want:  "testothera",
			wantN: 10,
		},
		{
			parts: [][]byte{[]byte("test"), []byte(strings.Repeat("a", 4096))},
			want:  "test" + strings.Repeat("a", 4096),
			wantN: 4100,
		},
		{
			parts: nil,
			want:  "",
			wantN: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			s := &contentStream{
				parts: tt.parts,
			}
			gotN, err := io.Copy(buf, s)
			if (err != nil) != tt.wantErr {
				t.Errorf("contentStream.Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("expected %d but got %d", tt.wantN, gotN)
			}
			if !bytes.Equal([]byte(tt.want), buf.Bytes()) {
				t.Errorf("contentStream.Read():\n%s\n%s", hex.Dump(buf.Bytes()), hex.Dump([]byte(tt.want)))
			}
		})
	}
}

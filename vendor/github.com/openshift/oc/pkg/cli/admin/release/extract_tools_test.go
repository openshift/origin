package release

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"strings"
	"testing"
)

func Test_copyAndReplaceReleaseImage(t *testing.T) {
	baseLen := len(installerReplacement)
	tests := []struct {
		name         string
		r            *bytes.Buffer
		buffer       int
		releaseImage string
		wantIndex    int
		wantErr      bool
	}{
		{buffer: 10, wantErr: true, wantIndex: -1},
		{buffer: baseLen, wantErr: false, wantIndex: -1},

		{releaseImage: "test:latest", r: fakeInput(1024, 0), wantIndex: 1024, name: "end of file"},
		{releaseImage: "test:latest", r: fakeInput(2*1024, 0), wantIndex: 2 * 1024},

		{releaseImage: "test:latest", r: fakeInput(1024-1, 0, 1), wantIndex: 1024 - 1},
		{releaseImage: "test:latest", r: fakeInput(0, 1), wantIndex: 0},

		{releaseImage: "test:latest", r: fakeInput(baseLen, 0), wantIndex: baseLen},
		{releaseImage: "test:latest", r: fakeInput(baseLen*2, 0), wantIndex: baseLen * 2},
		{releaseImage: "test:latest", r: fakeInput(baseLen-1, 0), wantIndex: baseLen - 1},
		{releaseImage: "test:latest", r: fakeInput(baseLen*2-1, 0), wantIndex: baseLen*2 - 1},
		{releaseImage: "test:latest", r: fakeInput(baseLen+1, 0), wantIndex: baseLen + 1},
		{releaseImage: "test:latest", r: fakeInput(baseLen*2+1, 0), wantIndex: baseLen*2 + 1},

		{releaseImage: strings.Repeat("a", baseLen), wantIndex: -1, wantErr: true},
		{releaseImage: strings.Repeat("a", baseLen+1), wantIndex: -1, wantErr: true},

		{releaseImage: strings.Repeat("a", baseLen-1), r: fakeInput(baseLen, 0), wantIndex: baseLen},
		{releaseImage: strings.Repeat("a", baseLen-2), r: fakeInput(baseLen, 0), wantIndex: baseLen},
		{releaseImage: strings.Repeat("a", baseLen-1), r: fakeInput(1, baseLen, 0), wantIndex: 1 + baseLen},
		{releaseImage: strings.Repeat("a", baseLen-2), r: fakeInput(1, 0, baseLen), wantIndex: 1},

		{releaseImage: strings.Repeat("a", baseLen-1), r: fakeInput(baseLen*2, 0), wantIndex: baseLen * 2},
		{releaseImage: strings.Repeat("a", baseLen-2), r: fakeInput(baseLen*2, 0), wantIndex: baseLen * 2},
		{releaseImage: strings.Repeat("a", baseLen-1), r: fakeInput(1, baseLen*2, 0), wantIndex: 1 + baseLen*2},
		{releaseImage: strings.Repeat("a", baseLen-2), r: fakeInput(1, 0, baseLen*2), wantIndex: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			if tt.buffer == 0 {
				tt.buffer = 1024
			}
			if tt.r == nil {
				tt.r = &bytes.Buffer{}
			}

			src := tt.r.Bytes()
			original := make([]byte, len(src))
			copy(original, src)

			got, err := copyAndReplaceReleaseImage(w, tt.r, tt.buffer, tt.releaseImage)
			if (err != nil) != tt.wantErr {
				t.Fatalf("copyAndReplaceReleaseImage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != (tt.wantIndex != -1) {
				t.Fatalf("copyAndReplaceReleaseImage() = %v, want %v", got, tt.wantIndex != -1)
			}
			if got {
				if len(w.Bytes()) != len(original) {
					t.Fatalf("mismatched lengths: %d vs %d \n%s\n%s", len(original), w.Len(), hex.Dump(original), hex.Dump(w.Bytes()))
				}
				index := bytes.Index(w.Bytes(), []byte(tt.releaseImage+"\x00"))
				if index != tt.wantIndex {
					t.Errorf("expected index %d, got index %d\n%s", tt.wantIndex, index, hex.Dump(w.Bytes()))
				}
			} else {
				if !bytes.Equal(w.Bytes(), original) {
					t.Fatalf("unexpected response body:\n%s\n%s", hex.Dump(original), hex.Dump(w.Bytes()))
				}
			}
		})
	}
}

func fakeInput(lengths ...int) *bytes.Buffer {
	buf := &bytes.Buffer{}
	for _, l := range lengths {
		if l == 0 {
			buf.WriteString(installerReplacement)
		} else {
			b := byte(rand.Intn(256))
			buf.Write(bytes.Repeat([]byte{b}, l))
		}
	}
	return buf
}

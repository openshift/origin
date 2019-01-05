package recordio

import (
	"bytes"
	"io"
	"testing"
)

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }

func TestWriter(t *testing.T) {
	for i, tc := range []struct {
		frame  []byte
		output []byte
		err    error
	}{
		{nil, ([]byte)("0\n"), nil},
		{([]byte)("a"), ([]byte)("1\na"), nil},
		{([]byte)("abc\n"), ([]byte)("4\nabc\n"), nil},
	} {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		err := w.WriteFrame(tc.frame)

		if err != tc.err {
			t.Fatalf("test case %d failed: expected error %v instead of %v", i, tc.err, err)
		}
		if b := buf.Bytes(); bytes.Compare(b, tc.output) != 0 {
			t.Fatalf("test case %d failed: expected bytes %q instead of %q", i, string(b), string(tc.output))
		}
	}
	for i := 0; i < 5; i++ {
		w := NewWriter(writerFunc(func(_ []byte) (int, error) { return i, nil }))
		err := w.WriteFrame(([]byte)("james"))
		if err != io.ErrShortWrite {
			t.Fatalf("expected short write error with i=%d instead of %v", i, err)
		}
	}
}

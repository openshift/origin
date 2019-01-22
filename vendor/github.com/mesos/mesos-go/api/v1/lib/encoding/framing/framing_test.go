package framing_test

import (
	"bytes"
	"io"
	"testing"

	. "github.com/mesos/mesos-go/api/v1/lib/encoding/framing"
)

func TestError(t *testing.T) {
	a := Error("a")
	if "a" != string(a) {
		t.Errorf("identity/sanity check failed")
	}
	if "a" != a.Error() {
		t.Errorf("expected 'a' instead of %q", a.Error())
	}
}

func TestReadAll(t *testing.T) {
	r := ReadAll(bytes.NewBufferString(""))
	buf, err := r.ReadFrame()
	if len(buf) != 0 {
		t.Errorf("expected zero length frame instead of %+v", buf)
	}
	if err != io.EOF {
		t.Errorf("expected EOF instead of %+v", err)
	}

	r = ReadAll(bytes.NewBufferString("foo"))
	buf, err = r.ReadFrame()
	if err != nil {
		t.Fatalf("unexpected error %+v", err)
	}
	if string(buf) != "foo" {
		t.Errorf("expected 'foo' instead of %q", string(buf))
	}

	// read again, now that there's no more data
	buf, err = r.ReadFrame()
	if len(buf) != 0 {
		t.Errorf("expected zero length frame instead of %+v", buf)
	}
	if err != io.EOF {
		t.Errorf("expected EOF instead of %+v", err)
	}
}

func TestWriterFor(t *testing.T) {
	buf := new(bytes.Buffer)
	w := WriterFor(buf)
	err := w.WriteFrame(([]byte)("foo"))
	if err != nil {
		t.Fatalf("failed to write frame: +%v", err)
	}
	if buf.String() != "foo" {
		t.Fatalf("expected 'foo' instead of %q", buf.String())
	}

	err = w.WriteFrame(([]byte)(""))
	if err != nil {
		t.Fatalf("failed to write empty frame: +%v", err)
	}
	if buf.String() != "foo" {
		t.Fatalf("expected 'foo' instead of %q", buf.String())
	}

	w = WriterFor(&shortWriter{w: buf, n: 1})
	err = w.WriteFrame(([]byte)(""))
	if err != nil {
		t.Fatalf("failed to write empty frame: +%v", err)
	}
	if buf.String() != "foo" {
		t.Fatalf("expected 'foo' instead of %q", buf.String())
	}

	err = w.WriteFrame(([]byte)("bar"))
	if err != io.ErrShortWrite {
		t.Fatalf("failed to detect short write: +%v", err)
	}
	if buf.String() != "foob" {
		t.Fatalf("expected 'foob' instead of %q", buf.String())
	}
}

type shortWriter struct {
	w io.Writer
	n int
}

func (s *shortWriter) Write(b []byte) (n int, err error) {
	if s.n <= 0 {
		return 0, nil
	}
	n = len(b)
	if n > s.n {
		n = s.n
	}
	n, err = s.w.Write(b[0:n])
	s.n -= n
	return
}

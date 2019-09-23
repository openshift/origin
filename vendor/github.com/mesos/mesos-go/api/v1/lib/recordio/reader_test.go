package recordio_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/mesos/mesos-go/api/v1/lib/encoding/framing"
	"github.com/mesos/mesos-go/api/v1/lib/recordio"
)

func Example() {
	var (
		r     = recordio.NewReader(strings.NewReader("6\nhello 0\n6\nworld!"))
		lines []string
	)
	for {
		fr, err := r.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		lines = append(lines, string(fr))
	}
	fmt.Println(lines)
	// Output:
	// [hello  world!]
}

func TestReadFrame(t *testing.T) {
	list := func(v ...string) []string { return v }
	variants := []struct {
		name     string
		decorate func(io.Reader) io.Reader
	}{
		{"identity", func(r io.Reader) io.Reader { return r }},
		{"one-byte", iotest.OneByteReader},
		{"half", iotest.HalfReader},
	}
	for ti, tc := range []struct {
		in     string
		frames []string
		err    error
	}{
		/*  0 */ {"", nil, nil},
		/*  1 */ {"a", nil, framing.ErrorUnderrun},
		/*  2 */ {"aaaaaaaaaaaaaaaaaaaaa", nil, framing.ErrorBadSize}, // 21 digits is too large for frame size
		/*  3 */ {"111111111111111111111", nil, framing.ErrorBadSize},
		/*  4 */ {"a\n", nil, framing.ErrorBadSize},
		/*  5 */ {"0\n", nil, nil},
		/*  6 */ {"00000000000000000000\n", nil, nil},
		/*  7 */ {"000000000000000000000\n", nil, framing.ErrorBadSize},
		/*  8 */ {"0\n0\n0\n", nil, nil},
		/*  9 */ {"1\n", nil, framing.ErrorUnderrun},
		/* 10 */ {"1\na", list("a"), nil},
		/* 11 */ {"1\na0\n1\nb", list("a", "b"), nil},
		/* 12 */ {"2\na", nil, framing.ErrorUnderrun},
		/* 13 */ {"1\na1\nb1\nc", list("a", "b", "c"), nil},
		/* 14 */ {"5\nabcde", list("abcde"), nil},
		/* 15 */ {"5\nabcde3\nfgh", list("abcde", "fgh"), nil},
		/* 16 */ {"5\nabcde5\nfgh", list("abcde"), framing.ErrorUnderrun},
		/* 17 */ {"23\n", nil, framing.ErrorOversizedFrame}, // 23 exceeds max of 22
	} {
		for _, v := range variants {
			t.Run(fmt.Sprintf("test case %d %s", ti, v.name), func(t *testing.T) {
				var (
					r = recordio.NewReader(v.decorate(
						strings.NewReader(tc.in)), recordio.MaxMessageSize(22))
					frames  []string
					lastErr error
				)
				for lastErr == nil {
					fr, err := r.ReadFrame()
					if err == nil {
						t.Log("read frame " + string(fr))
						frames = append(frames, string(fr))
					}
					lastErr = err
				}
				if tc.err == nil && lastErr != io.EOF {
					t.Fatalf("unexpected error %q", lastErr)
				}
				if tc.err != nil && lastErr != tc.err {
					t.Fatalf("expected error %q instead of error %q", tc.err, lastErr)
				}
				if !reflect.DeepEqual(tc.frames, frames) {
					t.Fatalf("expected frames %#v instead of frames %#v", tc.frames, frames)
				}
			})
		}
	}
}

func BenchmarkReader(b *testing.B) {
	var buf bytes.Buffer
	genRecords(b, &buf)

	r := recordio.NewReader(&buf)

	b.StopTimer()
	b.ResetTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		if tok, err := r.ReadFrame(); err != nil && err != io.EOF {
			b.Fatal(err)
		} else {
			b.SetBytes(int64(len(tok)))
		}
	}
}

func genRecords(tb testing.TB, w io.Writer) {
	rnd := rng{rand.New(rand.NewSource(0xdeadbeef))}
	buf := make([]byte, 2<<12)
	for i := 0; i < cap(buf); i++ {
		sz := rnd.Intn(cap(buf))
		n, err := rnd.Read(buf[:sz])
		if err != nil {
			tb.Fatal(err)
		}
		header := strconv.FormatInt(int64(n), 10) + "\n"
		if _, err = io.WriteString(w, header); err != nil {
			tb.Fatal(err)
		} else if _, err = w.Write(buf[:n]); err != nil {
			tb.Fatal(err)
		}
	}
}

type rng struct{ *rand.Rand }

func (r rng) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i += 7 {
		val := r.Int63()
		for j := 0; i+j < len(p) && j < 7; j++ {
			p[i+j] = byte(val)
			val >>= 8
		}
	}
	return len(p), nil
}

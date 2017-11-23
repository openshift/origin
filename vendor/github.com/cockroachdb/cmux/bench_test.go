package cmux

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
)

type mockConn struct {
	net.Conn
	r io.Reader
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

func BenchmarkCMuxConn(b *testing.B) {
	benchHTTPPayload := make([]byte, 4096)
	copy(benchHTTPPayload, []byte("GET http://www.w3.org/ HTTP/1.1"))

	m := New(nil).(*cMux)
	l := m.Match(HTTP1Fast())

	go func() {
		for {
			if _, err := l.Accept(); err != nil {
				return
			}
		}
	}()

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := &mockConn{
			r: bytes.NewReader(benchHTTPPayload),
		}
		m.serve(c, donec, &wg)
	}
}

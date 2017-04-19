package cmux

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"

	"golang.org/x/net/http2"
)

var (
	benchHTTP1Payload = make([]byte, 4096)
	benchHTTP2Payload = make([]byte, 4096)
)

func init() {
	copy(benchHTTP1Payload, []byte("GET http://www.w3.org/ HTTP/1.1"))
	copy(benchHTTP2Payload, http2.ClientPreface)
}

type mockConn struct {
	net.Conn
	r io.Reader
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

func discard(l net.Listener) {
	for {
		if _, err := l.Accept(); err != nil {
			return
		}
	}
}

func BenchmarkCMuxConnHTTP1(b *testing.B) {
	m := New(nil).(*cMux)
	l := m.Match(HTTP1Fast())

	go discard(l)

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := &mockConn{
			r: bytes.NewReader(benchHTTP1Payload),
		}
		m.serve(c, donec, &wg)
	}
}

func BenchmarkCMuxConnHTTP2(b *testing.B) {
	m := New(nil).(*cMux)
	l := m.Match(HTTP2())
	go discard(l)

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := &mockConn{
			r: bytes.NewReader(benchHTTP2Payload),
		}
		m.serve(c, donec, &wg)
	}
}

func BenchmarkCMuxConnHTTP1n2(b *testing.B) {
	m := New(nil).(*cMux)
	l1 := m.Match(HTTP1Fast())
	l2 := m.Match(HTTP2())

	go discard(l1)
	go discard(l2)

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := &mockConn{
			r: bytes.NewReader(benchHTTP2Payload),
		}
		m.serve(c, donec, &wg)
	}
}

func BenchmarkCMuxConnHTTP2n1(b *testing.B) {
	m := New(nil).(*cMux)
	l2 := m.Match(HTTP2())
	l1 := m.Match(HTTP1Fast())

	go discard(l1)
	go discard(l2)

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := &mockConn{
			r: bytes.NewReader(benchHTTP1Payload),
		}
		m.serve(c, donec, &wg)
	}
}

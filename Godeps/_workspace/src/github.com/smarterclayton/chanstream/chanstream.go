// Copyright 2014 Garrett D'Amore
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package chanstream provides an API that is similar to that used for TCP
// and Unix Domain sockets (see net.TCP), for use in intra-process
// communication on top of Go channels.  This makes it easy to swap it for
// another net.Conn interface.
//
// By using channels, we avoid exposing any
// interface to other processors, or involving the kernel to perform data
// copying.
package chanstream

import "net"
import "sync"
import "time"
import "io"

// ChanError implements the error and net.Error interfaces.
type ChanError struct {
	err string
	tmo bool
	tmp bool
}

// Error implements the error interface.
func (e *ChanError) Error() string {
	return e.err
}

// Timeout returns true if the error was a time out.
func (e *ChanError) Timeout() bool {
	return e.tmo
}

// Temporary returns true if the error was temporary in nature.  Operations
// resulting in temporary errors might be expected to succeed at a later time.
func (e *ChanError) Temporary() bool {
	return e.tmp
}

var (
	// ErrConnRefused is reported when no listener is present and
	// a client attempts to connect via Dial.
	ErrConnRefused = &ChanError{err: "Connection refused."}

	// ErrAddrInUse is reported when a server tries to Listen but another
	// Conn is already listening on the same address.
	ErrAddrInUse = &ChanError{err: "Address in use."}

	// ErrAcceptTimeout is reported when a request to Accept takes too
	// long.  (Note that this is not normally reported -- the default
	// is for no timeout to be used in Accept.)
	ErrAcceptTimeout = &ChanError{err: "Accept timeout.", tmo: true}

	// ErrListenQFull is reported if the listen backlog (default 32)
	// is exhausted.  This normally occurs if a server goroutine does
	// not call Accept often enough.
	ErrListenQFull = &ChanError{err: "Listen queue full.", tmp: true}

	// ErrConnClosed is reported when a peer closes the connection while
	// trying to establish the connection or send data.
	ErrConnClosed = &ChanError{err: "Connection closed."}

	// ErrConnTimeout is reported when a connection takes too long to
	// be established.
	ErrConnTimeout = &ChanError{err: "Connection timeout.", tmo: true}

	// ErrRdTimeout is reported when the read deadline on a connection
	// expires whle trying to read.
	ErrRdTimeout = &ChanError{err: "Read timeout.", tmo: true, tmp: true}

	// ErrWrTimeout is reported when the write deadline on a connection
	// expires whle trying to write.
	ErrWrTimeout = &ChanError{err: "Write timeout.", tmo: true, tmp: true}
)

// listeners acts as a registry of listeners.
var listeners struct {
	mtx sync.Mutex
	lst map[string]*ChanListener
}

// ChanAddr stores just the address, which will normally be something
// like a path, but any valid string can be used as a key.  This implements
// the net.Addr interface.
type ChanAddr struct {
	name string
}

// String returns the name of the end point -- the listen address.  This
// is just an arbitrary string used as a lookup key.
func (a *ChanAddr) String() string {
	return a.name
}

// Network returns "chan".
func (a *ChanAddr) Network() string {
	return "chan"
}

// ChanConn represents a logical connection between two peers communication
// using a pair of cross-connected go channels. This provides net.Conn
// semantics on top of channels.
type ChanConn struct {
	fifo      chan []byte
	fin       chan bool
	rdeadline time.Time
	wdeadline time.Time
	peer      *ChanConn
	pending   []byte
	closed    bool
	addr      *ChanAddr
}

type chanConnect struct {
	conn      *ChanConn
	connected chan bool
}

// ChanListener is used to listen to a socket.
type ChanListener struct {
	name     string
	connect  chan *chanConnect
	deadline time.Time
}

// ListenChan establishes the server address and receiving
// channel where clients can connect.  This service address is backed
// by a go channel.
func ListenChan(name string) (*ChanListener, error) {
	listeners.mtx.Lock()
	defer listeners.mtx.Unlock()

	if listeners.lst == nil {
		listeners.lst = make(map[string]*ChanListener)
	}
	if _, ok := listeners.lst[name]; ok {
		return nil, ErrAddrInUse
	}

	listener := new(ChanListener)
	listener.name = name
	// The listen backlog we support.. fairly arbitrary
	listener.connect = make(chan *chanConnect, 32)
	// Register listener on the service point
	listeners.lst[name] = listener
	return listener, nil
}

// AcceptChan accepts a client's connection request via Dial,
// and returns the associated underlying connection.
func (listener *ChanListener) AcceptChan() (*ChanConn, error) {

	deadline := mkTimer(listener.deadline)

	select {
	case connect := <-listener.connect:
		// Make a pair of channels, and twist them.  We keep
		// the first pair, client gets the twisted pair.
		// We support buffering up to 10 messages for efficiency
		chan1 := make(chan []byte, 10)
		chan2 := make(chan []byte, 10)
		fin1 := make(chan bool)
		fin2 := make(chan bool)
		addr := &ChanAddr{name: listener.name}
		server := &ChanConn{fifo: chan1, fin: fin1, addr: addr}
		client := &ChanConn{fifo: chan2, fin: fin2, addr: addr}
		server.peer = client
		client.peer = server
		// And send the client its info, and a wakeup
		connect.conn = client
		connect.connected <- true
		return server, nil

	case <-deadline:
		// NB: its never possible to read from a nil channel.
		// So this only counts if we have a timer running.
		return nil, ErrAcceptTimeout
	}
}

// Accept is a generic way to accept a connection.
func (listener *ChanListener) Accept() (net.Conn, error) {
	c, err := listener.AcceptChan()
	return c, err
}

// DialChan is the client side, think connect().
func DialChan(name string) (*ChanConn, error) {
	var listener *ChanListener
	listeners.mtx.Lock()
	if listeners.lst != nil {
		listener = listeners.lst[name]
	}
	listeners.mtx.Unlock()
	if listener == nil {
		return nil, ErrConnRefused
	}

	// TBD: This deadline is rather arbitrary
	deadline := time.After(time.Second * 10)
	creq := &chanConnect{conn: nil}
	creq.connected = make(chan bool)

	// Note: We assume the buffering is sufficient.  If the server
	// side cannot keep up with connect requests, then we'll fail.  The
	// connect is "non-blocking" in this regard.  As there is a reasonable
	// listen backlog, this should only happen if lots of clients try to
	// connect too fast.  In TCP world if this happens it becomes
	// ECONNREFUSED.  We use ErrListenQFull.
	select {
	case listener.connect <- creq:

	default:
		return nil, ErrListenQFull
	}

	select {
	case _, ok := <-creq.connected:
		if !ok {
			return nil, ErrConnClosed
		}

	case <-deadline:
		return nil, ErrConnTimeout
	}

	return creq.conn, nil
}

// Close implements the io.Closer interface.  It closes the channel for
// communications.  Messages that have already been sent may be received
// by the peer before the peer closes its side of the connection.  A
// notification is sent to the peer so it will close its side as well.
func (conn *ChanConn) Close() error {
	conn.CloseRead()
	conn.CloseWrite()
	return nil
}

// CloseRead closes the read side of the connection.  Addtionally, a
// notification is sent to the peer, to begin an orderly shutdown of the
// connection.  No further data may be read from the connection.
func (conn *ChanConn) CloseRead() error {
	close(conn.fin)
	conn.closed = true
	return nil
}

// CloseWrite closes the write side of the channel.  After this point, it
// is illegal to write data on the connection.
func (conn *ChanConn) CloseWrite() error {
	close(conn.fifo)
	return nil
}

// LocalAddr returns the local address.  For now, both client and server
// use the same address, which is the key used for Listen or Dial.
func (conn *ChanConn) LocalAddr() net.Addr {
	return conn.addr
}

// RemoteAddr returns the peer's address.  For now, both client and server
// use the same address, which is the key used for Listen or Dial.
func (conn *ChanConn) RemoteAddr() net.Addr {
	return conn.peer.addr
}

// SetDeadline sets the timeout for both read and write.
func (conn *ChanConn) SetDeadline(t time.Time) error {
	conn.rdeadline = t
	conn.wdeadline = t
	return nil
}

// SetReadDeadline sets the timeout for read (receive).
func (conn *ChanConn) SetReadDeadline(t time.Time) error {
	conn.rdeadline = t
	return nil
}

// SetWriteDeadline sets the timeout for write (send).
func (conn *ChanConn) SetWriteDeadline(t time.Time) error {
	conn.wdeadline = t
	return nil
}

// Read implements the io.Reader interface.
func (conn *ChanConn) Read(b []byte) (int, error) {
	b = b[0:0] // empty slice
	for len(b) < cap(b) {

		// get a byte slice from our peer if we don't have one yet
		if conn.pending == nil || len(conn.pending) == 0 {
			if len(b) > 0 {
				return len(b), nil
			}
			timer := mkTimer(conn.rdeadline)
			select {
			case msg := <-conn.peer.fifo:
				if msg != nil {
					conn.pending = msg
				} else if len(b) > 0 {
					return len(b), nil
				} else {
					return 0, io.EOF
				}

			case <-timer:
				// Timeout
				return len(b), ErrRdTimeout
			}
		}

		if conn.closed {
			return len(b), io.EOF
		}
		want := cap(b) - len(b)
		if want > len(conn.pending) {
			want = len(conn.pending)
		}
		b = append(b, conn.pending[:want]...)
		conn.pending = conn.pending[want:]
	}
	return len(b), nil
}

// Write implements the io.Writer interface.
func (conn *ChanConn) Write(b []byte) (int, error) {
	// Unlike Read, Write is quite a bit simpler, since
	// we don't have to deal with buffers.  We just write to the
	// channel/fifo.  We do have to respect when the peer has notified
	// us that its side is closed, however.

	// We have to make a copy to ensure that once a message is sent to
	// us, we are insulated against later modification by the sender.
	// Later we should consider limiting the size of this array to
	// prevent someone from trying to send ridiculous message sizes all
	// at once.  (E.g. avoid trying to alloc and copy 100 megabytes here!)
	a := make([]byte, len(b))
	copy(a, b)
	b = a

	deadline := mkTimer(conn.wdeadline)
	n := len(b)

	select {
	case <-conn.peer.fin:
		// Remote close
		return n, ErrConnClosed

	case conn.fifo <- b:
		// Sent it
		return n, nil

	case <-deadline:
		// Timeout
		return n, ErrWrTimeout
	}
}

// ReaderFrom, WriterTo interfaces can give some better performance,
// but we skip that for now, they're optional interfaces
// TO Add  Read, Write, (CloseRead, CloseWrite)
// ReadFrom, WriteTo,
func mkTimer(deadline time.Time) <-chan time.Time {

	if deadline.IsZero() {
		return nil
	}

	dur := deadline.Sub(time.Now())
	if dur < 0 {
		// a closed channel never blocks
		tm := make(chan time.Time)
		close(tm)
		return tm
	}

	return time.After(dur)
}

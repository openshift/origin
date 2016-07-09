package loopback

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"
	"github.com/smarterclayton/chanstream"
)

// DialFunc returns a net.Conn for the provided network or address.
type DialFunc func(network, address string) (net.Conn, error)

// Dialer returns a DialFunc that always tries to connect via the provided
// loopback device first, then invokes the provided DialFunc if a connection
// cannot be established.
func Dialer(fn DialFunc) DialFunc {
	return func(network, address string) (net.Conn, error) {
		if conn, err := chanstream.DialChan(address); err == nil {
			return conn, nil
		} else {
			glog.V(4).Infof("Unable to connect to %s over loopback: %v", address, err)
		}
		return fn(network, address)
	}
}

// NewListener creates a loopback listener on the provided loopback addresses (host:port
// or other net.Addr).
func NewListener(listener net.Listener, loopback ...string) (net.Listener, error) {
	if len(loopback) == 0 {
		return listener, nil
	}
	listeners := make([]net.Listener, 0, len(loopback)+1)
	listeners = append(listeners, listener)
	for _, addr := range loopback {
		ln, err := chanstream.ListenChan(addr)
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, &listenerWithAddr{ln, listener.Addr()})
	}
	return NewMultiListener(listeners...), nil
}

type listenerWithAddr struct {
	*chanstream.ChanListener
	Address net.Addr
}

func (l *listenerWithAddr) Addr() net.Addr {
	return l.Address
}

func (l *listenerWithAddr) Close() error {
	return nil
}

type connPair struct {
	conn net.Conn
	err  error
}

var ErrListenerClosed = fmt.Errorf("loopback listener closed")

type multiListener struct {
	listeners []net.Listener
	accept    chan connPair
	stop      chan struct{}
	once      sync.Once
	wg        sync.WaitGroup
}

func NewMultiListener(listeners ...net.Listener) net.Listener {
	l := &multiListener{
		listeners: listeners,
		accept:    make(chan connPair),
		stop:      make(chan struct{}),
	}
	for i := range listeners {
		l.wg.Add(1)
		go l.Run(listeners[i])
	}
	return l
}

func (l *multiListener) Run(listener net.Listener) {
	defer l.wg.Done()
	for {
		select {
		case _, ok := <-l.stop:
			if !ok {
				return
			}
		default:
		}
		c, err := listener.Accept()
		l.accept <- connPair{conn: c, err: err}
	}
}

func (l *multiListener) Accept() (c net.Conn, err error) {
	pair, ok := <-l.accept
	if !ok {
		return nil, ErrListenerClosed
	}
	return pair.conn, pair.err
}

// Close invokes close on each listener in the list, then signals to the
// accept workers to close, starts a goroutine to drain any new accepts
// while the workers complete, waits for the workers to exit, and finally
// returns. Each listener will only be closed once.
func (l *multiListener) Close() error {
	var firstErr error
	l.once.Do(func() {
		for i, li := range l.listeners {
			if i == 0 {
				firstErr = li.Close()
			} else {
				li.Close()
			}
		}
		close(l.stop)
		go func() {
			// drain any connections that may not have been accepted yet
			for pair := range l.accept {
				if pair.conn != nil {
					pair.conn.Close()
				}
			}
		}()
		l.wg.Wait()
		close(l.accept)
	})
	return firstErr
}

func (l *multiListener) Addr() net.Addr {
	return l.listeners[0].Addr()
}

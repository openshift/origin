package ldapserver

import (
	"bufio"
	"log"
	"net"
	"sync"
	"time"
)

// Server is an LDAP server.
type Server struct {
	Listener     net.Listener
	ReadTimeout  time.Duration  // optional read timeout
	WriteTimeout time.Duration  // optional write timeout
	wg           sync.WaitGroup // group of goroutines (1 by client)
	chDone       chan bool      // Channel Done, value => shutdown

	// OnNewConnection, if non-nil, is called on new connections.
	// If it returns non-nil, the connection is closed.
	OnNewConnection func(c net.Conn) error

	// Handler handle ldap message received by client
	// it SHOULD "implement" RequestHandler interface
	Handler Handler
}

//NewServer return a LDAP Server
func NewServer() *Server {
	return &Server{}
}

// Handle registers the handler for the server.
// If a handler already exists for pattern, Handle panics
func (s *Server) Handle(h Handler) {
	if s.Handler != nil {
		panic("LDAP: multiple Handler registrations")
	}
	s.Handler = h
}

// ListenAndServe listens on the TCP network address s.Addr and then
// calls Serve to handle requests on incoming connections.  If
// s.Addr is blank, ":389" is used.
func (s *Server) ListenAndServe(addr string, options ...func(*Server)) error {

	if addr == "" {
		addr = ":389"
	}

	var e error
	s.Listener, e = net.Listen("tcp", addr)
	if e != nil {
		return e
	}
	log.Printf("Listening on %s\n", addr)

	for _, option := range options {
		option(s)
	}

	return s.serve()
}

// Handle requests messages on the ln listener
func (s *Server) serve() error {
	defer s.Listener.Close()

	if s.Handler == nil {
		log.Panicln("No LDAP Request Handler defined")
	}

	s.chDone = make(chan bool)
	i := 0

	for {
		select {
		case <-s.chDone:
			log.Print("Stopping server")
			s.Listener.Close()
			return nil
		default:
		}

		rw, err := s.Listener.Accept()
		rw.SetDeadline(time.Now().Add(1e9))
		if nil != err {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			log.Println(err)
		}

		cli, err := s.newClient(rw)

		if err != nil {
			continue
		}

		i = i + 1
		cli.Numero = i
		log.Printf("Connection client [%d] from %s accepted", cli.Numero, cli.rwc.RemoteAddr().String())
		s.wg.Add(1)
		go cli.serve()
	}

	return nil
}

// Return a new session with the connection
// client has a writer and reader buffer
func (s *Server) newClient(rwc net.Conn) (c *client, err error) {
	c = &client{
		srv: s,
		rwc: rwc,
		br:  bufio.NewReader(rwc),
		bw:  bufio.NewWriter(rwc),
	}
	return c, nil
}

// Termination of the LDAP session is initiated by the server sending a
// Notice of Disconnection.  In this case, each
// protocol peer gracefully terminates the LDAP session by ceasing
// exchanges at the LDAP message layer, tearing down any SASL layer,
// tearing down any TLS layer, and closing the transport connection.
// A protocol peer may determine that the continuation of any
// communication would be pernicious, and in this case, it may abruptly
// terminate the session by ceasing communication and closing the
// transport connection.
// In either case, when the LDAP session is terminated.
func (s *Server) Stop() {
	close(s.chDone)
	log.Print("waiting for clients shutdown...")
	s.wg.Wait()
	log.Print("all client connections closed")
}

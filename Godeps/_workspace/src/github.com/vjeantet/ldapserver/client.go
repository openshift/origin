package ldapserver

import (
	"bufio"
	"log"
	"net"
	"reflect"
	"sync"
	"time"
)

type client struct {
	Numero      int
	srv         *Server
	rwc         net.Conn
	br          *bufio.Reader
	bw          *bufio.Writer
	chanOut     chan response
	wg          sync.WaitGroup
	closing     bool
	requestList map[int]Message
}

func (c *client) GetConn() net.Conn {
	return c.rwc
}

func (c *client) SetConn(conn net.Conn) {
	c.rwc = conn
	c.br = bufio.NewReader(c.rwc)
	c.bw = bufio.NewWriter(c.rwc)
}

func (c *client) GetMessageByID(messageID int) (*Message, bool) {
	if requestToAbandon, ok := c.requestList[messageID]; ok {
		return &requestToAbandon, true
	}
	return nil, false
}

func (c *client) Addr() net.Addr {
	return c.rwc.RemoteAddr()
}

func (c *client) serve() {
	defer c.close()

	if onc := c.srv.OnNewConnection; onc != nil {
		if err := onc(c.rwc); err != nil {
			log.Printf("Erreur OnNewConnection: %s", err)
			return
		}
	}

	// Create the ldap response queue to be writted to client (buffered to 20)
	// buffered to 20 means that If client is slow to handler responses, Server
	// Handlers will stop to send more respones
	c.chanOut = make(chan response)

	// for each message in c.chanOut send it to client
	go func() {
		for msg := range c.chanOut {
			c.writeLdapResult(msg)
		}

	}()

	// Listen for server signal to shutdown
	go func() {
		for {
			select {
			case <-c.srv.chDone: // server signals shutdown process
				r := NewExtendedResponse(LDAPResultUnwillingToPerform)
				r.DiagnosticMessage = "server is about to stop"
				r.ResponseName = NoticeOfDisconnection
				c.chanOut <- r
				c.rwc.SetReadDeadline(time.Now().Add(time.Second))
				return
			default:
				//FIX: This cause a Race condition
				if c.closing == true {
					return
				}
			}
		}
	}()

	c.requestList = make(map[int]Message)

	for {

		if c.srv.ReadTimeout != 0 {
			c.rwc.SetReadDeadline(time.Now().Add(c.srv.ReadTimeout))
		}

		//Read client input as a ASN1/BER binary message
		messagePacket, err := readMessagePacket(c.br)
		if err != nil {
			log.Printf("Error readMessagePacket: %s", err)
			return
		}

		// if client is in closing mode, drop message and exit
		// FIX: this cause a race condition
		if c.closing == true {
			log.Print("one client message dropped !")
			return
		}

		//Convert ASN1 binaryMessage to a ldap Request Message
		var message Message
		message, err = messagePacket.readMessage()

		if err != nil {
			log.Printf("Error reading Message : %s", err.Error())
			continue
		}
		log.Printf("<<< %d - %s - hex=%x", c.Numero, reflect.TypeOf(message.protocolOp).Name(), messagePacket.Packet.Bytes())

		// TODO: Use a implementation to limit runnuning request by client
		// solution 1 : when the buffered output channel is full, send a busy
		// solution 2 : when 10 client requests (goroutines) are running, send a busy message
		// And when the limit is reached THEN send a BusyLdapMessage

		// When message is an UnbindRequest, stop serving
		if _, ok := message.protocolOp.(UnbindRequest); ok {
			return
		}

		// If client requests a startTls, do not handle it in a
		// goroutine, connection has to remain free until TLS is OK
		// @see RFC https://tools.ietf.org/html/rfc4511#section-4.14.1
		if req, ok := message.protocolOp.(ExtendedRequest); ok {
			if req.GetResponseName() == NoticeOfStartTLS {
				c.wg.Add(1)
				c.ProcessRequestMessage(message)
				continue
			}
		}

		// TODO: go/non go routine choice should be done in the ProcessRequestMessage
		// not in the client.serve func
		c.wg.Add(1)
		go c.ProcessRequestMessage(message)
	}

}

// close closes client,
// * stop reading from client
// * signals to all currently running request processor to stop
// * wait for all request processor to end
// * close client connection
// * signal to server that client shutdown is ok
func (c *client) close() {
	log.Printf("client %d close()", c.Numero)
	c.closing = true //FIXME: subject to data race condition ?
	// stop reading from client
	c.rwc.SetReadDeadline(time.Now().Add(time.Second))

	// TODO: Send a Disconnection notification

	// signals to all currently running request processor to stop
	for messageID, request := range c.requestList {
		log.Printf("Client [%d] sent abandon signal to request[messageID = %d]", c.Numero, messageID)
		go request.Abandon()
	}

	// wait for all request processor to end
	//log.Printf("waiting the end of current (%d) client's requests", c.Numero)
	c.wg.Wait()
	close(c.chanOut)
	log.Printf("client [%d] request processors ended", c.Numero)

	// close client connection
	c.rwc.Close()
	log.Printf("client [%d] connection closed", c.Numero)

	// signal to server that client shutdown is ok
	c.srv.wg.Done()
}

func (c *client) writeLdapResult(lr response) {
	data := newMessagePacket(lr).Bytes()
	log.Printf(">>> %d - %s - hex=%x", c.Numero, reflect.TypeOf(lr).Elem().Name(), data)
	c.bw.Write(data)
	c.bw.Flush()
}

// ResponseWriter interface is used by an LDAP handler to
// construct an LDAP response.
type ResponseWriter interface {
	// Write writes the LDAPResponse to the connection as part of an LDAP reply.
	Write(lr response)
}

type responseWriterImpl struct {
	chanOut   chan response
	messageID int
}

func (w responseWriterImpl) Write(lr response) {
	lr.SetMessageID(w.messageID)
	w.chanOut <- lr
}

func (c *client) ProcessRequestMessage(m Message) {
	defer c.wg.Done()

	c.requestList[m.MessageID] = m
	defer delete(c.requestList, m.MessageID)

	m.Done = make(chan bool)

	var w responseWriterImpl
	w.chanOut = c.chanOut
	w.messageID = m.MessageID
	m.Client = c

	c.srv.Handler.ServeLDAP(w, &m)
}

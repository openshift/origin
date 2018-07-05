package nfnetlink

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"
)

var ErrShortResponse = errors.New("Got short response from netlink")

const recvBufferSize = 8192

var readResponseTimeout = 250 * time.Millisecond

type SockFlags int

const (
	FlagDebug SockFlags = 1 << iota
	FlagAckRequests
	FlagLogWarnings
)

func (sf SockFlags) isSet(f SockFlags) bool {
	return sf&f == f

}

func (sf *SockFlags) set(f SockFlags) {
	*sf |= f
}

func (sf *SockFlags) clear(f SockFlags) {
	*sf &= ^f
}

type nlResponseType int

const (
	responseAck nlResponseType = iota
	responseErr
	responseMsg
)

type netlinkResponse struct {
	rtype nlResponseType
	errno uint32
	msg   *NfNlMessage
}

type NetlinkSocket struct {
	fd          int                      // Socket file descriptor
	peer        *syscall.SockaddrNetlink // Destination address for sendto()
	recvChan    chan *NfNlMessage        // Channel for transmitting received messages
	recvError   error                    // If an error interrupts reception of messages store it here
	recvBuffer  []byte                   // Buffer for storing bytes read from socket
	seq         uint32                   // next sequence number to use
	flags       SockFlags
	responseMap map[uint32]chan *netlinkResponse // maps sequence numbers to channels to deliver response message on
	lock        sync.Mutex                       // protects responseMap and recvChan
}

// NewNetlinkSocket creates a new NetlinkSocket
func NewNetlinkSocket(bus int) (*NetlinkSocket, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, bus)
	if err != nil {
		return nil, err
	}

	lsa := &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}
	if err := syscall.Bind(fd, lsa); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	s := &NetlinkSocket{
		fd:          fd,
		flags:       FlagAckRequests,
		peer:        &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK},
		recvBuffer:  make([]byte, recvBufferSize),
		responseMap: make(map[uint32]chan *netlinkResponse),
	}

	go s.runReceiveLoop()

	return s, nil
}

// SetFlag adds the flag f to the set of enabled flags for this socket
func (s *NetlinkSocket) SetFlag(f SockFlags) {
	s.flags.set(f)
}

// ClearFlag removes the flag f from the set of enabled flags for this socket
func (s *NetlinkSocket) ClearFlag(f SockFlags) {
	s.flags.clear(f)
}

// Close the socket
func (s *NetlinkSocket) Close() {
	syscall.Close(s.fd)
}

// nextSeq returns a new sequence number to use when building a message
func (s *NetlinkSocket) nextSeq() uint32 {
	s.seq += 1
	if s.seq == 0 {
		s.seq = 1
	}
	return s.seq
}

// Send serializes msg and transmits in on the socket.
func (s *NetlinkSocket) Send(msg *NfNlMessage) error {
	msg.Seq = s.nextSeq()
	if s.flags.isSet(FlagAckRequests) {
		return s.sendWithAck(msg)
	}
	return s.sendMessage(msg)
}

// sendWithAck is called to send messages when FlagAckRequests is set to handle delivery
// and processing of reponse messages.
func (s *NetlinkSocket) sendWithAck(msg *NfNlMessage) error {
	msg.Flags |= syscall.NLM_F_ACK
	ch := s.createResponseChannel(msg.Seq)
	if err := s.sendMessage(msg); err != nil {
		s.removeResponseChannel(msg.Seq, true)
		return err
	}
	return s.readResponse(ch, msg)
}

// readResponse message from the provided channel and convert it into an error return value or
// return nil if the response was an ack
func (s *NetlinkSocket) readResponse(ch chan *netlinkResponse, msg *NfNlMessage) error {
	select {
	case resp := <-ch:
		switch resp.rtype {
		case responseAck:
			return nil
		case responseErr:
			return syscall.Errno(resp.errno)
		default:
			return fmt.Errorf("unexpected response type: %v to message (seq=%d)", resp.rtype, msg.Seq)
		}
	case <-time.After(readResponseTimeout):
		s.removeResponseChannel(msg.Seq, true)
		return fmt.Errorf("timeout waiting for expected response to message (seq=%d)", msg.Seq)
	}
}

func (s *NetlinkSocket) sendMessage(msg *NfNlMessage) error {
	bs := msg.Serialize()
	if s.flags.isSet(FlagDebug) {
		log.Printf("Send: %v '%s'", msg, hex.EncodeToString(bs))
	}
	return syscall.Sendto(s.fd, bs, 0, s.peer)
}

// Receive returns a channel to read incoming event messages from.
func (s *NetlinkSocket) Receive() <-chan *NfNlMessage {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.recvChan != nil {
		return s.recvChan
	}
	s.recvChan = make(chan *NfNlMessage)
	return s.recvChan
}

func (s *NetlinkSocket) runReceiveLoop() {
	if err := s.receive(); err != nil {
		s.recvError = err
		if s.recvChan != nil {
			close(s.recvChan)
		}
	}
}

// RecvErr returns an error value if reception of messages ended with
// an error.  When the channel returned by Receive() is closed this
// function should be called to determine the error, if any, that occurred.
func (s *NetlinkSocket) RecvErr() error {
	return s.recvError
}

// receive reads from the socket, parses messages, and writes each parsed message
// to the recvChan channel. The received buffer is aligned to NLMSG_ALIGNTO before
// parsing.
// It will loop reading and processing messages until an
// error occurs and then return the error.
func (s *NetlinkSocket) receive() error {
	for {
		n, err := s.fillRecvBuffer()
		if err != nil {
			return err
		}
		msgs, err := syscall.ParseNetlinkMessage(s.recvBuffer[:nlmAlignOf(n)])
		if err != nil {
			return err
		}
		for _, msg := range msgs {
			if err := s.processMessage(msg); err != nil {
				return err
			}
		}
	}
}

func (s *NetlinkSocket) processMessage(msg syscall.NetlinkMessage) error {
	if msg.Header.Type == syscall.NLMSG_ERROR {
		s.processErrorMsg(msg)
		return nil
	}
	nlm, err := s.parseMessage(msg)
	if err != nil {
		return err
	}
	s.deliverMessage(nlm)
	return nil
}

// deliverMessage sends the message out on the recvChan channel.  If the user
// has not called Receive() this channel will not have been created yet and
// the message will be dropped to avoid deadlocking.
func (s *NetlinkSocket) deliverMessage(nlm *NfNlMessage) {
	s.lock.Lock()
	if s.recvChan == nil {
		s.warn("Dropping message because nobody is listening for received messages")
		return
	}
	s.lock.Unlock()
	s.recvChan <- nlm
}

func (s *NetlinkSocket) warn(format string, v ...interface{}) {
	if s.flags.isSet(FlagLogWarnings) {
		log.Println(fmt.Sprintf(format, v...))
	}
}

func (s *NetlinkSocket) processErrorMsg(msg syscall.NetlinkMessage) {
	if len(msg.Data) < 4 {
		s.warn("Netlink error message received with short (%d byte) body", len(msg.Data))
		return
	}
	errno := readErrno(msg.Data)
	rtype := responseAck
	if errno != 0 {
		rtype = responseErr
	}
	m := s.parseMessageFromBytes(msg.Data[4:])
	s.sendResponse(msg.Header.Seq, rtype, errno, m)
}

func (s *NetlinkSocket) parseMessageFromBytes(data []byte) *NfNlMessage {
	if len(data) < syscall.NLMSG_HDRLEN+NFGEN_HDRLEN {
		return nil
	}
	msgs, err := syscall.ParseNetlinkMessage(data)
	if err != nil {
		s.warn("Error parsing netlink message inside error message: %v", err)
		return nil
	}
	if len(msgs) != 1 {
		s.warn("Expected 1 message got %d", len(msgs))
		return nil
	}
	m := s.NewNfNlMsg()
	if err := m.parse(bytes.NewReader(msgs[0].Data), msgs[0].Header); err != nil {
		s.warn("Error parsing message %v", err)
		return nil
	}
	return m
}

func (s *NetlinkSocket) sendResponse(seq uint32, rtype nlResponseType, errno uint32, msg *NfNlMessage) {
	ch := s.removeResponseChannel(seq, false)
	if ch == nil {
		s.warn("No response channel found for seq %d", seq)
		return
	}
	ch <- &netlinkResponse{
		rtype: rtype,
		errno: errno,
		msg:   msg,
	}
	close(ch)
}

func (s *NetlinkSocket) removeResponseChannel(seq uint32, closeChan bool) chan *netlinkResponse {
	s.lock.Lock()
	defer s.lock.Unlock()
	ch, ok := s.responseMap[seq]
	if !ok {
		return nil
	}
	delete(s.responseMap, seq)
	if closeChan {
		close(ch)
		return nil
	}
	return ch
}

func (s *NetlinkSocket) createResponseChannel(seq uint32) chan *netlinkResponse {
	ch := make(chan *netlinkResponse)
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.responseMap[seq] != nil {
		close(s.responseMap[seq])
	}
	s.responseMap[seq] = ch
	return ch
}

func readErrno(data []byte) uint32 {
	errno := int32(native.Uint32(data))
	return uint32(-errno)
}

// fillRecvBuffer reads from the socket into recvBuffer and returns the number
// of bytes read.  If less than NLMSG_HDRLEN bytes are read, ErrShortResponse
// is returned as an error.
func (s *NetlinkSocket) fillRecvBuffer() (int, error) {
	n, _, err := syscall.Recvfrom(s.fd, s.recvBuffer, 0)
	if err != nil {
		return 0, err
	}
	if n < syscall.NLMSG_HDRLEN {
		return 0, ErrShortResponse
	}
	return n, nil
}

// parseMessage converts a syscall.NetlinkMessage into a NfNlMessage by
// parsing the Data byte slice into a NfGenHdr and zero or more attribute
// instances.
func (s *NetlinkSocket) parseMessage(msg syscall.NetlinkMessage) (*NfNlMessage, error) {
	m := s.NewNfNlMsg()
	r := bytes.NewReader(msg.Data)
	if err := m.parse(r, msg.Header); err != nil {
		return nil, err
	}
	if s.flags.isSet(FlagDebug) {
		log.Printf("Recv: %v\n", m)
	}
	return m, nil
}

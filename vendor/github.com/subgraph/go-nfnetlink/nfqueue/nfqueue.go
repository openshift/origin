package nfqueue

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/subgraph/go-nfnetlink"
	"syscall"
)

const (
	NFQNL_CFG_CMD_BIND   = 1
	NFQNL_CFG_CMD_UNBIND = 2

	NFQNL_COPY_META   = 1
	NFQNL_COPY_PACKET = 2

	NFNL_SUBSYS_QUEUE = 3

	NFQNL_MSG_PACKET  = 0
	NFQNL_MSG_VERDICT = 1
	NFQNL_MSG_CONFIG  = 2

	NFNETLINK_V0 = 0

	NFQA_CFG_COMMAND = 1
	NFQA_CFG_PARAMS  = 2

	NFQA_PACKET_HDR  = 1
	NFQA_VERDICT_HDR = 2
	NFQA_MARK        = 3
	NFQA_PAYLOAD     = 10

	NF_DROP   = 0
	NF_ACCEPT = 1
)

type NFQPacket struct {
	id      uint32          // packet id
	HwProto uint16          // hardware protocol
	Packet  gopacket.Packet // packet data
	q       *NFQueue        // queue instance
}

type NFQueue struct {
	queue      uint16                   // queue number
	packets    chan *NFQPacket          // channel for delivering packets
	copySize   uint32                   // configured copy size
	pendingErr error                    // error which occured while receiving packet messages
	nls        *nfnetlink.NetlinkSocket // netlink socket to netfilter queue subsystem
	debug      bool                     // set to true if debugging enabled
}

const defaultCopySize = 0xFFFF

// NewNFQueue creates and returns a new NFQueue instance.
func NewNFQueue(queue uint16) *NFQueue {
	return &NFQueue{
		queue:    queue,
		copySize: defaultCopySize,
		packets:  make(chan *NFQPacket),
	}
}

// EnableDebug sets a flag on the associated NetlinkSocket causing it to dump information
// about each received and transmitted message.
func (q *NFQueue) EnableDebug() {
	if q.nls != nil {
		q.nls.SetFlag(nfnetlink.FlagDebug)
	}
	q.debug = true

}

// SetCopySize can be called before Open to set the packet capture size
func (q *NFQueue) SetCopySize(sz uint32) {
	q.copySize = sz
}

// open creates a netlink socket connection to the netfilter subsystem and
// configures the connection to receive packets from netfilter queue
func (q *NFQueue) open() error {
	nls, err := nfnetlink.NewNetlinkSocket(syscall.NETLINK_NETFILTER)
	if err != nil {
		return err
	}
	q.nls = nls
	if q.debug {
		q.nls.SetFlag(nfnetlink.FlagDebug)
	}

	err = q.sendAll(
		q.nfqRequestConfigCmd(NFQNL_CFG_CMD_BIND, q.queue, 0),
		q.nfqRequestConfigParams(q.copySize, NFQNL_COPY_PACKET),
	)
	if err != nil {
		q.Close()
		return err
	}
	return nil
}

// Close this queue instance
func (q *NFQueue) Close() {
	q.sendAll(
		q.nfqRequestConfigCmd(NFQNL_CFG_CMD_UNBIND, q.queue, 0),
	)
	q.nls.Close()
}

// sendAll sends a series of messages, returning the first error encountered if any.
func (q *NFQueue) sendAll(msgs ...*nfnetlink.NfNlMessage) error {
	for _, m := range msgs {
		if err := m.Send(); err != nil {
			return err
		}
	}
	return nil
}

// Open this queue instance.  Returns a channel for reading received packets.
func (q *NFQueue) Open() (<-chan *NFQPacket, error) {
	if err := q.open(); err != nil {
		return nil, err
	}
	go q.receivePackets()
	return q.packets, nil
}

// receivePackets reads messages from the channel returned by NetlinkSocket.Receive and
// processes them until the channel is closed.  If an error occurs, this error is assigned to
// q.pendingError
func (q *NFQueue) receivePackets() {
	for m := range q.nls.Receive() {
		if err := q.processPacket(m); err != nil {
			q.pendingErr = err
			close(q.packets)
			return
		}
	}
	if q.nls.RecvErr() != nil {
		q.pendingErr = q.nls.RecvErr()
	}
	close(q.packets)
}

// PendingError returns the error that was encountered while receiving packets if any.
func (q *NFQueue) PendingError() error {
	return q.pendingErr
}

// processPacket handles an incoming NfNlMessage which is assumed to contain a received packet.
func (q *NFQueue) processPacket(m *nfnetlink.NfNlMessage) error {
	hdr := m.AttrByType(NFQA_PACKET_HDR)
	if hdr == nil {
		return fmt.Errorf("No NFQA_PACKET_HDR\n")
	}
	p := &NFQPacket{q: q}
	hdr.ReadFields(&p.id, &p.HwProto)
	payload := m.AttrByType(NFQA_PAYLOAD)
	if payload != nil {
		p.Packet = gopacket.NewPacket(payload.Data, layers.LayerTypeIPv4,
			gopacket.DecodeOptions{Lazy: true, NoCopy: true})
	}
	q.packets <- p
	return nil
}

// nfqRequestConfigCmd creates an NFQNL_MSG_CONFIG message with a NFQA_CFG_COMMAND attribute for
// the provided cmd, queue number, and protocol family.
func (q *NFQueue) nfqRequestConfigCmd(cmd uint8, queue uint16, pf uint16) *nfnetlink.NfNlMessage {
	nr := q.nfqNewRequest(NFQNL_MSG_CONFIG, queue)
	nr.AddAttributeFields(NFQA_CFG_COMMAND, cmd, uint8(0), pf)
	return nr
}

// nfqRequestConfigParams creates an NFQNL_MSG_CONFIG message with a NFQA_CFG_PARAMS attribute for
// the provided copyRange and copyMode values.
func (q *NFQueue) nfqRequestConfigParams(copyRange uint32, copyMode uint8) *nfnetlink.NfNlMessage {
	nr := q.nfqNewRequest(NFQNL_MSG_CONFIG, q.queue)
	nr.AddAttributeFields(NFQA_CFG_PARAMS, copyRange, copyMode)
	return nr
}

// nfqRequestVerdictMark creates an NFQNL_MSG_VERDICT with the provided id and verdict values.  If hasMark is set
// an optional NFQA_MARK attribute will be included to set a mark on the packet with value mark
func (q *NFQueue) nfqRequestVerdictMark(verdict uint32, id uint32, hasMark bool, mark uint32) *nfnetlink.NfNlMessage {
	nr := q.nfqNewRequest(NFQNL_MSG_VERDICT, q.queue)
	nr.AddAttributeFields(NFQA_VERDICT_HDR, verdict, id)
	if hasMark {
		nr.AddAttributeFields(NFQA_MARK, mark)
	}
	return nr
}

// nfqNewRequest creates a new message to the queue subsystem with the given type and queue number
func (q *NFQueue) nfqNewRequest(mtype uint8, queue uint16) *nfnetlink.NfNlMessage {
	nlm := q.nls.NewNfNlMsg()
	nlm.Type = uint16((NFNL_SUBSYS_QUEUE << 8) | uint16(mtype))
	nlm.Flags = syscall.NLM_F_REQUEST
	nlm.Family = syscall.AF_UNSPEC
	nlm.Version = NFNETLINK_V0
	nlm.ResID = queue
	return nlm
}

// Drop sets the NF_DROP verdict on this packet
func (p *NFQPacket) Drop() error {
	return p.verdict(NF_DROP)
}

// Accept sets the NF_ACCEPT verdict on this packet
func (p *NFQPacket) Accept() error {
	return p.verdict(NF_ACCEPT)
}

// verdict sends a NFQNL_MSG_VERDICT message for the packet id with the verdict value v
func (p *NFQPacket) verdict(v uint32) error {
	return p.q.nfqRequestVerdictMark(v, p.id, false, 0).Send()
}

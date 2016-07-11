package ldapserver

import (
	"fmt"
	"reflect"
)

// response is the interface implemented by each ldap response (BinResponse, SearchResponse, SearchEntryResult,...) struct
type response interface {
	SetMessageID(ID int)
}

// ldapResult is the construct used in LDAP protocol to return
// success or failure indications from servers to clients.  To various
// requests, servers will return responses containing the elements found
// in LDAPResult to indicate the final status of the protocol operation
// request.
type ldapResult struct {
	ResultCode        int
	MatchedDN         LDAPDN
	DiagnosticMessage string
	referral          interface{}
	MessageID         int
}

func (e *ldapResult) SetMessageID(ID int) {
	e.MessageID = ID
}

func NewResponse(resultCode int) *ldapResult {
	r := &ldapResult{}
	r.ResultCode = resultCode
	return r
}

type ProtocolOp interface {
}

type Message struct {
	Client     *client
	MessageID  int
	protocolOp ProtocolOp
	Controls   []interface{}
	Done       chan bool
}

func (m *Message) String() string {
	return fmt.Sprintf("MessageId=%d, %s", m.MessageID, reflect.TypeOf(m.protocolOp).Name)
}

// Abandon close the Done channel, to notify handler's user function to stop any
// running process
func (m *Message) Abandon() {
	if m.Done != nil {
		close(m.Done)
	}
}

//GetDoneChannel return a channel, which indicate the the request should be
//aborted quickly, because the client abandonned the request, the server qui quitting, ...
func (m *Message) GetDoneChannel() chan bool {
	return m.Done
}

func (m *Message) GetProtocolOp() ProtocolOp {
	return m.protocolOp
}

func (m *Message) GetAbandonRequest() AbandonRequest {
	return m.protocolOp.(AbandonRequest)
}
func (m *Message) GetSearchRequest() SearchRequest {
	return m.protocolOp.(SearchRequest)
}

func (m *Message) GetBindRequest() BindRequest {
	return m.protocolOp.(BindRequest)
}

func (m *Message) GetAddRequest() AddRequest {
	return m.protocolOp.(AddRequest)
}

func (m *Message) GetDeleteRequest() DeleteRequest {
	return m.protocolOp.(DeleteRequest)
}

func (m *Message) GetModifyRequest() ModifyRequest {
	return m.protocolOp.(ModifyRequest)
}

func (m *Message) GetCompareRequest() CompareRequest {
	return m.protocolOp.(CompareRequest)
}

func (m *Message) GetExtendedRequest() ExtendedRequest {
	return m.protocolOp.(ExtendedRequest)
}

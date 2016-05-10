package ldapserver

// ExtendedRequest operation allows additional operations to be defined for
// services not already available in the protocol
// The Extended operation allows clients to send request with predefined
// syntaxes and semantics.  These may be defined in RFCs or be private to
// particular implementations.
type ExtendedRequest struct {
	requestName  LDAPOID
	requestValue []byte
}

func (r *ExtendedRequest) GetResponseName() LDAPOID {
	return r.requestName
}

func (r *ExtendedRequest) GetResponseValue() []byte {
	return r.requestValue
}

// ExtendedResponse operation allows additional operations to be defined for
// services not already available in the protocol, like the disconnection
// notification sent by the server before it stops serving
// The Extended operation allows clients to receive
// responses with predefined syntaxes and semantics.  These may be
// defined in RFCs or be private to particular implementations.
type ExtendedResponse struct {
	ldapResult
	ResponseName  LDAPOID
	ResponseValue string
}

func NewExtendedResponse(resultCode int) *ExtendedResponse {
	r := &ExtendedResponse{}
	r.ResultCode = resultCode
	return r
}

package ldapserver

type CompareRequest struct {
	entry LDAPDN
	ava   AttributeValueAssertion
}

func (r *CompareRequest) GetEntry() LDAPDN {
	return r.entry
}

func (r *CompareRequest) GetAttributeValueAssertion() *AttributeValueAssertion {
	return &r.ava
}

type CompareResponse struct {
	ldapResult
}

func NewCompareResponse(resultCode int) *CompareResponse {
	r := &CompareResponse{}
	r.ResultCode = resultCode
	return r
}

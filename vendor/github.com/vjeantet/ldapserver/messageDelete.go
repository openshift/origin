package ldapserver

// DeleteRequest is a definition of the Delete Operation
type DeleteRequest LDAPDN

// GetEntryDN returns the entry's DN to delete
func (r *DeleteRequest) GetEntryDN() LDAPDN {
	return LDAPDN(*r)
}

type DeleteResponse struct {
	ldapResult
}

func NewDeleteResponse(resultCode int) *DeleteResponse {
	r := &DeleteResponse{}
	r.ResultCode = resultCode
	return r
}

package ldapserver

type ModifyRequest struct {
	object  LDAPDN
	changes []modifyRequestChange
}

func (r *ModifyRequest) GetChanges() []modifyRequestChange {
	return r.changes
}

func (r *ModifyRequest) GetObject() LDAPDN {
	return r.object
}

type modifyRequestChange struct {
	operation    int
	modification PartialAttribute
}

func (r *modifyRequestChange) GetModification() PartialAttribute {
	return r.modification
}

func (r *modifyRequestChange) GetOperation() int {
	return r.operation
}

type ModifyResponse struct {
	ldapResult
}

func NewModifyResponse(resultCode int) *ModifyResponse {
	r := &ModifyResponse{}
	r.ResultCode = resultCode
	return r
}

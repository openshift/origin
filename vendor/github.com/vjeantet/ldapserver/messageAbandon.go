package ldapserver

// AbandonRequest operation's function is allow a client to request
// that the server abandon an uncompleted operation.  The Abandon
// Request is defined as follows:
type AbandonRequest int

// GetIDToAbandon retrieves the message ID of the operation to abandon
func (r *AbandonRequest) GetIDToAbandon() int {
	return int(*r)
}

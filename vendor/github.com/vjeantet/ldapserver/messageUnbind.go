package ldapserver

// UnbindRequest's function is to terminate an LDAP session.
// The Unbind operation is not the antithesis of the Bind operation as
// the name implies.  The naming of these operations are historical.
// The Unbind operation should be thought of as the "quit" operation.
type UnbindRequest struct {
}

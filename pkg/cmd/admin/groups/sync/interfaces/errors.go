package interfaces

import (
	"fmt"
)

func NewMemberLookupError(ldapGroupUID, ldapUserUID string, causedBy error) error {
	return &memberLookupError{ldapGroupUID: ldapGroupUID, ldapUserUID: ldapUserUID, causedBy: causedBy}
}

type memberLookupError struct {
	ldapGroupUID string
	ldapUserUID  string
	causedBy     error
}

func (e *memberLookupError) Error() string {
	return fmt.Sprintf("membership lookup for user %q in group %q failed because of %q", e.ldapUserUID, e.ldapGroupUID, e.causedBy.Error())
}

func IsmemberLookupError(e error) bool {
	if e == nil {
		return false
	}

	_, ok := e.(*memberLookupError)
	return ok
}

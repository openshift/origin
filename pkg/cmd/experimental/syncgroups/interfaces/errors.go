package interfaces

import (
	"fmt"
)

type MemberLookupError struct {
	LDAPGroupUID string
	LDAPUserUID  string
	CausedBy     error
}

func (e *MemberLookupError) Error() string {
	return fmt.Sprintf("membership lookup for user %q in group %q failed because of %q", e.LDAPUserUID, e.LDAPGroupUID, e.CausedBy.Error())
}

func IsMemberLookupError(e error) bool {
	if e == nil {
		return false
	}

	_, ok := e.(*MemberLookupError)
	return ok
}

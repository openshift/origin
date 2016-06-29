package groupdetector

import (
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/interfaces"
)

// NewGroupBasedDetector returns an LDAPGroupDetector that determines group existence based on
// the presence of a first-class group entry in LDAP as found by an LDAPGroupGetter
func NewGroupBasedDetector(groupGetter interfaces.LDAPGroupGetter) interfaces.LDAPGroupDetector {
	return &GroupBasedDetector{groupGetter: groupGetter}
}

// GroupBasedDetector is an LDAPGroupDetector that determines group existence based on
// the presence of a first-class group entry in LDAP as found by an LDAPGroupGetter
type GroupBasedDetector struct {
	groupGetter interfaces.LDAPGroupGetter
}

func (l *GroupBasedDetector) Exists(ldapGroupUID string) (bool, error) {
	group, err := l.groupGetter.GroupEntryFor(ldapGroupUID)
	if ldaputil.IsQueryOutOfBoundsError(err) || ldaputil.IsEntryNotFoundError(err) || ldaputil.IsNoSuchObjectError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if group == nil {
		return false, nil
	}

	return true, nil
}

// NewMemberBasedDetector returns an LDAPGroupDetector that determines group existence based on
// the presence of a non-zero number of first-class member entries in LDAP as found by an LDAPMemberExtractor
func NewMemberBasedDetector(memberExtractor interfaces.LDAPMemberExtractor) interfaces.LDAPGroupDetector {
	return &MemberBasedDetector{memberExtractor: memberExtractor}
}

// MemberBasedDetector is an LDAPGroupDetector that determines group existence based on
// the presence of a non-zero number of first-class member entries in LDAP as found by an LDAPMemberExtractor
type MemberBasedDetector struct {
	memberExtractor interfaces.LDAPMemberExtractor
}

func (l *MemberBasedDetector) Exists(ldapGrouUID string) (bool, error) {
	members, err := l.memberExtractor.ExtractMembers(ldapGrouUID)
	if ldaputil.IsQueryOutOfBoundsError(err) || ldaputil.IsEntryNotFoundError(err) || ldaputil.IsNoSuchObjectError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if len(members) == 0 {
		return false, nil
	}

	return true, nil
}

// NewCompoundDetector returns an LDAPGroupDetector that subsumes some other LDAPGroupDetectors.
// This detector checks all subordinate detectors in order to determine if a group exists. If any of
// the subordinate detectors raise an error while being queried, the the search is abandoned and the
// error returned. All detectors must successfully determine existence for the compound detector to
// determine that the group exists.
func NewCompoundDetector(locators ...interfaces.LDAPGroupDetector) interfaces.LDAPGroupDetector {
	return &CompoundDetector{locators: locators}
}

// CompoundDetector is an LDAPGroupDetector that subsumes some other LDAPGroupDetectors.
// This detector checks all subordinate detectors in order to determine if a group exists. If any of
// the subordinate detectors raise an error while being queried, the the search is abandoned and the
// error returned. All detectors must successfully determine existence for the compound detector to
// determine that the group exists.
type CompoundDetector struct {
	locators []interfaces.LDAPGroupDetector
}

func (l *CompoundDetector) Exists(ldapGrouUID string) (bool, error) {
	if len(l.locators) == 0 {
		return false, nil
	}

	conclusion := true
	for _, locator := range l.locators {
		opinion, err := locator.Exists(ldapGrouUID)
		if err != nil {
			return false, err
		}
		conclusion = conclusion && opinion
	}
	return conclusion, nil
}

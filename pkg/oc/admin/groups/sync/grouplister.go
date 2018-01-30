package syncgroups

import (
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync/interfaces"
	ouserapi "github.com/openshift/origin/pkg/user/apis/user"
	usertypedclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

// NewAllOpenShiftGroupLister returns a new allOpenShiftGroupLister
func NewAllOpenShiftGroupLister(blacklist []string, ldapURL string, groupClient usertypedclient.GroupInterface) interfaces.LDAPGroupListerNameMapper {
	return &allOpenShiftGroupLister{
		blacklist: sets.NewString(blacklist...),
		client:    groupClient,
		ldapURL:   ldapURL,
		ldapGroupUIDToOpenShiftGroupName: map[string]string{},
	}
}

// allOpenShiftGroupLister lists unique identifiers for LDAP lookup of all local OpenShift Groups that
// have been marked with an LDAP URL annotation as a result of a previous sync.
type allOpenShiftGroupLister struct {
	blacklist sets.String

	client  usertypedclient.GroupInterface
	ldapURL string

	ldapGroupUIDToOpenShiftGroupName map[string]string
}

func (l *allOpenShiftGroupLister) ListGroups() ([]string, error) {
	host, _, err := net.SplitHostPort(l.ldapURL)
	if err != nil {
		return nil, err
	}
	hostSelector := labels.Set(map[string]string{ldaputil.LDAPHostLabel: host}).AsSelector()
	allGroups, err := l.client.List(metav1.ListOptions{LabelSelector: hostSelector.String()})
	if err != nil {
		return nil, err
	}

	var ldapGroupUIDs []string
	for _, group := range allGroups.Items {
		if l.blacklist.Has(group.Name) {
			continue
		}

		matches, err := validateGroupAnnotations(l.ldapURL, group)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}

		ldapGroupUID := group.Annotations[ldaputil.LDAPUIDAnnotation]
		l.ldapGroupUIDToOpenShiftGroupName[ldapGroupUID] = group.Name
		ldapGroupUIDs = append(ldapGroupUIDs, ldapGroupUID)
	}

	return ldapGroupUIDs, nil
}

func (l *allOpenShiftGroupLister) GroupNameFor(ldapGroupUID string) (string, error) {
	// we probabably haven't been initialized.  This would be really weird
	if len(l.ldapGroupUIDToOpenShiftGroupName) == 0 {
		_, err := l.ListGroups()
		if err != nil {
			return "", err
		}
	}

	openshiftGroupName, exists := l.ldapGroupUIDToOpenShiftGroupName[ldapGroupUID]
	if !exists {
		return "", fmt.Errorf("no mapping found for %q", ldapGroupUID)
	}
	return openshiftGroupName, nil
}

// validateGroupAnnotations determines if the group matches and errors if the annotations are missing
func validateGroupAnnotations(ldapURL string, group ouserapi.Group) (bool, error) {
	if actualURL, exists := group.Annotations[ldaputil.LDAPURLAnnotation]; !exists {
		return false, fmt.Errorf("group %q marked as having been synced did not have an %s annotation", group.Name, ldaputil.LDAPURLAnnotation)

	} else if actualURL != ldapURL {
		return false, nil
	}

	if _, exists := group.Annotations[ldaputil.LDAPUIDAnnotation]; !exists {
		return false, fmt.Errorf("group %q marked as having been synced did not have an %s annotation", group.Name, ldaputil.LDAPUIDAnnotation)
	}

	return true, nil
}

// NewOpenShiftGroupLister returns a new openshiftGroupLister that divulges the LDAP group unique identifier for
// each entry in the given whitelist of OpenShift Group names
func NewOpenShiftGroupLister(whitelist, blacklist []string, ldapURL string, client usertypedclient.GroupInterface) interfaces.LDAPGroupListerNameMapper {
	return &openshiftGroupLister{
		whitelist: whitelist,
		blacklist: sets.NewString(blacklist...),
		client:    client,
		ldapURL:   ldapURL,
		ldapGroupUIDToOpenShiftGroupName: map[string]string{},
	}
}

// openshiftGroupLister lists unique identifiers for LDAP lookup of all local OpenShift groups that have
// been given to it upon creation.
type openshiftGroupLister struct {
	whitelist []string
	blacklist sets.String

	client  usertypedclient.GroupInterface
	ldapURL string

	ldapGroupUIDToOpenShiftGroupName map[string]string
}

func (l *openshiftGroupLister) ListGroups() ([]string, error) {
	var groups []ouserapi.Group
	for _, name := range l.whitelist {
		if l.blacklist.Has(name) {
			continue
		}

		group, err := l.client.Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}

	var ldapGroupUIDs []string
	for _, group := range groups {
		matches, err := validateGroupAnnotations(l.ldapURL, group)
		if err != nil {
			return nil, err
		}
		if !matches {
			return nil, fmt.Errorf("group %q was not synchronized from: %s", group.Name, l.ldapURL)
		}

		ldapGroupUID := group.Annotations[ldaputil.LDAPUIDAnnotation]
		l.ldapGroupUIDToOpenShiftGroupName[ldapGroupUID] = group.Name
		ldapGroupUIDs = append(ldapGroupUIDs, ldapGroupUID)
	}
	return ldapGroupUIDs, nil
}

func (l *openshiftGroupLister) GroupNameFor(ldapGroupUID string) (string, error) {
	// we probabably haven't been initialized.  This would be really weird
	if len(l.ldapGroupUIDToOpenShiftGroupName) == 0 {
		_, err := l.ListGroups()
		if err != nil {
			return "", err
		}
	}

	openshiftGroupName, exists := l.ldapGroupUIDToOpenShiftGroupName[ldapGroupUID]
	if !exists {
		return "", fmt.Errorf("no mapping found for %q", ldapGroupUID)
	}
	return openshiftGroupName, nil
}

// NewLDAPWhitelistGroupLister returns a new whitelistLDAPGroupLister that divulges the given whitelist
// of LDAP group unique identifiers
func NewLDAPWhitelistGroupLister(whitelist []string) interfaces.LDAPGroupLister {
	return &whitelistLDAPGroupLister{
		ldapGroupUIDs: whitelist,
	}
}

// LDAPGroupLister lists LDAP groups unique group identifiers given to it upon creation.
type whitelistLDAPGroupLister struct {
	ldapGroupUIDs []string
}

func (l *whitelistLDAPGroupLister) ListGroups() ([]string, error) {
	return l.ldapGroupUIDs, nil
}

// NewLDAPBlacklistGroupLister filters out the blacklisted names from the base lister
func NewLDAPBlacklistGroupLister(blacklist []string, baseLister interfaces.LDAPGroupLister) interfaces.LDAPGroupLister {
	return &blacklistLDAPGroupLister{
		blacklist:  sets.NewString(blacklist...),
		baseLister: baseLister,
	}
}

// LDAPGroupLister lists LDAP groups unique group identifiers given to it upon creation.
type blacklistLDAPGroupLister struct {
	blacklist sets.String

	baseLister interfaces.LDAPGroupLister
}

func (l *blacklistLDAPGroupLister) ListGroups() ([]string, error) {
	allNames, err := l.baseLister.ListGroups()
	if err != nil {
		return nil, err
	}

	// iterate through instead of  "Difference" to preserve ordering
	ret := []string{}
	for _, name := range allNames {
		if l.blacklist.Has(name) {
			continue
		}

		ret = append(ret, name)
	}

	return ret, nil
}

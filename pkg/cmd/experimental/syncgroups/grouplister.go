package syncgroups

import (
	"fmt"

	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	ouserapi "github.com/openshift/origin/pkg/user/api"
)

// NewAllOpenShiftGroupLister returns a new AllLocalGroupLister
func NewAllOpenShiftGroupLister(ldapURL string, groupClient osclient.GroupInterface) interfaces.LDAPGroupLister {
	return &AllLocalGroupLister{
		client:  groupClient,
		ldapURL: ldapURL,
	}
}

// AllLocalGroupLister lists unique identifiers for LDAP lookup of all local OpenShift Groups that
// have been marked with an LDAP URL annotation as a result of a previous sync.
type AllLocalGroupLister struct {
	client osclient.GroupInterface
	// ldapURL is the host:port of the LDAP server, used to identify if an OpenShift Group has
	// been synced with a specific server in order to isolate sync jobs between different servers
	ldapURL string
}

func (l *AllLocalGroupLister) ListGroups() (ldapGroupUIDs []string, err error) {
	allGroups, err := l.client.List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	var potentialGroups []ouserapi.Group
	for _, group := range allGroups.Items {
		val, exists := group.Annotations[ldaputil.LDAPURLAnnotation]
		if exists && (val == l.ldapURL) {
			potentialGroups = append(potentialGroups, group)
		}
	}

	for _, group := range potentialGroups {
		if err := validateGroupAnnotations(group); err != nil {
			return nil, err
		}
		ldapGroupUIDs = append(ldapGroupUIDs, group.Annotations[ldaputil.LDAPUIDAnnotation])
	}
	return ldapGroupUIDs, nil
}

// validateGroupAnnotations determines if the appropriate and annotations exist on a group
func validateGroupAnnotations(group ouserapi.Group) error {
	_, exists := group.Annotations[ldaputil.LDAPUIDAnnotation]
	if !exists {
		return fmt.Errorf("an OpenShift Group marked as having been synced did not have a %s annotation: %v",
			ldaputil.LDAPUIDAnnotation, group)
	}
	return nil
}

// NewOpenShiftWhitelistGroupLister returns a new LocalGroupLister that divulges the LDAP group unique identifier for
// each entry in the given whitelist of OpenShift Group names
func NewOpenShiftWhitelistGroupLister(whitelist []string, client osclient.GroupInterface) interfaces.LDAPGroupLister {
	return &LocalGroupLister{
		whitelist: whitelist,
		client:    client,
	}
}

// LocalGroupLister lists unique identifiers for LDAP lookup of all local OpenShift groups that have
// been given to it upon creation.
type LocalGroupLister struct {
	whitelist []string
	client    osclient.GroupInterface
}

func (l *LocalGroupLister) ListGroups() (ldapGroupUIDs []string, err error) {
	groups, err := getOpenShiftGroups(l.whitelist, l.client)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		if err := validateGroupAnnotations(group); err != nil {
			return nil, err
		}
		ldapGroupUIDs = append(ldapGroupUIDs, group.Annotations[ldaputil.LDAPUIDAnnotation])
	}
	return ldapGroupUIDs, err
}

// getOpenShiftGroups uses a client to retrieve all groups from the names given
func getOpenShiftGroups(names []string, client osclient.GroupInterface) ([]ouserapi.Group, error) {
	var groups []ouserapi.Group
	for _, name := range names {
		group, err := client.Get(name)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}
	return groups, nil
}

// NewLDAPWhitelistGroupLister returns a new WhitelistLDAPGroupLister that divulges the given whitelist
// of LDAP group unique identifiers
func NewLDAPWhitelistGroupLister(whitelist []string) interfaces.LDAPGroupLister {
	return &WhitelistLDAPGroupLister{
		GroupUIDs: whitelist,
	}
}

// LDAPGroupLister lists LDAP groups unique group identifiers given to it upon creation.
type WhitelistLDAPGroupLister struct {
	GroupUIDs []string
}

func (l *WhitelistLDAPGroupLister) ListGroups() (ldapGroupUIDs []string, err error) {
	return l.GroupUIDs, nil
}

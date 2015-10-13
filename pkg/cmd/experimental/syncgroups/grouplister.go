package syncgroups

import (
	"fmt"
	"strings"

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
	// ldapURL is the host:port of the LDAP server, used to identify if an OpenShift Group has been synced
	// with a specific server in order to isolate sync jobs between different servers
	ldapURL string
}

func (l *AllLocalGroupLister) ListGroups() ([]string, error) {
	host := strings.Split(l.ldapURL, ":")[0] // we only want to select on the host, not the port
	hostSelector := labels.Set(map[string]string{ldaputil.LDAPHostLabel: host}).AsSelector()
	potentialGroups, err := l.client.List(hostSelector, fields.Everything())
	if err != nil {
		return nil, err
	}

	var ldapGroupUIDs []string
	for _, group := range potentialGroups.Items {
		url, exists := group.Annotations[ldaputil.LDAPURLAnnotation]
		if !exists {
			return nil, fmt.Errorf("group %q: %s annotation expected: wanted %s",
				group.Name, ldaputil.LDAPURLAnnotation, l.ldapURL)
		}
		if url != l.ldapURL {
			// this group was created by another LDAP endpoint on the same server, skip it
			continue
		}
		if err := validateGroupAnnotations(group); err != nil {
			return nil, err
		}
		ldapGroupUIDs = append(ldapGroupUIDs, group.Annotations[ldaputil.LDAPUIDAnnotation])
	}
	return ldapGroupUIDs, nil
}

// validateGroupAnnotations determines if the appropriate and annotations exist on a group
func validateGroupAnnotations(group ouserapi.Group) error {
	if _, exists := group.Annotations[ldaputil.LDAPUIDAnnotation]; !exists {
		return fmt.Errorf("group %q: %s annotation expected", group.Name, ldaputil.LDAPUIDAnnotation)
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

func (l *LocalGroupLister) ListGroups() ([]string, error) {
	groups, err := getOpenShiftGroups(l.whitelist, l.client)
	if err != nil {
		return nil, err
	}

	var ldapGroupUIDs []string
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

func (l *WhitelistLDAPGroupLister) ListGroups() ([]string, error) {
	return l.GroupUIDs, nil
}

package syncgroups

import (
	"fmt"
	"net"

	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	ouserapi "github.com/openshift/origin/pkg/user/api"
)

// NewAllOpenShiftGroupLister returns a new allOpenShiftGroupLister
func NewAllOpenShiftGroupLister(blacklist []string, ldapURL string, groupClient osclient.GroupInterface) interfaces.LDAPGroupLister {
	return &allOpenShiftGroupLister{
		blacklist: sets.NewString(blacklist...),
		client:    groupClient,
		ldapURL:   ldapURL,
	}
}

// allOpenShiftGroupLister lists unique identifiers for LDAP lookup of all local OpenShift Groups that
// have been marked with an LDAP URL annotation as a result of a previous sync.
type allOpenShiftGroupLister struct {
	blacklist sets.String

	client osclient.GroupInterface
	// ldapURL is the host:port of the LDAP server, used to identify if an OpenShift Group has been synced
	// with a specific server in order to isolate sync jobs between different servers
	ldapURL string
}

func (l *allOpenShiftGroupLister) ListGroups() ([]string, error) {
	host, _, err := net.SplitHostPort(l.ldapURL)
	if err != nil {
		return nil, err
	}
	hostSelector := labels.Set(map[string]string{ldaputil.LDAPHostLabel: host}).AsSelector()
	potentialGroups, err := l.client.List(hostSelector, fields.Everything())
	if err != nil {
		return nil, err
	}

	var ldapGroupUIDs []string
	for _, group := range potentialGroups.Items {
		if !l.blacklist.Has(group.Name) {
			matches, err := validateGroupAnnotations(l.ldapURL, group)
			if err != nil {
				return nil, err
			}
			if matches {
				ldapGroupUIDs = append(ldapGroupUIDs, group.Annotations[ldaputil.LDAPUIDAnnotation])
			}
		}
	}

	return ldapGroupUIDs, nil
}

// validateGroupAnnotations determines if the group matches and errors if the annotations are missing
func validateGroupAnnotations(ldapURL string, group ouserapi.Group) (bool, error) {
	if actualURL, exists := group.Annotations[ldaputil.LDAPURLAnnotation]; !exists {
		return false, fmt.Errorf("group %q marked as having been synced did not have an %s annotation",
			group.Name, ldaputil.LDAPURLAnnotation)
	} else if actualURL != ldapURL {
		return false, nil
	}

	if _, exists := group.Annotations[ldaputil.LDAPUIDAnnotation]; !exists {
		return false, fmt.Errorf("group %q marked as having been synced did not have an %s annotation",
			group.Name, ldaputil.LDAPUIDAnnotation)
	}
	return true, nil
}

// NewOpenShiftGroupLister returns a new openshiftGroupLister that divulges the LDAP group unique identifier for
// each entry in the given whitelist of OpenShift Group names
func NewOpenShiftGroupLister(whitelist, blacklist []string, ldapURL string, client osclient.GroupInterface) interfaces.LDAPGroupLister {
	return &openshiftGroupLister{
		whitelist: whitelist,
		blacklist: sets.NewString(blacklist...),
		client:    client,
		ldapURL:   ldapURL,
	}
}

// openshiftGroupLister lists unique identifiers for LDAP lookup of all local OpenShift groups that have
// been given to it upon creation.
type openshiftGroupLister struct {
	whitelist []string
	blacklist sets.String

	client  osclient.GroupInterface
	ldapURL string
}

func (l *openshiftGroupLister) ListGroups() ([]string, error) {
	var groups []ouserapi.Group
	for _, name := range l.whitelist {
		fmt.Printf("looking at: %v\n", name)
		if !l.blacklist.Has(name) {
			fmt.Printf("getting at: %v\n", name)
			group, err := l.client.Get(name)
			if err != nil {
				return nil, err
			}
			fmt.Printf("adding %v\n", group)
			groups = append(groups, *group)
		}

	}

	var ldapGroupUIDs []string
	for _, group := range groups {
		matches, err := validateGroupAnnotations(l.ldapURL, group)
		if err != nil {
			return nil, err
		}
		if matches {
			ldapGroupUIDs = append(ldapGroupUIDs, group.Annotations[ldaputil.LDAPUIDAnnotation])
		} else {
			return nil, fmt.Errorf("group %q was not synchronized from: %s", group.Name, l.ldapURL)
		}
	}
	return ldapGroupUIDs, nil
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

	// iterate through to preserve ordering
	ret := []string{}
	for _, name := range allNames {
		if l.blacklist.Has(name) {
			continue
		}

		ret = append(ret, name)
	}

	return ret, nil
}

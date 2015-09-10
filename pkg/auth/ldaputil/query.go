package ldaputil

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

// LDAPQuery encodes an LDAP query
type LDAPQuery struct {
	// The DN of the branch of the directory where all searches should start from
	BaseDN string

	// The (optional) scope of the search. Defaults to the entire subtree if not set
	Scope Scope

	// The (optional) behavior of the search with regards to alisases. Defaults to always
	// dereferencing if not set
	DerefAliases DerefAliases

	// TimeLimit holds the limit of time in seconds that any request to the server can remain outstanding
	// before the wait for a response is given up. If this is 0, no client-side limit is imposed
	TimeLimit int

	// Filter is a valid LDAP search filter that retrieves all relevant entries from the LDAP server with the base DN
	Filter string
}

// NewSearchRequest creates a new search request for the LDAP query and optionally includes more attributes
func (q *LDAPQuery) NewSearchRequest(additionalAttributes []string) *ldap.SearchRequest {
	return ldap.NewSearchRequest(
		q.BaseDN,
		int(q.Scope),
		int(q.DerefAliases),
		0, // allowed return size - indicates no limit
		q.TimeLimit,
		false, // not types only
		q.Filter,
		additionalAttributes,
		nil, // no controls
	)
}

// LDAPQueryOnAttribute encodes an LDAP query that conjoins two filters to extract a specific LDAP entry
// This query is not self-sufficient and needs the value of the QueryAttribute to construct the final filter
type LDAPQueryOnAttribute struct {
	// Query retrieves entries from an LDAP server
	LDAPQuery

	// QueryAttribute is the attribute for a specific filter that, when conjoined with the common filter,
	// retrieves the specific LDAP entry from the LDAP server. (e.g. "cn", when formatted with "aGroupName"
	// and conjoined with "objectClass=groupOfNames", becomes (&(objectClass=groupOfNames)(cn=aGroupName))")
	QueryAttribute string
}

// IdentifiyingLDAPQueryOptions holds a query and the attribute that identifies the entries that the query returns
type IdentifiyingLDAPQueryOptions struct {
	// Query retrieves entries from an LDAP server
	LDAPQueryOnAttribute

	// NameAttributes defines the attributes for the LDAP entries returned by the Query that will be interpreted
	// as their names
	NameAttributes []string
}

// NewIdentifiyingLDAPQueryOptions converts a user-provided LDAPQuery into a version we can use by parsing
// the input and combining it with a set of name attributes
func NewIdentifiyingLDAPQueryOptions(config api.LDAPQuery,
	nameAttributes []string) (IdentifiyingLDAPQueryOptions, error) {

	scope, err := DetermineLDAPScope(config.Scope)
	if err != nil {
		return IdentifiyingLDAPQueryOptions{}, err
	}

	derefAliases, err := DetermineDerefAliasesBehavior(config.DerefAliases)
	if err != nil {
		return IdentifiyingLDAPQueryOptions{}, err
	}

	return IdentifiyingLDAPQueryOptions{
		LDAPQueryOnAttribute: LDAPQueryOnAttribute{
			LDAPQuery: LDAPQuery{
				BaseDN:       config.BaseDN,
				Scope:        scope,
				DerefAliases: derefAliases,
				TimeLimit:    config.TimeLimit,
				Filter:       config.Filter,
			},
			QueryAttribute: config.QueryAttribute,
		},
		NameAttributes: nameAttributes,
	}, nil
}

// NewSearchRequest creates a new search request from the identifying query by internalizing the value of
// the attribute to be filtered as well as any additional attributest that need to be recovereds
func (o *IdentifiyingLDAPQueryOptions) NewSearchRequest(attributeValue string,
	additionalAttributes []string) (*ldap.SearchRequest, error) {

	allAttributes := util.NewStringSet(o.NameAttributes...)
	allAttributes.Insert(additionalAttributes...)

	if o.QueryAttribute == "DN" || o.QueryAttribute == "dn" {
		if !strings.Contains(attributeValue, o.BaseDN) {
			return nil, fmt.Errorf("search for entry with %s=%s would search outside of the base dn specified (dn=%s)",
				o.QueryAttribute, attributeValue, o.BaseDN)
		}
		if _, err := ldap.ParseDN(attributeValue); err != nil {
			return nil, fmt.Errorf("could not search by dn, invalid dn value: %v", err)
		}
		return o.buildDNQuery(attributeValue, allAttributes.List()), nil
	} else {
		return o.buildAttributeQuery(attributeValue, allAttributes.List()), nil
	}
}

// buildDNQuery builds the query that finds an LDAP entry with the given DN
// this is done by setting the DN to be the base DN for the search and setting the search scope
// to only consider the base object found
func (o *IdentifiyingLDAPQueryOptions) buildDNQuery(dn string,
	attributes []string) *ldap.SearchRequest {
	return ldap.NewSearchRequest(
		dn,
		ldap.ScopeBaseObject, // over-ride original
		int(o.DerefAliases),
		0, // allowed return size - indicates no limit
		o.TimeLimit,
		false,           // not types only
		"objectClass=*", // filter that returns all values
		attributes,
		nil, // no controls
	)
}

// buildAttributeQuery builds the query containing a filter that conjoins the common filter given
// in the configuration with the specific attribute filter for which the attribute value is given
func (o *IdentifiyingLDAPQueryOptions) buildAttributeQuery(attributeValue string,
	attributes []string) *ldap.SearchRequest {
	specificFilter := fmt.Sprintf("%s=%s",
		ldap.EscapeFilter(o.QueryAttribute),
		ldap.EscapeFilter(attributeValue))

	filter := fmt.Sprintf("(&(%s)(%s))", o.Filter, specificFilter)

	return ldap.NewSearchRequest(
		o.BaseDN,
		int(o.Scope),
		int(o.DerefAliases),
		0, // allowed return size - indicates no limit
		o.TimeLimit,
		false, // not types only
		filter,
		attributes,
		nil, // no controls
	)
}

// QueryForUniqueEntry queries for an LDAP entry on an LDAP server determined from a ClientConfig
// by creating a search request from the requisite input. The query is expected to return one unqiue
// result. If this is not the case, errors are raised
func QueryForUniqueEntry(clientConfig LDAPClientConfig,
	query *ldap.SearchRequest) (entry *ldap.Entry, err error) {

	result, err := QueryForEntries(clientConfig, query)
	if err != nil {
		return nil, err
	}
	if len(result) > 1 {
		return nil, fmt.Errorf("multiple entries found matching %s", query.Filter)
	}
	entry = result[0]
	glog.V(4).Infof("found dn=%q for %s", entry.DN, query.Filter)
	return entry, nil
}

// QueryForEntries queries for LDAP entries on an LDAP server determined from a ClientConfig by
// creating a search request from the requisite input.
func QueryForEntries(clientConfig LDAPClientConfig,
	query *ldap.SearchRequest) (result []*ldap.Entry, err error) {

	connection, err := clientConfig.Connect()
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	glog.V(4).Infof("searching LDAP server for %s", query.Filter)
	searchResult, err := connection.Search(query)
	if err != nil {
		return nil, err
	}

	entries := searchResult.Entries
	// No entries returned from the LDAP search request means that the LDAP search was not configured
	// correctly. The search must return with at least one LDAP entry
	if len(entries) == 0 {
		return nil, fmt.Errorf("no LDAP user entry found for filter: %s", query.Filter)
	}

	for _, entry := range entries {
		glog.V(4).Infof("found dn=%q for %s", entry.DN, query.Filter)
	}
	return entries, nil
}

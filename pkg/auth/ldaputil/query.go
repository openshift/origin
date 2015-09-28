package ldaputil

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

// QueryOutOfBounds is an error that occurs when trying to search by DN for an entry that exists
// outside of the tree specified with the BaseDN for search.
type QueryOutOfBounds struct {
	BaseDN  string
	QueryDN string
}

// Error returns the error string for the out-of-bounds query
func (q *QueryOutOfBounds) Error() string {
	return fmt.Sprintf("search for entry with dn=%q would search outside of the base dn specified (dn=%q)",
		q.QueryDN, q.BaseDN)
}

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

// NewLDAPQueryOnAttribute converts a user-provided LDAPQuery into a version we can use by parsing
// the input and combining it with a set of name attributes
func NewLDAPQueryOnAttribute(config api.LDAPQuery) (LDAPQueryOnAttribute, error) {
	scope, err := DetermineLDAPScope(config.Scope)
	if err != nil {
		return LDAPQueryOnAttribute{}, err
	}

	derefAliases, err := DetermineDerefAliasesBehavior(config.DerefAliases)
	if err != nil {
		return LDAPQueryOnAttribute{}, err
	}

	return LDAPQueryOnAttribute{
		LDAPQuery: LDAPQuery{
			BaseDN:       config.BaseDN,
			Scope:        scope,
			DerefAliases: derefAliases,
			TimeLimit:    config.TimeLimit,
			Filter:       config.Filter,
		},
		QueryAttribute: config.QueryAttribute,
	}, nil
}

// NewSearchRequest creates a new search request from the identifying query by internalizing the value of
// the attribute to be filtered as well as any attributes that need to be recovered
func (o *LDAPQueryOnAttribute) NewSearchRequest(attributeValue string,
	attributes []string) (*ldap.SearchRequest, error) {

	if strings.EqualFold(o.QueryAttribute, "dn") {
		if !strings.Contains(attributeValue, o.BaseDN) {
			return nil, &QueryOutOfBounds{QueryDN: attributeValue, BaseDN: o.BaseDN}
		}
		if _, err := ldap.ParseDN(attributeValue); err != nil {
			return nil, fmt.Errorf("could not search by dn, invalid dn value: %v", err)
		}
		return o.buildDNQuery(attributeValue, attributes), nil
	} else {
		return o.buildAttributeQuery(attributeValue, attributes), nil
	}
}

// buildDNQuery builds the query that finds an LDAP entry with the given DN
// this is done by setting the DN to be the base DN for the search and setting the search scope
// to only consider the base object found
func (o *LDAPQueryOnAttribute) buildDNQuery(dn string, attributes []string) *ldap.SearchRequest {
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
func (o *LDAPQueryOnAttribute) buildAttributeQuery(attributeValue string,
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

// QueryForUniqueEntry queries for an LDAP entry with the given searchRequest. The query is expected
// to return one unqiue result. If this is not the case, errors are raised
func QueryForUniqueEntry(clientConfig LDAPClientConfig, query *ldap.SearchRequest) (*ldap.Entry, error) {
	result, err := QueryForEntries(clientConfig, query)
	if err != nil {
		return nil, err
	}
	if len(result) > 1 {
		if query.Scope == ldap.ScopeBaseObject {
			return nil, fmt.Errorf("multiple entries found matching dn=%q:\n%s",
				query.BaseDN, formatResult(result))
		} else {
			return nil, fmt.Errorf("multiple entries found matching filter %s:\n%s",
				query.Filter, formatResult(result))
		}
	}
	entry := result[0]
	glog.V(4).Infof("found dn=%q for %s", entry.DN, query.Filter)
	return entry, nil
}

// formatResult pretty-prints the first ten DNs in the slice of entries
func formatResult(results []*ldap.Entry) string {
	var names []string
	for _, entry := range results {
		names = append(names, entry.DN)
	}
	return "\t" + strings.Join(names[0:10], "\n\t")
}

// QueryForEntries queries for LDAP with the given searchRequest. The query is expected to return
// at least one unique result. If this is not the case, errors are raised
func QueryForEntries(clientConfig LDAPClientConfig, query *ldap.SearchRequest) ([]*ldap.Entry, error) {
	results, err := IssueLDAPQuery(clientConfig, query)
	if err != nil {
		return nil, err
	}

	// No entries returned from the LDAP search request means that the LDAP search was not configured
	// correctly. The search must return with at least one LDAP entry
	if len(results) == 0 {
		return nil, fmt.Errorf("no LDAP user entry found for filter: %s", query.Filter)
	}

	for _, entry := range results {
		glog.V(4).Infof("found dn=%q for %s", entry.DN, query.Filter)
	}
	return results, nil
}

// IssueLDAPQuery connects to an LDAP server and binds to it if credentials are given, then issues
// the given search request and returns the result or any errors encountered
func IssueLDAPQuery(clientConfig LDAPClientConfig, query *ldap.SearchRequest) ([]*ldap.Entry, error) {
	connection, err := clientConfig.Connect()
	if err != nil {
		return nil, fmt.Errorf("could not connect to the LDAP server: %v", err)
	}
	defer connection.Close()

	if _, err := clientConfig.Bind(connection); err != nil {
		return nil, fmt.Errorf("could not bind to the LDAP server: %v", err)
	}

	glog.V(4).Infof("searching LDAP server at dn=%q for %s", query.BaseDN, query.Filter)
	searchResult, err := connection.Search(query)
	if err != nil {
		return nil, fmt.Errorf("could not search the LDAP server: %v", err)
	}

	return searchResult.Entries, nil
}

package ldaputil

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

// NewLDAPQuery converts a user-provided LDAPQuery into a version we can use
func NewLDAPQuery(config api.LDAPQuery) (LDAPQuery, error) {
	scope, err := DetermineLDAPScope(config.Scope)
	if err != nil {
		return LDAPQuery{}, err
	}

	derefAliases, err := DetermineDerefAliasesBehavior(config.DerefAliases)
	if err != nil {
		return LDAPQuery{}, err
	}

	return LDAPQuery{
		BaseDN:       config.BaseDN,
		Scope:        scope,
		DerefAliases: derefAliases,
		TimeLimit:    config.TimeLimit,
		Filter:       config.Filter,
	}, nil
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

// NewLDAPQueryOnAttribute converts a user-provided LDAPQuery into a version we can use by parsing
// the input and combining it with a set of name attributes
func NewLDAPQueryOnAttribute(config api.LDAPQuery, attribute string) (LDAPQueryOnAttribute, error) {
	ldapQuery, err := NewLDAPQuery(config)
	if err != nil {
		return LDAPQueryOnAttribute{}, err
	}

	return LDAPQueryOnAttribute{
		LDAPQuery:      ldapQuery,
		QueryAttribute: attribute,
	}, nil
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

// NewSearchRequest creates a new search request from the identifying query by internalizing the value of
// the attribute to be filtered as well as any attributes that need to be recovered
func (o *LDAPQueryOnAttribute) NewSearchRequest(attributeValue string, attributes []string) (*ldap.SearchRequest, error) {
	if strings.EqualFold(o.QueryAttribute, "dn") {
		if _, err := ldap.ParseDN(attributeValue); err != nil {
			return nil, fmt.Errorf("could not search by dn, invalid dn value: %v", err)
		}
		if !strings.Contains(attributeValue, o.BaseDN) {
			return nil, NewQueryOutOfBoundsError(attributeValue, o.BaseDN)
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
		false,             // not types only
		"(objectClass=*)", // filter that returns all values
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
func QueryForUniqueEntry(clientConfig ldapclient.Config, query *ldap.SearchRequest) (*ldap.Entry, error) {
	result, err := QueryForEntries(clientConfig, query)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, NewEntryNotFoundError(query.BaseDN, query.Filter)
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

// QueryForEntries queries for LDAP with the given searchRequest
func QueryForEntries(clientConfig ldapclient.Config, query *ldap.SearchRequest) ([]*ldap.Entry, error) {
	connection, err := clientConfig.Connect()
	if err != nil {
		return nil, fmt.Errorf("could not connect to the LDAP server: %v", err)
	}
	defer connection.Close()

	if bindDN, bindPassword := clientConfig.GetBindCredentials(); len(bindDN) > 0 {
		if err := connection.Bind(bindDN, bindPassword); err != nil {
			return nil, fmt.Errorf("could not bind to the LDAP server: %v", err)
		}
	}

	glog.V(4).Infof("searching LDAP server with config %v with dn=%q and scope %v for %s requesting %v", clientConfig, query.BaseDN, query.Scope, query.Filter, query.Attributes)
	searchResult, err := connection.Search(query)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
			return nil, NewNoSuchObjectError(query.BaseDN)
		}
		return nil, err
	}

	for _, entry := range searchResult.Entries {
		glog.V(4).Infof("found dn=%q ", entry.DN)
	}
	return searchResult.Entries, nil
}

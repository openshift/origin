package ldaputil

import "fmt"

func NewNoSuchObjectError(baseDN string) error {
	return &errNoSuchObject{baseDN: baseDN}
}

// errNoSuchObject is an error that occurs when a base DN for a search refers to an object that does not exist
type errNoSuchObject struct {
	baseDN string
}

// Error returns the error string for the invalid base DN query error
func (e *errNoSuchObject) Error() string {
	return fmt.Sprintf("search for entry with base dn=%q refers to a non-existent entry", e.baseDN)
}

func IsNoSuchObjectError(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*errNoSuchObject)
	return ok
}

func NewEntryNotFoundError(baseDN, filter string) error {
	return &errEntryNotFound{baseDN: baseDN, filter: filter}
}

// errEntryNotFound is an error that occurs when trying to find a specific entry fails.
type errEntryNotFound struct {
	baseDN string
	filter string
}

// Error returns the error string for the entry not found error
func (e *errEntryNotFound) Error() string {
	return fmt.Sprintf("search for entry with base dn=%q and filter %q did not return any results", e.baseDN, e.filter)
}

func IsEntryNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*errEntryNotFound)
	return ok
}

func NewQueryOutOfBoundsError(queryDN, baseDN string) error {
	return &errQueryOutOfBounds{baseDN: baseDN, queryDN: queryDN}
}

// errQueryOutOfBounds is an error that occurs when trying to search by DN for an entry that exists
// outside of the tree specified with the BaseDN for search.
type errQueryOutOfBounds struct {
	baseDN  string
	queryDN string
}

// Error returns the error string for the out-of-bounds query
func (q *errQueryOutOfBounds) Error() string {
	return fmt.Sprintf("search for entry with dn=%q would search outside of the base dn specified (dn=%q)", q.queryDN, q.baseDN)
}

func IsQueryOutOfBoundsError(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*errQueryOutOfBounds)
	return ok
}

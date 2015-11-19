package util

import "github.com/vjeantet/ldapserver"

type testLDAPServer struct {
	// Passwords holds a map of DNs to valid passwords
	Passwords map[string]string

	// BindRequests holds submitted bind requests
	BindRequests []ldapserver.BindRequest
	// SearchRequests holds submitted search requests
	SearchRequests []ldapserver.SearchRequest

	// SearchResults holds results that will be returned from any search
	SearchResults []*ldapserver.SearchResultEntry

	// server is the internal Server
	server *ldapserver.Server
}

func NewTestLDAPServer() *testLDAPServer {
	t := &testLDAPServer{}

	// set up bind and search handlers
	routes := ldapserver.NewRouteMux()
	routes.Bind(t.handleBind)
	routes.Search(t.handleSearch)

	// new LDAP Server
	t.server = ldapserver.NewServer()
	t.server.Handle(routes)

	return t
}

func (t *testLDAPServer) Start(address string) {
	go t.server.ListenAndServe(address)
}

func (t *testLDAPServer) Stop() {
	t.server.Stop()
}

func (t *testLDAPServer) ResetRequests() {
	t.BindRequests = []ldapserver.BindRequest{}
	t.SearchRequests = []ldapserver.SearchRequest{}
}

func (t *testLDAPServer) AddSearchResult(dn string, attributes map[string]string) {
	e := ldapserver.NewSearchResultEntry()
	e.SetDn(dn)
	for k, v := range attributes {
		e.AddAttribute(ldapserver.AttributeDescription(k), ldapserver.AttributeValue(v))
	}
	t.SearchResults = append(t.SearchResults, e)
}

func (t *testLDAPServer) SetPassword(dn string, password string) {
	if t.Passwords == nil {
		t.Passwords = map[string]string{}
	}
	t.Passwords[dn] = password
}

func (t *testLDAPServer) handleBind(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetBindRequest()

	// Record the request
	t.BindRequests = append(t.BindRequests, r)

	dn := string(r.GetLogin())
	password := string(r.GetPassword())

	// Require a non-empty username and password
	if len(dn) == 0 || len(password) == 0 {
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultUnwillingToPerform))
		return
	}

	// Require the DN to be found and the password to match
	expectedPassword, ok := t.Passwords[dn]
	if !ok || expectedPassword != password {
		w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultInvalidCredentials))
		return
	}

	w.Write(ldapserver.NewBindResponse(ldapserver.LDAPResultSuccess))
}

func (t *testLDAPServer) handleSearch(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetSearchRequest()

	// Record the entry
	t.SearchRequests = append(t.SearchRequests, r)

	// Write the results
	for _, entry := range t.SearchResults {
		w.Write(entry)
	}
	w.Write(ldapserver.NewSearchResultDoneResponse(ldapserver.LDAPResultSuccess))
}

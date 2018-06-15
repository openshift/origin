package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	ldap "github.com/vjeantet/ldapserver"
)

func main() {
	//Create a new LDAP Server
	server := ldap.NewServer()

	//Create routes bindings
	routes := ldap.NewRouteMux()
	routes.NotFound(handleNotFound)
	routes.Extended(handleStartTLS).RequestName(ldap.NoticeOfStartTLS)
	routes.Abandon(handleAbandon)
	routes.Bind(handleBind)
	routes.Compare(handleCompare)
	routes.Add(handleAdd)
	routes.Delete(handleDelete)
	routes.Extended(handleWhoAmI).RequestName(ldap.NoticeOfWhoAmI)
	routes.Extended(handleExtended)
	routes.Modify(handleModify)
	routes.Search(handleSearchMyCompany).BaseDn("o=My Company, c=US")
	routes.Search(handleSearch)

	//Attache routes to server
	server.Handle(routes)

	// listen on 10389 and serve
	go server.ListenAndServe(":10389")

	// When CTRL+C, SIGINT and SIGTERM signal occurs
	// Then stop server gracefully
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	close(ch)

	server.Stop()
}

func handleNotFound(w ldap.ResponseWriter, r *ldap.Message) {
	switch r.GetProtocolOp().(type) {
	case ldap.BindRequest:
		res := ldap.NewBindResponse(ldap.LDAPResultSuccess)
		res.DiagnosticMessage = "Default binding behavior set to return Success"
		w.Write(res)

	default:
		res := ldap.NewResponse(ldap.LDAPResultUnwillingToPerform)
		res.DiagnosticMessage = "Operation not implemented by server"
		w.Write(res)
	}
}

func handleAbandon(w ldap.ResponseWriter, m *ldap.Message) {
	var req = m.GetAbandonRequest()
	messageIDToAbandon := req.GetIDToAbandon()
	// retreive the request to abandon, and send a abort signal to it
	if requestToAbandon, ok := m.Client.GetMessageByID(messageIDToAbandon); ok {
		requestToAbandon.Abandon()
		log.Printf("Abandon signal sent to request processor [messageID=%d]", messageIDToAbandon)
	}
}

func handleBind(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetBindRequest()
	res := ldap.NewBindResponse(ldap.LDAPResultSuccess)

	if string(r.GetLogin()) == "myLogin" {
		w.Write(res)
		return
	}

	log.Printf("Bind failed User=%s, Pass=%s", string(r.GetLogin()), string(r.GetPassword()))
	res.ResultCode = ldap.LDAPResultInvalidCredentials
	res.DiagnosticMessage = "invalid credentials"
	w.Write(res)
}

// The resultCode is set to compareTrue, compareFalse, or an appropriate
// error.  compareTrue indicates that the assertion value in the ava
// Comparerequest field matches a value of the attribute or subtype according to the
// attribute's EQUALITY matching rule.  compareFalse indicates that the
// assertion value in the ava field and the values of the attribute or
// subtype did not match.  Other result codes indicate either that the
// result of the comparison was Undefined, or that
// some error occurred.
func handleCompare(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetCompareRequest()
	log.Printf("Comparing entry: %s", r.GetEntry())
	//attributes values
	log.Printf(" attribute name to compare : \"%s\"", r.GetAttributeValueAssertion().GetName())
	log.Printf(" attribute value expected : \"%s\"", r.GetAttributeValueAssertion().GetValue())

	res := ldap.NewCompareResponse(ldap.LDAPResultCompareTrue)

	w.Write(res)
}

func handleAdd(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetAddRequest()
	log.Printf("Adding entry: %s", r.GetEntryDN())
	//attributes values
	for _, attribute := range r.GetAttributes() {
		for _, attributeValue := range attribute.GetValues() {
			log.Printf("- %s:%s", attribute.GetDescription(), attributeValue)
		}
	}
	res := ldap.NewAddResponse(ldap.LDAPResultSuccess)
	w.Write(res)
}

func handleModify(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetModifyRequest()
	log.Printf("Modify entry: %s", r.GetObject())

	for _, change := range r.GetChanges() {
		modification := change.GetModification()
		var operationString string
		switch change.GetOperation() {
		case ldap.ModifyRequestChangeOperationAdd:
			operationString = "Add"
		case ldap.ModifyRequestChangeOperationDelete:
			operationString = "Delete"
		case ldap.ModifyRequestChangeOperationReplace:
			operationString = "Replace"
		}

		log.Printf("%s attribute '%s'", operationString, modification.GetDescription())
		for _, attributeValue := range modification.GetValues() {
			log.Printf("- value: %s", attributeValue)
		}

	}

	res := ldap.NewModifyResponse(ldap.LDAPResultSuccess)
	w.Write(res)
}

func handleDelete(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetDeleteRequest()
	log.Printf("Deleting entry: %s", r.GetEntryDN())
	res := ldap.NewDeleteResponse(ldap.LDAPResultSuccess)
	w.Write(res)
}

func handleExtended(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetExtendedRequest()
	log.Printf("Extended request received, name=%s", r.GetResponseName())
	log.Printf("Extended request received, value=%x", r.GetResponseValue())
	res := ldap.NewExtendedResponse(ldap.LDAPResultSuccess)
	w.Write(res)
}

func handleWhoAmI(w ldap.ResponseWriter, m *ldap.Message) {
	res := ldap.NewExtendedResponse(ldap.LDAPResultSuccess)
	w.Write(res)
}

func handleSearchMyCompany(w ldap.ResponseWriter, m *ldap.Message) {
	res := ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess)
	w.Write(res)
}

func handleSearch(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetSearchRequest()
	log.Printf("Request BaseDn=%s", r.GetBaseObject())
	log.Printf("Request Filter=%s", r.GetFilter())
	log.Printf("Request Attributes=%s", r.GetAttributes())

	// Handle Stop Signal (server stop / client disconnected / Abandoned request....)
	select {
	case <-m.Done:
		log.Print("Leaving handleSearch...")
		return
	default:
	}

	e := ldap.NewSearchResultEntry()
	e.SetDn("cn=Valere JEANTET, " + string(r.GetBaseObject()))
	e.AddAttribute("mail", "valere.jeantet@gmail.com", "mail@vjeantet.fr")
	e.AddAttribute("company", "SODADI")
	e.AddAttribute("department", "DSI/SEC")
	e.AddAttribute("l", "Ferrieres en brie")
	e.AddAttribute("mobile", "0612324567")
	e.AddAttribute("telephoneNumber", "0612324567")
	e.AddAttribute("cn", "Valère JEANTET")
	w.Write(e)

	e = ldap.NewSearchResultEntry()
	e.SetDn("cn=Claire Thomas, " + string(r.GetBaseObject()))
	e.AddAttribute("mail", "claire.thomas@gmail.com")
	e.AddAttribute("cn", "Claire THOMAS")
	w.Write(e)

	res := ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess)
	w.Write(res)

}

// localhostCert is a PEM-encoded TLS cert with SAN DNS names
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBOTCB5qADAgECAgEAMAsGCSqGSIb3DQEBBTAAMB4XDTcwMDEwMTAwMDAwMFoX
DTQ5MTIzMTIzNTk1OVowADBaMAsGCSqGSIb3DQEBAQNLADBIAkEAsuA5mAFMj6Q7
qoBzcvKzIq4kzuT5epSp2AkcQfyBHm7K13Ws7u+0b5Vb9gqTf5cAiIKcrtrXVqkL
8i1UQF6AzwIDAQABo08wTTAOBgNVHQ8BAf8EBAMCACQwDQYDVR0OBAYEBAECAwQw
DwYDVR0jBAgwBoAEAQIDBDAbBgNVHREEFDASggkxMjcuMC4wLjGCBVs6OjFdMAsG
CSqGSIb3DQEBBQNBAJH30zjLWRztrWpOCgJL8RQWLaKzhK79pVhAx6q/3NrF16C7
+l1BRZstTwIGdoGId8BRpErK1TXkniFb95ZMynM=
-----END CERTIFICATE-----
`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPQIBAAJBALLgOZgBTI+kO6qAc3LysyKuJM7k+XqUqdgJHEH8gR5uytd1rO7v
tG+VW/YKk3+XAIiCnK7a11apC/ItVEBegM8CAwEAAQJBAI5sxq7naeR9ahyqRkJi
SIv2iMxLuPEHaezf5CYOPWjSjBPyVhyRevkhtqEjF/WkgL7C2nWpYHsUcBDBQVF0
3KECIQDtEGB2ulnkZAahl3WuJziXGLB+p8Wgx7wzSM6bHu1c6QIhAMEp++CaS+SJ
/TrU0zwY/fW4SvQeb49BPZUF3oqR8Xz3AiEA1rAJHBzBgdOQKdE3ksMUPcnvNJSN
poCcELmz2clVXtkCIQCLytuLV38XHToTipR4yMl6O+6arzAjZ56uq7m7ZRV0TwIh
AM65XAOw8Dsg9Kq78aYXiOEDc5DL0sbFUu/SlmRcCg93
-----END RSA PRIVATE KEY-----
`)

// getTLSconfig returns a tls configuration used
// to build a TLSlistener for TLS or StartTLS
func getTLSconfig() (*tls.Config, error) {
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		return &tls.Config{}, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionSSL30,
		MaxVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ServerName:   "127.0.0.1",
	}, nil
}

func handleStartTLS(w ldap.ResponseWriter, m *ldap.Message) {
	tlsconfig, _ := getTLSconfig()
	tlsConn := tls.Server(m.Client.GetConn(), tlsconfig)
	res := ldap.NewExtendedResponse(ldap.LDAPResultSuccess)
	res.ResponseName = ldap.NoticeOfStartTLS
	w.Write(res)

	if err := tlsConn.Handshake(); err != nil {
		log.Printf("StartTLS Handshake error %v", err)
		res.DiagnosticMessage = fmt.Sprintf("StartTLS Handshake error : \"%s\"", err.Error())
		res.ResultCode = ldap.LDAPResultOperationsError
		w.Write(res)
		return
	}

	m.Client.SetConn(tlsConn)
	log.Println("StartTLS OK")
}

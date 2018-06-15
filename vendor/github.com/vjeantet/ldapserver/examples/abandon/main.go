package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	ldap "github.com/vjeantet/ldapserver"
)

func main() {
	//Create a new LDAP Server
	server := ldap.NewServer()
	// server.ReadTimeout = time.Millisecond * 100
	// server.WriteTimeout = time.Millisecond * 100
	routes := ldap.NewRouteMux()
	routes.Bind(handleBind)
	routes.Search(handleSearch)
	server.Handle(routes)

	// listen on 10389
	go server.ListenAndServe("127.0.0.1:10389")

	// When CTRL+C, SIGINT and SIGTERM signal occurs
	// Then stop server gracefully
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	close(ch)

	server.Stop()
}

func handleSearch(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetSearchRequest()
	log.Printf("Request BaseDn=%s", r.GetBaseObject())
	log.Printf("Request Filter=%s", r.GetFilter())
	log.Printf("Request Attributes=%s", r.GetAttributes())

	// Handle Stop Signal (server stop / client disconnected / Abandoned request....)
	for {
		select {
		case <-m.Done:
			log.Printf("Leaving handleSearch... for msgid=%d", m.MessageID)
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
		time.Sleep(time.Millisecond * 800)
	}
	// e = ldap.NewSearchResultEntry()
	// e.SetDn("cn=Claire Thomas, " + string(r.GetBaseObject()))
	// e.AddAttribute("mail", "claire.thomas@gmail.com")
	// e.AddAttribute("cn", "Claire THOMAS")
	// w.Write(e)

	res := ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess)
	w.Write(res)

}

// handleBind return Success if login == mysql
func handleBind(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetBindRequest()
	res := ldap.NewBindResponse(ldap.LDAPResultSuccess)

	if string(r.GetLogin()) == "login" {
		w.Write(res)
		return
	}

	log.Printf("Bind failed User=%s, Pass=%s", string(r.GetLogin()), string(r.GetPassword()))
	res.ResultCode = ldap.LDAPResultInvalidCredentials
	res.DiagnosticMessage = "invalid credentials"
	w.Write(res)
}

package main

import (
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"syscall"

	ldap "github.com/vjeantet/ldapserver"
)

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

func main() {
	//Create a new LDAP Server
	server := ldap.NewServer()

	//Set routes, here, we only serve bindRequest
	routes := ldap.NewRouteMux()
	routes.Bind(handleBind)
	server.Handle(routes)

	//SSL
	secureConn := func(s *ldap.Server) {
		config, _ := getTLSconfig()
		s.Listener = tls.NewListener(s.Listener, config)
	}
	go server.ListenAndServe(":10636", secureConn)

	// When CTRL+C, SIGINT and SIGTERM signal occurs
	// Then stop server gracefully
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-ch
	close(ch)
	server.Stop()
}

// handleBind return Success if login == mysql
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

[![GoDoc](https://godoc.org/github.com/vjeantet/ldapserver?status.svg)](https://godoc.org/github.com/vjeantet/ldapserver)
[![Build Status](https://travis-ci.org/vjeantet/ldapserver.svg)](https://travis-ci.org/vjeantet/ldapserver)

**This package is a work in progress.**

I'm co-learning golang with [R00x](https://github.com/lor00x). Have a look at his implementation: https://github.com/lor00x/goldap

**ldapserver** is a helper library for building server software capable of speaking the LDAP protocol. This could be an alternate implementation of LDAP, a custom LDAP proxy or even a completely different backend capable of "masquerading" its API as a LDAP Server.

The package supports 
* All basic LDAP Operations (bind, search, add, compare, modify, delete, extended)
* SSL
* StartTLS
* Unbind request is implemented, but is handled internally to close the connection.
* Graceful stopping
* Basic request routing inspired by [net/http ServeMux](http://golang.org/pkg/net/http/#ServeMux)

# Default behavior
When no route matches the request, the server will first try to call a special *NotFound* route, if nothing is specified, it will return an *UnwillingToResponse* Error code (53)

Feel free to contribute, comment :)

#  Sample Code
```Go
// Listen to 10389 port for LDAP Request
// and route bind request to the handleBind func
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	ldap "github.com/vjeantet/ldapserver"
)

func main() {
	//LDAP handlers routes
	routes := ldap.NewRouteMux()
	routes.Bind(handleBind)

	//Create a new LDAP Server
	server := ldap.NewServer()
	server.Handle(routes)

	// listen on 10389
	go server.ListenAndServe(":10389")

	// When CTRL+C, SIGINT and SIGTERM signal occurs
	// Then stop server gracefully
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	close(ch)
	server.Stop()
}

func handleBind(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetBindRequest()
	res := ldap.NewBindResponse(ldap.LDAPResultSuccess)

	if string(r.GetLogin()) == "myLogin" {
		w.Write(res)
		return
	}

	res.ResultCode = ldap.LDAPResultInvalidCredentials
	w.Write(res)
}
```

# more examples
Look into the "examples" folder

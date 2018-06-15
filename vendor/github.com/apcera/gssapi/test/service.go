// Copyright 2013-2015 Apcera Inc. All rights reserved.

package test

import (
	"fmt"
	"net/http"
	"os"

	"github.com/apcera/gssapi"
)

type loggingHandler struct {
	*Context
	handler func(*Context, http.ResponseWriter, *http.Request) (code int, message string)
}

func (h loggingHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	code, message := h.handler(h.Context, rw, r)

	severity := gssapi.Info
	if code != http.StatusOK {
		severity = gssapi.Err
		rw.WriteHeader(code)
	}
	h.Print(severity, fmt.Sprintf(
		"%d %q %q %q", code, r.Method, r.URL.String(), message))
}

func Service(c *Context) error {
	if c.ServiceName == "" {
		return fmt.Errorf("Must provide a non-empty value for --service-name")
	}
	c.Debug(fmt.Sprintf("Starting service %q", c.ServiceName))

	nameBuf, err := c.MakeBufferString(c.ServiceName)
	if err != nil {
		return err
	}
	defer nameBuf.Release()

	name, err := nameBuf.Name(c.GSS_KRB5_NT_PRINCIPAL_NAME)
	if err != nil {
		return err
	}
	defer name.Release()

	cred, actualMechs, _, err := c.AcquireCred(name,
		gssapi.GSS_C_INDEFINITE, c.GSS_C_NO_OID_SET, gssapi.GSS_C_ACCEPT)
	actualMechs.Release()
	if err != nil {
		return err
	}
	c.credential = cred

	keytab := os.Getenv("KRB5_KTNAME")
	if keytab == "" {
		keytab = "default /etc/krb5.keytab"
	}
	c.Debug(fmt.Sprintf("Acquired credentials using %v", keytab))

	http.Handle("/access/", loggingHandler{c, HandleAccess})
	http.Handle("/verify_mic/", loggingHandler{c, HandleVerifyMIC})
	http.Handle("/unwrap/", loggingHandler{c, HandleUnwrap})
	http.Handle("/inquire_context/", loggingHandler{c, HandleInquireContext})

	err = http.ListenAndServe(c.ServiceAddress, nil)
	if err != nil {
		return err
	}

	// this isn't executed since the entire container is killed, but for
	// illustration purposes
	c.credential.Release()

	return nil
}

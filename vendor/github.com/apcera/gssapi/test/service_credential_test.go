// Copyright 2013-2015 Apcera Inc. All rights reserved.

//+build servicetest

package test

// test the credentials APIs with a keytab, configured against a real KDC

import (
	"strings"
	"testing"
	"time"

	"github.com/apcera/gssapi"
)

func TestAcquireCredential(t *testing.T) {
	name := prepareServiceName(t)
	defer name.Release()
	if name.String() != c.ServiceName {
		t.Fatalf("name: got %q, expected %q", name.String(), c.ServiceName)
	}

	mechs, err := c.MakeOIDSet(c.GSS_MECH_KRB5)
	if err != nil {
		t.Fatal(err)
	}
	defer mechs.Release()

	cred, actualMechs, timeRec, err := c.AcquireCred(name,
		gssapi.GSS_C_INDEFINITE, mechs, gssapi.GSS_C_ACCEPT)
	defer cred.Release()
	defer actualMechs.Release()
	verifyCred(t, cred, actualMechs, timeRec, err)
}

func TestAddCredential(t *testing.T) {
	name := prepareServiceName(t)
	defer name.Release()
	if name.String() != c.ServiceName {
		t.Fatalf("name: got %q, expected %q", name.String(), c.ServiceName)
	}

	mechs, err := c.MakeOIDSet(c.GSS_MECH_KRB5)
	if err != nil {
		t.Fatal(err)
	}
	defer mechs.Release()

	cred := c.NewCredId()
	cred, actualMechs, _, acceptorTimeRec, err := c.AddCred(
		cred, name, c.GSS_MECH_KRB5, gssapi.GSS_C_ACCEPT,
		gssapi.GSS_C_INDEFINITE, gssapi.GSS_C_INDEFINITE)
	defer cred.Release()
	defer actualMechs.Release()
	verifyCred(t, cred, actualMechs, acceptorTimeRec, err)
}

func verifyCred(t *testing.T, cred *gssapi.CredId,
	actualMechs *gssapi.OIDSet, timeRec time.Duration, err error) {

	if err != nil {
		t.Fatal(err)
	}
	if cred == nil {
		t.Fatal("Got nil cred, expected non-nil")
	}
	if actualMechs == nil {
		t.Fatal("Got nil actualMechs, expected non-nil")
	}
	contains, _ := actualMechs.TestOIDSetMember(c.GSS_MECH_KRB5)
	if !contains {
		t.Fatalf("Expected mechs to contain %q, got %q",
			c.GSS_MECH_KRB5.DebugString(),
			actualMechs.DebugString)
	}
	name, lifetime, credUsage, _, err := c.InquireCred(cred)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(name.String(), "@")
	if len(parts) != 2 || parts[0] != c.ServiceName {
		t.Fatalf("name: got %q, expected %q", name.String(), c.ServiceName+"@<domain>")
	}
	if credUsage != gssapi.GSS_C_ACCEPT {
		t.Fatalf("credUsage: got %v, expected gssapi.GSS_C_ACCEPT", credUsage)
	}
	if timeRec != lifetime {
		t.Fatalf("timeRec:%v != lifetime:%v", timeRec, lifetime)
	}
}

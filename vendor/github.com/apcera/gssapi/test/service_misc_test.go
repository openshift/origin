// Copyright 2013-2015 Apcera Inc. All rights reserved.

//+build servicetest

package test

import (
	"testing"

	"github.com/apcera/gssapi"
)

func TestIndicateMechs(t *testing.T) {
	expectedMechs := []*gssapi.OID{
		c.GSS_MECH_KRB5,
		// c.GSS_MECH_KRB5_OLD,
		// c.GSS_MECH_KRB5_LEGACY,
		// c.GSS_MECH_IAKERB,
		c.GSS_MECH_SPNEGO,
	}
	mechs, err := c.IndicateMechs()
	if err != nil {
		t.Fatal(err)
	}
	defer mechs.Release()

	for _, oid := range expectedMechs {
		if !mechs.Contains(oid) {
			t.Errorf("Expected to find %s in mechs %s", oid.DebugString(), mechs.DebugString())
		}
	}
}

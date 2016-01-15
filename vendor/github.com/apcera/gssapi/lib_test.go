// Copyright 2013-2015 Apcera Inc. All rights reserved.

package gssapi

import (
	"fmt"
	"log"
	"os"
	"testing"
)

func testLoad() (lib *Lib, err error) {
	pp := make([]Printer, 0, MaxSeverity)
	for i := Severity(0); i < MaxSeverity; i++ {
		pp = append(pp, log.New(os.Stderr,
			fmt.Sprintf("%s gssapi-test:\t", i),
			log.LstdFlags))
	}
	return Load(&Options{
		Printers: pp,
	})
}

func TestLoadLib(t *testing.T) {
	l, err := testLoad()
	if err != nil {
		t.Fatal(err)
	}

	if l.Fp_gss_export_name == nil {
		t.Error("Fp_gss_export_name did not get initialized")
		return
	}

	// TODO: maybe use reflect to enumerate all Fp's

	defer l.Unload()
}

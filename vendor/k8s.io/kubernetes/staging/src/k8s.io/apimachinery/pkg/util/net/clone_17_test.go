// +build !go1.8

package net

import (
	"crypto/tls"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestCloneTLSConfig(t *testing.T) {
	expected := sets.NewString(
		// These fields are copied in CloneTLSConfig
		"Rand",
		"Time",
		"Certificates",
		"RootCAs",
		"NextProtos",
		"ServerName",
		"InsecureSkipVerify",
		"CipherSuites",
		"PreferServerCipherSuites",
		"MinVersion",
		"MaxVersion",
		"CurvePreferences",
		"NameToCertificate",
		"GetCertificate",
		"ClientAuth",
		"ClientCAs",
		"ClientSessionCache",

		// These fields are not copied
		"SessionTicketsDisabled",
		"SessionTicketKey",

		// These fields are unexported
		"serverInitOnce",
		"mutex",
		"sessionTicketKeys",

		// go1.7 See #33936
		"DynamicRecordSizingDisabled",
		"Renegotiation",
	)

	fields := sets.NewString()
	structType := reflect.TypeOf(tls.Config{})
	for i := 0; i < structType.NumField(); i++ {
		fields.Insert(structType.Field(i).Name)
	}

	if missing := expected.Difference(fields); len(missing) > 0 {
		t.Errorf("Expected fields that were not seen in http.Transport: %v", missing.List())
	}
	if extra := fields.Difference(expected); len(extra) > 0 {
		t.Errorf("New fields seen in http.Transport: %v\nAdd to CopyClientTLSConfig if client-relevant, then add to expected list in TestCopyClientTLSConfig", extra.List())
	}
}


package flagtypes

import (
	"testing"
)

func TestAddr(t *testing.T) {
	if a := (Addr{Value: "somehost:90"}); a.Provided {
		t.Errorf("bypassing Set should not result in a provided value: %#v", a)
	}
	if a := (Addr{Value: "somehost:90"}).Default(); a.Provided {
		t.Errorf("Default() should not result in a provided value: %#v", a)
	}
	if a := (Addr{Value: "somehost:90"}).Default(); a.Port != 90 {
		t.Errorf("port is incorrect: %#v", a)
	}
	if a := (Addr{Value: "somehost:90"}).Default(); a.Host != "somehost" {
		t.Errorf("host is incorrect: %#v", a)
	}
	if a := (Addr{Value: "somehost:90"}).Default(); a.URL.Host != "somehost:90" {
		t.Errorf("host is incorrect: %#v", a)
	}
	if a := (Addr{Value: "http://somehost:90/var"}).Default(); a.URL.Path != "" {
		t.Errorf("path is incorrect: %#v", a)
	}
	if a := (Addr{Value: "http://somehost:90/var", AllowPrefix: true}).Default(); a.URL.Path != "/var" {
		t.Errorf("path is incorrect: %#v", a)
	}
	if a := (Addr{Value: "somehost:90", DefaultScheme: "https"}).Default(); a.URL.Scheme != "https" {
		t.Errorf("scheme is incorrect: %#v", a)
	}
	if a := (Addr{Value: "http://somehost/var", DefaultPort: 100}).Default(); a.Port != 80 {
		t.Errorf("port is incorrect: %#v", a)
	}
	if a := (Addr{Value: "somehost:90", DefaultScheme: "https"}).Default(); a.Port != 90 {
		t.Errorf("port is incorrect: %#v", a)
	}
	if a := (Addr{Value: "somehost", DefaultScheme: "https"}).Default(); a.Port != 443 {
		t.Errorf("port is incorrect: %#v", a)
	}
	if err := (&Addr{DefaultScheme: "unknown"}).Set("somehost"); err == nil {
		t.Errorf("unexpected non-error")
	}
	if a := (Addr{Value: "somehost", DefaultScheme: "unknown", DefaultPort: 10}).Default(); a.Port != 10 {
		t.Errorf("port is incorrect: %#v", a)
	}
	addr := &Addr{DefaultPort: 10}
	if err := addr.Set("somehost"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !addr.Provided || addr.URL == nil || addr.Host == "" || addr.Port != 10 {
		t.Errorf("value was provided: %#v", addr)
	}
}

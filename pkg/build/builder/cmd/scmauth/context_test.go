package scmauth

import (
	"net/url"
	"testing"
)

func TestContextSet(t *testing.T) {
	c := NewDefaultSCMContext()
	err := c.Set("VAR1", "value1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	c.Set("VAR2", "value2")
	if v, _ := c.Get("VAR1"); v != "value1" {
		t.Errorf("unexpected value")
	}
	if v, _ := c.Get("VAR2"); v != "value2" {
		t.Errorf("unexpected value")
	}
	if len(c.Env()) != 2 {
		t.Errorf("unexpected length of Env")
	}
}

func TestContextSetExisting(t *testing.T) {
	c := NewDefaultSCMContext()
	err := c.Set("VAR1", "value1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Setting the same value should be ok
	err = c.Set("VAR1", "value1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Setting a different value should return an error
	err = c.Set("VAR1", "value2")
	if err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestSetOverrideURL(t *testing.T) {
	c := NewDefaultSCMContext()
	u, _ := url.Parse("https://my.override.url/test/repo")
	err := c.SetOverrideURL(u)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	u2 := c.OverrideURL()
	if u.String() != u2.String() {
		t.Errorf("did not get the same URL: %v", u2)
	}
}

func TestSetOverrideURLExisting(t *testing.T) {
	c := NewDefaultSCMContext()
	urlStr := "https://my.override.url/test/repo"
	u, _ := url.Parse(urlStr)
	err := c.SetOverrideURL(u)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	sameURL, _ := url.Parse(urlStr)
	err = c.SetOverrideURL(sameURL)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	differentURL, _ := url.Parse("https://another.url/different/path")
	err = c.SetOverrideURL(differentURL)
	if err == nil {
		t.Errorf("did not get expected error")
	}
}

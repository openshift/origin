package dockercfg

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestReadDockercfg(t *testing.T) {
	content := "{\"test-server-1\":{\"auth\":\"my-auth\",\"email\":\"test@email.test.com\"}}"
	tempfile, err := ioutil.TempFile("", "cfgtest")
	if err != nil {
		t.Fatalf("Unable to create temp file: %v", err)
	}
	defer os.Remove(tempfile.Name())
	tempfile.WriteString(content)
	tempfile.Close()

	dockercfg, err := readDockercfg(tempfile.Name())
	if err != nil {
		t.Errorf("Received unexpected error reading dockercfg: %v", err)
		return
	}

	auth, ok := dockercfg["test-server-1"]
	if !ok {
		t.Errorf("Expected entry test-server-1 not found in dockercfg")
		return
	}
	if auth.Auth != "my-auth" {
		t.Errorf("Unexpected Auth value: %s", auth.Auth)
	}
	if auth.Email != "test@email.test.com" {
		t.Errorf("Unexpected Email value: %s", auth.Email)
	}
}

func TestGetCredentials(t *testing.T) {
	testStr := "dGVzdDpwYXNzd29yZA==" // test:password
	uname, pass, err := getCredentials(testStr)
	if err != nil {
		t.Errorf("Unexpected error getting credentials: %v", err)
	}
	if uname != "test" && pass != "password" {
		t.Errorf("Unexpected username and password: %s,%s", uname, pass)
	}
}

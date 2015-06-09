package dockercfg

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/credentialprovider"
)

func TestReadDockercfg(t *testing.T) {
	content := "{\"test-server-1.tld\":{\"auth\":\"Zm9vOmJhcgo=\",\"email\":\"test@email.test.com\"}}"
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

	keyring := credentialprovider.BasicDockerKeyring{}
	keyring.Add(dockercfg)
	authConfs, found := keyring.Lookup("test-server-1.tld/foo/bar")
	if !found || len(authConfs) == 0 {
		t.Errorf("Expected lookup success, got not found")
	}
	if authConfs[0].Email != "test@email.test.com" {
		t.Errorf("Unexpected Email value: %s", authConfs[0].Email)
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

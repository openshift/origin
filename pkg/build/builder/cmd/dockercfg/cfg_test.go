package dockercfg

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/kubernetes/pkg/credentialprovider"
)

func TestGetDockerAuth(t *testing.T) {
	var (
		configJsonFileName = "config.json"
		testEnvKey         = "TMP_PULL_DOCKER_CFG_AUTH_ENV_FOO_BAR"
	)
	var fileInfo *os.File

	content := "{ \"auths\": { \"test-server-1.tld\":{\"auth\":\"Zm9vOmJhcgo=\",\"email\":\"test@email.test.com\"}}}"

	tmpDirPath, err := ioutil.TempDir("", "test_foo_bar_")
	if err != nil {
		t.Fatalf("Creating tmp dir fail: %v", err)
		return
	}
	defer os.RemoveAll(tmpDirPath)

	absDockerConfigFileLocation, err := filepath.Abs(filepath.Join(tmpDirPath, configJsonFileName))
	if err != nil {
		t.Fatalf("while trying to canonicalize %s: %v", tmpDirPath, err)
		return
	}

	if _, err = os.Stat(absDockerConfigFileLocation); os.IsNotExist(err) {
		//create test cfg file
		fileInfo, err = os.OpenFile(absDockerConfigFileLocation, os.O_CREATE|os.O_RDWR, 0664)
		if err != nil {
			t.Fatalf("while trying to create file %s: %v", absDockerConfigFileLocation, err)
			return
		}
		defer fileInfo.Close()

		os.Setenv(testEnvKey, tmpDirPath)
		defer os.Unsetenv(testEnvKey)
	}

	fileInfo.WriteString(content)

	_, ok := NewHelper().GetDockerAuth("test-server-1.tld/foo/bar", testEnvKey)
	if !ok {
		t.Errorf("unexpected value getting docker auth fail")
		return
	}
}

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

func TestReadDockerConfigJson(t *testing.T) {
	content := "{ \"auths\": { \"test-server-1.tld\":{\"auth\":\"Zm9vOmJhcgo=\",\"email\":\"test@email.test.com\"}}}"
	tempfile, err := ioutil.TempFile("", "cfgtest")
	if err != nil {
		t.Fatalf("Unable to create temp file: %v", err)
	}
	defer os.Remove(tempfile.Name())
	tempfile.WriteString(content)
	tempfile.Close()

	dockercfg, err := readDockerConfigJson(tempfile.Name())
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

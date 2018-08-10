package daemon

import "testing"
import (
	"github.com/containers/image/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"path/filepath"
)

func TestDockerClientFromNilSystemContext(t *testing.T) {
	client, err := newDockerClient(nil)

	assert.Nil(t, err, "There should be no error creating the Docker client")
	assert.NotNil(t, client, "A Docker client reference should have been returned")

	assert.Equal(t, dockerclient.DefaultDockerHost, client.DaemonHost(), "The default docker host should have been used")
	assert.Equal(t, defaultAPIVersion, client.ClientVersion(), "The default api version should have been used")
}

func TestDockerClientFromCertContext(t *testing.T) {
	testDir := testDir(t)

	host := "tcp://127.0.0.1:2376"
	systemCtx := &types.SystemContext{
		DockerDaemonCertPath:              filepath.Join(testDir, "testdata", "certs"),
		DockerDaemonHost:                  host,
		DockerDaemonInsecureSkipTLSVerify: true,
	}

	client, err := newDockerClient(systemCtx)

	assert.Nil(t, err, "There should be no error creating the Docker client")
	assert.NotNil(t, client, "A Docker client reference should have been returned")

	assert.Equal(t, host, client.DaemonHost())
	assert.Equal(t, "1.22", client.ClientVersion())
}

func TestTlsConfigFromInvalidCertPath(t *testing.T) {
	ctx := &types.SystemContext{
		DockerDaemonCertPath: "/foo/bar",
	}

	_, err := tlsConfig(ctx)

	if assert.Error(t, err, "An error was expected") {
		assert.Regexp(t, "could not read CA certificate", err.Error())
	}
}

func TestTlsConfigFromCertPath(t *testing.T) {
	testDir := testDir(t)

	ctx := &types.SystemContext{
		DockerDaemonCertPath:              filepath.Join(testDir, "testdata", "certs"),
		DockerDaemonInsecureSkipTLSVerify: true,
	}

	httpClient, err := tlsConfig(ctx)

	assert.NoError(t, err, "There should be no error creating the HTTP client")

	tlsConfig := httpClient.Transport.(*http.Transport).TLSClientConfig
	assert.True(t, tlsConfig.InsecureSkipVerify, "TLS verification should be skipped")
	assert.Len(t, tlsConfig.Certificates, 1, "There should be one certificate")
}

func TestSkipTLSVerifyOnly(t *testing.T) {
	//testDir := testDir(t)

	ctx := &types.SystemContext{
		DockerDaemonInsecureSkipTLSVerify: true,
	}

	httpClient, err := tlsConfig(ctx)

	assert.NoError(t, err, "There should be no error creating the HTTP client")

	tlsConfig := httpClient.Transport.(*http.Transport).TLSClientConfig
	assert.True(t, tlsConfig.InsecureSkipVerify, "TLS verification should be skipped")
	assert.Len(t, tlsConfig.Certificates, 0, "There should be no certificate")
}

func TestSpecifyPlainHTTPViaHostScheme(t *testing.T) {
	host := "http://127.0.0.1:2376"
	ctx := &types.SystemContext{
		DockerDaemonHost: host,
	}

	client, err := newDockerClient(ctx)

	assert.Nil(t, err, "There should be no error creating the Docker client")
	assert.NotNil(t, client, "A Docker client reference should have been returned")

	assert.Equal(t, host, client.DaemonHost())
}

func testDir(t *testing.T) string {
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatal("Unable to determine the current test directory")
	}
	return testDir
}

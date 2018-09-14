package layout

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/containers/image/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const RemoteLayerContent = "This is the remote layer content"

var httpServerAddr string

func TestMain(m *testing.M) {
	httpServer, err := startRemoteLayerServer()
	if err != nil {
		println("Error starting test TLS server", err.Error())
		os.Exit(1)
	}

	httpServerAddr = strings.Replace(httpServer.URL, "127.0.0.1", "localhost", 1)
	code := m.Run()
	httpServer.Close()
	os.Exit(code)
}

func TestGetBlobForRemoteLayers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello world")
	}))
	defer ts.Close()

	imageSource := createImageSource(t, &types.SystemContext{})
	layerInfo := types.BlobInfo{
		Digest: digest.FromBytes([]byte("Hello world")),
		Size:   -1,
		URLs: []string{
			"brokenurl",
			ts.URL,
		},
	}

	reader, _, err := imageSource.GetBlob(context.Background(), layerInfo)
	require.NoError(t, err)
	defer reader.Close()

	data, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Hello world")
}

func TestGetBlobForRemoteLayersWithTLS(t *testing.T) {
	imageSource := createImageSource(t, &types.SystemContext{
		OCICertPath: "fixtures/accepted_certs",
	})

	layer, size, err := imageSource.GetBlob(context.Background(), types.BlobInfo{
		URLs: []string{httpServerAddr},
	})
	require.NoError(t, err)

	layerContent, _ := ioutil.ReadAll(layer)
	assert.Equal(t, RemoteLayerContent, string(layerContent))
	assert.Equal(t, int64(len(RemoteLayerContent)), size)
}

func TestGetBlobForRemoteLayersOnTLSFailure(t *testing.T) {
	imageSource := createImageSource(t, &types.SystemContext{
		OCICertPath: "fixtures/rejected_certs",
	})
	layer, size, err := imageSource.GetBlob(context.Background(), types.BlobInfo{
		URLs: []string{httpServerAddr},
	})

	require.Error(t, err)
	assert.Nil(t, layer)
	assert.Equal(t, int64(0), size)
}

func remoteLayerContent(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, RemoteLayerContent)
}

func startRemoteLayerServer() (*httptest.Server, error) {
	certBytes, err := ioutil.ReadFile("fixtures/accepted_certs/cert.cert")
	if err != nil {
		return nil, err
	}

	clientCertPool := x509.NewCertPool()
	if !clientCertPool.AppendCertsFromPEM(certBytes) {
		return nil, fmt.Errorf("Could not append certificate")
	}

	cert, err := tls.LoadX509KeyPair("fixtures/accepted_certs/cert.cert", "fixtures/accepted_certs/cert.key")
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		// Reject any TLS certificate that cannot be validated
		ClientAuth: tls.RequireAndVerifyClientCert,
		// Ensure that we only use our "CA" to validate certificates
		ClientCAs:    clientCertPool,
		Certificates: []tls.Certificate{cert},
	}

	httpServer := httptest.NewUnstartedServer(http.HandlerFunc(remoteLayerContent))
	httpServer.TLS = tlsConfig

	httpServer.StartTLS()

	return httpServer, nil
}

func createImageSource(t *testing.T, sys *types.SystemContext) types.ImageSource {
	imageRef, err := NewReference("fixtures/manifest", "")
	require.NoError(t, err)
	imageSource, err := imageRef.NewImageSource(context.Background(), sys)
	require.NoError(t, err)
	return imageSource
}

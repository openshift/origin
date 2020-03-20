package tests

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
)

func TestProxy_Concurrency(t *testing.T) {
	length := 1 << 20
	chunks := 10
	server := httptest.NewServer(newSizedServer(length, chunks))
	defer server.Close()

	stopCh := make(chan struct{})
	defer close(stopCh)

	proxy, cleanup, err := runGRPCProxyServer()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	runAgent(proxy.agent, stopCh)

	// Wait for agent to register on proxy server
	time.Sleep(time.Second)

	// run test client
	tunnel, err := client.CreateGrpcTunnel(proxy.front, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	verify := func() {
		defer wg.Done()

		c := &http.Client{
			Transport: &http.Transport{
				Dial: tunnel.Dial,
			},
		}

		r, err := c.Get(server.URL)
		if err != nil {
			t.Error(err)
		}

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		if len(data) != length*chunks {
			t.Errorf("expect data length %d; got %d", length*chunks, len(data))
		}
	}

	concurrency := 10
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go verify()
	}
	wg.Wait()
}

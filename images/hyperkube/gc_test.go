package gc_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"golang.org/x/net/http2"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

var startGC = make(chan struct{})
var lock = sync.Mutex{}
var counter = 0

type targetHTTPHandler struct {
}

func (d *targetHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lock.Lock()
	counter++
	if counter == 25 {
		startGC <- struct{}{}
	}
	if r.Proto != "HTTP/2.0" {
		w.Write([]byte(fmt.Sprintf("unexpected proto %v", r.Proto)))
		w.WriteHeader(http.StatusInternalServerError)
		lock.Unlock()
		return
	}
	lock.Unlock()
	time.Sleep(60 * time.Second)
	w.Write([]byte("hello from the backend"))
	w.WriteHeader(http.StatusOK)
}

func TestGracefulShutdownForActiveStreams(t *testing.T) {
	// set up the backend server
	backendHandler := &targetHTTPHandler{}
	backendServer := httptest.NewUnstartedServer(backendHandler)
	backendCert, err := tls.X509KeyPair(backendCrt, backendKey)
	if err != nil {
		t.Fatalf("backend: invalid x509/key pair: %v", err)
	}
	backendServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{backendCert},
		NextProtos:   []string{http2.NextProtoTLS},
	}
	backendServer.StartTLS()
	defer backendServer.Close()

	// set up the client
	clientCACertPool := x509.NewCertPool()
	clientCACertPool.AppendCertsFromPEM(backendCrt)
	clientTLSConfig := &tls.Config{
		RootCAs:    clientCACertPool,
		NextProtos: []string{http2.NextProtoTLS},
	}

	client := &http.Client{}
	client.Transport = &http2.Transport{
		TLSClientConfig: clientTLSConfig,
	}

	sendRequest := func(wg *sync.WaitGroup) {
		defer func() {
			wg.Done()
		}()
		// act
		resp, err := client.Get(fmt.Sprintf("https://localhost:%d", backendServer.Listener.Addr().(*net.TCPAddr).Port))
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// validate
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("%v", err)
		}
		t.Logf("body %s", body)

		if resp.StatusCode != 200 {
			t.Errorf("unexpected HTTP staus: %v, expected: 200", resp.StatusCode)
		}
		expectedProto := "HTTP/2.0"
		if resp.Proto != expectedProto {
			t.Errorf("unexpected response proto: %v, expected: %v", resp.Proto, expectedProto)
		}
	}

	go func() {
		<-startGC
		time.Sleep(5 * time.Second) // gives the last connection a chance to go into the sleep state

		// Requires https://github.com/golang/go/issues/39776 to work properly
		backendServer.Config.Shutdown(context.Background())
	}()

	wg := sync.WaitGroup{}
	wg.Add(25)
	for i := 0; i < 25; i++ {
		go sendRequest(&wg)
	}

	wg.Wait()
}

// valid for localhost
var backendCrt = []byte(`-----BEGIN CERTIFICATE-----
MIIC5TCCAc2gAwIBAgIJAOS8kx4rqQxcMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNV
BAMMCWxvY2FsaG9zdDAeFw0yMDA2MTUxMTE2MDFaFw0yMDA3MTUxMTE2MDFaMBQx
EjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAPL7bdiu1h8BadQPn5tgN4cBbMLmP8jpNduoC7KExbtKz7mdbCi7t5/vRgEq
tEgJqcsBbCCZzAYQHExkRqchaiVOQf1JvYywHSJ0IQ9IpIB4WwZiRitKsBoUwufn
ekpvHNOUwKbjQWdxCz26sCSgDsLNK2COmJwFoTFUQWuC0X1SYsT5KqnJwTMP19Xq
GFI0sWsZoQxe7QhJBYu8ierA+OkS0yZiBvFX8Cb1ChjUA3D1Bred2eNSZSafij2z
ZsvpAQea7lUmRVAJe/+HHGgptXiHR+voWh5LnI+SGTfRIjgXSogc6rSlkDxBL3qs
BSOKoiF8sy9WkowY8FKGGQkMmJcCAwEAAaM6MDgwFAYDVR0RBA0wC4IJbG9jYWxo
b3N0MAsGA1UdDwQEAwIHgDATBgNVHSUEDDAKBggrBgEFBQcDATANBgkqhkiG9w0B
AQsFAAOCAQEAvqdSHV2OAY36Xwe+5egq2oH98zfxTyp9hgsIO/8VJf/ukw+sSKFY
ZEl3ABzjHk9BDyLLoj6DjvjHva6Ghk/ruYg9Q312+dkn/RRCuKx2cOUSq+SFZxra
Lv4BMO8miiPeVmvP1klhqZZMCV7qpC/MdVVn3SgGYB9ymhGQa0iE0scUk1+zDNIg
p7iHbi227WZ/pROEFt8sSf1MltaQ/0QI9G2yCxDgjPSNte8vCqVDbXZkXBE5i6qF
TbvIk4/K13UC3YAgfhedNzf5Smbe6moK008BCp7itKL6IDb20gI9htBsgzolT8O+
G5lxVI5gSU/VdcGjW0EyWcKEct4LZMUTCg==
-----END CERTIFICATE-----
`)

var backendKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDy+23YrtYfAWnU
D5+bYDeHAWzC5j/I6TXbqAuyhMW7Ss+5nWwou7ef70YBKrRICanLAWwgmcwGEBxM
ZEanIWolTkH9Sb2MsB0idCEPSKSAeFsGYkYrSrAaFMLn53pKbxzTlMCm40FncQs9
urAkoA7CzStgjpicBaExVEFrgtF9UmLE+SqpycEzD9fV6hhSNLFrGaEMXu0ISQWL
vInqwPjpEtMmYgbxV/Am9QoY1ANw9Qa3ndnjUmUmn4o9s2bL6QEHmu5VJkVQCXv/
hxxoKbV4h0fr6FoeS5yPkhk30SI4F0qIHOq0pZA8QS96rAUjiqIhfLMvVpKMGPBS
hhkJDJiXAgMBAAECggEAIreyFkfE6GE3Ucl5sKWqyWt2stJbQsWvoFb+dN9rsTsb
OxY3IgrQTdXOVtRXNgPLcuodHPtcn3El2fRp8+9eTz5DR4GFx9hSEV4uaxSiDIkl
2F+qTv049EELKD92xbPiloimjjHiYnlQdd161YDZGxRdoko9m+1h/r5fKpFihVk8
5H6RaGb2hga6iuIvAoZ0sGPOIchSOOXC0Dpz7AimSW8JnE2aWNlRu/jiBQ9RxhAr
WP5Ey2FpNqgQfD22pbx3Ql7ULdFV2GP4owo3eWDbvHtIq+Q9WibE7zfWTtBuTKYu
oeo2e8mkKR83KmtqWzLRGEgxDvzwT/fk8ldoiSZewQKBgQD8kdEYrqyJA6kGCrQY
YjX8BXu+c4fkm3yxwGLJiA7RExckQ5smxy1Fzl4I4PApxWStOWxm5Uh2s0xSbDW3
TnRyuzVq5XehQiB5vFzPgU3ywKLy4hXrKxSotH/k6yHQMF4QJZaPpkxPQNIbdN03
6yntrdNB4sUxpYbrtAeSYaqziQKBgQD2SEbbOOO6Zl96KAiQ0D3v7vP86H4gjyLV
w1VDiyRCPimHbT3kCNKQZMQdKPssvf3ie6JBMNQc0K3lkzU2qvihI6jOb6QK6QIF
5eqySPDV7ZysU4CaFHSLXg6pyJ5XB+3Y8mmxnEm6EmpeOuI+4MCZ5zcFR1+kIRHU
ORzLGERDHwKBgQDKHXJjuxyNBKXVFOmr/aPPyx+Md+2OjrMJl7g2KDAbNZi2R3e4
X3mmPA/aMQ9fjfwT9zj9WoxTmQYBi2CtERZ03cVQhtLl9AIDCS6IS6RyF6AOl8gM
ikwc+VzDdzp23M3ZRAspZ133qhq5KBsDbaf+8LR3LB67rQe8RTQt+wRcaQKBgQDE
BS7wWU1YFRc1IRwANt61U5k62MlanNJ7FWeNxPdtChD/y0EReLwvVSSKmQ2hxO6I
DyNLg9Ovw6BFM2+NPXN6vekjtdP5IxALJb4xfMDDZMXomuWmvVUtgAVnuVfdqV/z
5q2dQemkgffLXE6rATQKyu8N8or7FZ8dLP/v3jamvQKBgER5Rr91lvS2HFXow8Rg
tAmpti96MRH0SK2gdAoT7Xr9hsqqC8dAtgAdF+jzeecQk5IaNlGS0SRQpCdMIvE1
Qy8OUgg/TEsBhDpg4FbFXwqlOE1PVsV+HNw578YHkOkamSH1rxTW9EJI8h+aiwqr
Yw8ovJTCPLC33LushxKas9hY
-----END PRIVATE KEY-----
`)

package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func main() {
	server := "https://api.ci-ln-grrpvyk-76ef8.aws-2.ci.openshift.org:6443"
	token := "eyJhbGciOiJSUzI1NiIsImtpZCI6IlBLazg4RXFXMjFZbUVEaDFnTzNMUFFvWmdEWm81QkFSbUQ4dVAyMG9nYnMifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImRlZmF1bHQtdG9rZW4tdDlsNjciLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZGVmYXVsdCIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjBkMGM4NjRmLTExYjktNDI3NC1iODA4LWJmNmY2ZjcxNTIzYyIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmRlZmF1bHQifQ.BgJLBfQIyCYwlku8a3AB8th_PEwz1oDGm186KS6DlVmT3A3hfOlXnEYl8S72Pud5h5bREOc5aqs573X2aPoNenOlvuad-neFqhXGy_r9do3mtj3RxY0f6R85OB093rl0Zkqe7gqCrSwRa1D6uJkKFhAzSO5D-RyKVzdZj3BZ79KsyA8sFhajSD6audY4DTlong0TUqBkhteflN46JVv8pZkWQj4FPDgygI9V6tF_6eRqrAoSwSBjhdzi7DME_RJbaxEW88_clVrOzcbBRUQA13CZdQRHkNH8HDhgHwzkISti5sjNrsAO3vg5AaHnmF2U-Gm8LjnjcSqno0Y67Pl2PA"

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	req, err := http.NewRequest("GET", server+"/api/v1/nodes", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	start := time.Now()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	end := time.Now()

	fmt.Printf("read %v bytes in %v\n", len(content), end.Sub(start))
}

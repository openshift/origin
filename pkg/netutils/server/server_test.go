package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/openshift/openshift-sdn/pkg/netutils"
)

func delIP(t *testing.T, delip string) error {
	url := fmt.Sprintf("http://127.0.0.1:9080/netutils/ip/%s", delip)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatalf("Error in forming request to IPAM server: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error in connecting to IPAM server: %v", err)
	}
	if res.StatusCode > 400 {
		return fmt.Errorf("Bad response from server: %d", res.StatusCode)
	}
	return err
}

func getIP(t *testing.T) string {
	res, err := http.Get("http://127.0.0.1:9080/netutils/ip")
	if err != nil {
		t.Fatalf("Error in connecting to IPAM server: %v", err)
	}
	ip, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("Error in obtaining IP address through server: %v", err)
	}
	res.Body.Close()
	return string(ip)
}

func TestIPServe(t *testing.T) {
	inuse := make([]string, 0)
	ipam, err := netutils.NewIPAllocator("10.20.30.40/24", inuse)
	if err != nil {
		t.Fatalf("Error while initializing IPAM: %v", err)
	}
	go ListenAndServeNetutilServer(ipam, net.ParseIP("127.0.0.1"), 9080, nil)

	// get, get, delete, get
	ip := getIP(t)
	if ip != "10.20.30.1/24" {
		t.Fatalf("Wrong IP. Expected 10.20.30.1/24, got %s", ip)
	}
	ip = getIP(t)
	if ip != "10.20.30.2/24" {
		t.Fatalf("Wrong IP. Expected 10.20.30.2/24, got %s", ip)
	}
	err = delIP(t, ip)
	if err != nil {
		t.Fatalf("Error while deleting IP address %s: %v", ip, err)
	}
	// get it again
	ip = getIP(t)
	if ip != "10.20.30.2/24" {
		t.Fatalf("Wrong IP. Expected 10.20.30.2/24, got %s", ip)
	}
	// delete the wrong one and fail if there is no error
	err = delIP(t, "10.10.10.10/23")
	if err == nil {
		t.Fatalf("Error while deleting IP address %s: %v", ip, err)
	}
}

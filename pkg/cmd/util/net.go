package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/golang/glog"
)

// TryListen tries to open a connection on the given port and returns true if it succeeded.
func TryListen(hostPort string) (bool, error) {
	l, err := net.Listen("tcp", hostPort)
	if err != nil {
		glog.V(5).Infof("Failure while checking listen on %s: %v", err)
		return false, err
	}
	defer l.Close()
	return true, nil
}

// WaitForSuccessfulDial attempts to connect to the given address, closing and returning nil on the first successful connection.
func WaitForSuccessfulDial(https bool, network, address string, timeout, interval time.Duration, retries int) error {
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i <= retries; i++ {
		dialer := net.Dialer{Timeout: timeout}
		if https {
			conn, err = tls.DialWithDialer(&dialer, network, address, &tls.Config{InsecureSkipVerify: true})
		} else {
			conn, err = dialer.Dial(network, address)
		}
		if err != nil {
			glog.V(5).Infof("Got error %#v, trying again: %#v\n", err, address)
			time.Sleep(interval)
			continue
		}
		conn.Close()
		return nil
	}
	return err
}

// TransportFor returns an http.Transport for the given ca and client cert (which may be empty strings)
func TransportFor(ca string, certFile string, keyFile string) (http.RoundTripper, error) {
	if len(ca) == 0 && len(certFile) == 0 && len(keyFile) == 0 {
		return http.DefaultTransport, nil
	}

	if (len(certFile) == 0) != (len(keyFile) == 0) {
		return nil, errors.New("certFile and keyFile must be specified together")
	}

	// Copy default transport
	transport := *http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{}

	if len(ca) != 0 {
		roots, err := CertPoolFromFile(ca)
		if err != nil {
			return nil, fmt.Errorf("error loading cert pool from ca file %s: %v", ca, err)
		}
		transport.TLSClientConfig.RootCAs = roots
	}

	if len(certFile) != 0 {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading x509 keypair from cert file %s and key file %s: %v", certFile, keyFile, err)
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	return &transport, nil
}

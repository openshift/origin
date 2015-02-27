package util

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/golang/glog"
)

// WaitForDial attempts to connect to the given address, closing and returning nil on the first successful connection.
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
		glog.V(4).Infof("Got success: %#v\n", address)
		return nil
	}
	glog.V(4).Infof("Got error, failing: %#v\n", address)
	return err
}

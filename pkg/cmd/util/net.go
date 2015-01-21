package util

import (
	"net"
	"time"

	"github.com/golang/glog"
)

// WaitForDial attempts to connect to the given address, closing and returning nil on the first successful connection.
func WaitForSuccessfulDial(network, address string, timeout, interval time.Duration, retries int) error {
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i <= retries; i++ {
		conn, err = net.DialTimeout(network, address, timeout)
		if err != nil {
			glog.V(4).Infof("Got error %#v, trying again: %#v\n", err, address)
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

package node

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/util/ovs/ovsclient"
)

const (
	ovsDialTimeout         = 5 * time.Second
	ovsHealthcheckInterval = 30 * time.Second
	ovsRecoveryTimeout     = 10 * time.Second
	ovsDialDefaultNetwork  = "unix"
	ovsDialDefaultAddress  = "/var/run/openvswitch/db.sock"
)

// waitForOVS polls until the OVS server responds to a connection and an 'echo'
// command.
func waitForOVS(network, addr string) error {
	return utilwait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		c, err := ovsclient.DialTimeout(network, addr, ovsDialTimeout)
		if err != nil {
			glog.V(2).Infof("waiting for OVS to start: %v", err)
			return false, nil
		}
		defer c.Close()
		if err := c.Ping(); err != nil {
			glog.V(2).Infof("waiting for OVS to start, ping failed: %v", err)
			return false, nil
		}
		return true, nil
	})
}

// runOVSHealthCheck runs two background loops - one that waits for disconnection
// from the OVS server and then checks healthFn, and one that periodically checks
// healthFn. If healthFn returns false in either of these two cases while the OVS
// server is responsive the node process will terminate.
func runOVSHealthCheck(network, addr string, healthFn func() error) {
	// this loop holds an open socket connection to OVS until it times out, then
	// checks for health
	go utilwait.Until(func() {
		c, err := ovsclient.DialTimeout(network, addr, ovsDialTimeout)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("SDN healthcheck unable to connect to OVS server: %v", err))
			return
		}
		defer c.Close()

		err = c.WaitForDisconnect()
		utilruntime.HandleError(fmt.Errorf("SDN healthcheck disconnected from OVS server: %v", err))

		err = utilwait.PollImmediate(100*time.Millisecond, ovsRecoveryTimeout, func() (bool, error) {
			c, err := ovsclient.DialTimeout(network, addr, ovsDialTimeout)
			if err != nil {
				glog.V(2).Infof("SDN healthcheck unable to reconnect to OVS server: %v", err)
				return false, nil
			}
			defer c.Close()
			if err := c.Ping(); err != nil {
				glog.V(2).Infof("SDN healthcheck unable to ping OVS server: %v", err)
				return false, nil
			}
			if err := healthFn(); err != nil {
				return false, fmt.Errorf("OVS health check failed: %v", err)
			}
			return true, nil
		})
		if err != nil {
			// If OVS restarts and our health check fails, we exit
			// TODO: make openshift-sdn able to reconcile without a restart
			glog.Fatalf("SDN healthcheck detected unhealthy OVS server, restarting: %v", err)
		}
	}, ovsDialTimeout, utilwait.NeverStop)

	// this loop periodically verifies we can still connect to the OVS server and
	// is an upper bound on the time we wait before detecting a failed OVS configuartion
	go utilwait.Until(func() {
		c, err := ovsclient.DialTimeout(network, addr, ovsDialTimeout)
		if err != nil {
			glog.V(2).Infof("SDN healthcheck unable to reconnect to OVS server: %v", err)
			return
		}
		defer c.Close()
		if err := c.Ping(); err != nil {
			glog.V(2).Infof("SDN healthcheck unable to ping OVS server: %v", err)
			return
		}
		if err := healthFn(); err != nil {
			glog.Fatalf("SDN healthcheck detected unhealthy OVS server, restarting: %v", err)
		}
		glog.V(4).Infof("SDN healthcheck succeeded")
	}, ovsHealthcheckInterval, utilwait.NeverStop)
}

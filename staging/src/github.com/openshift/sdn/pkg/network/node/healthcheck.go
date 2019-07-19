package node

import (
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	"github.com/openshift/sdn/pkg/network/node/ovs/ovsclient"
)

const (
	ovsDialTimeout         = 5 * time.Second
	ovsHealthcheckInterval = 30 * time.Second
	ovsRecoveryTimeout     = 10 * time.Second
	ovsDialDefaultNetwork  = "unix"
	ovsDialDefaultAddress  = "/var/run/openvswitch/db.sock"
)

// dialAndPing connects to OVS once and pings the server. It returns
// the dial error (if any) or the ping error (if any), or neither.
func dialAndPing(network, addr string) (error, error) {
	c, err := ovsclient.DialTimeout(network, addr, ovsDialTimeout)
	if err != nil {
		return err, nil
	}
	defer c.Close()
	if err := c.Ping(); err != nil {
		return nil, err
	}
	return nil, nil
}

// waitForOVS polls until the OVS server responds to a connection and an 'echo'
// command.
func waitForOVS(network, addr string) error {
	return utilwait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		dialErr, pingErr := dialAndPing(network, addr)
		if dialErr != nil {
			klog.V(2).Infof("waiting for OVS to start: %v", dialErr)
			return false, nil
		} else if pingErr != nil {
			klog.V(2).Infof("waiting for OVS to start, ping failed: %v", pingErr)
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

		_ = c.WaitForDisconnect()

		// Poll OVS in a tight loop waiting for reconnect
		err = utilwait.PollImmediate(100*time.Millisecond, ovsRecoveryTimeout, func() (bool, error) {
			if dialErr, pingErr := dialAndPing(network, addr); dialErr != nil || pingErr != nil {
				return false, nil
			}
			if err := healthFn(); err != nil {
				return false, fmt.Errorf("OVS reinitialization required: %v", err)
			}
			return true, nil
		})
		if err != nil {
			// If OVS restarts and our health check fails, we exit
			// TODO: make openshift-sdn able to reconcile without a restart
			klog.Fatalf("SDN healthcheck detected OVS server change, restarting: %v", err)
		}
		klog.V(2).Infof("SDN healthcheck reconnected to OVS server")
	}, ovsDialTimeout, utilwait.NeverStop)

	// this loop periodically verifies we can still connect to the OVS server and
	// is an upper bound on the time we wait before detecting a failed OVS configuartion
	go utilwait.Until(func() {
		dialErr, pingErr := dialAndPing(network, addr)
		if dialErr != nil {
			klog.V(2).Infof("SDN healthcheck unable to reconnect to OVS server: %v", dialErr)
			return
		} else if pingErr != nil {
			klog.V(2).Infof("SDN healthcheck unable to ping OVS server: %v", pingErr)
			return
		}
		if err := healthFn(); err != nil {
			klog.Fatalf("SDN healthcheck detected unhealthy OVS server, restarting: %v", err)
		}
		klog.V(4).Infof("SDN healthcheck succeeded")
	}, ovsHealthcheckInterval, utilwait.NeverStop)
}

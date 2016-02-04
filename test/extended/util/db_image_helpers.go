package util

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"
)

// Database interface allows testing database images.
type Database interface {
	// PodName returns the name of the Pod this helper is bound to.
	PodName() string

	// IsReady indicates whether the underlying Pod is ready for queries.
	IsReady(oc *CLI) (bool, error)

	// Query queries the database as a regular user.
	Query(oc *CLI, query string) (string, error)

	// QueryPrivileged queries the database as a privileged user.
	QueryPrivileged(oc *CLI, query string) (string, error)

	// TestRemoteLogin tests wheather it is possible to remote login to hostAddress.
	TestRemoteLogin(oc *CLI, hostAddress string) error
}

// WaitForQueryOutput will execute the query multiple times, until the
// specified substring is found in the results. This function should be used for
// testing replication, since it might take some time untill the data is propagated
// to slaves.
func WaitForQueryOutput(oc *CLI, d Database, timeout time.Duration, admin bool, query, resultSubstr string) error {
	err := wait.Poll(5*time.Second, timeout, func() (bool, error) {
		var (
			out string
			err error
		)

		if admin {
			out, err = d.QueryPrivileged(oc, query)
		} else {
			out, err = d.Query(oc, query)
		}
		if _, ok := err.(*exec.ExitError); ok {
			// Ignore exit errors
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if strings.Contains(out, resultSubstr) {
			return true, nil
		}
		return false, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("timed out waiting for query: %q", query)
	}
	return err
}

// WaitUntilUp continuously waits for the server to become ready, up until timeout.
func WaitUntilUp(oc *CLI, d Database, timeout time.Duration) error {
	err := wait.Poll(2*time.Second, timeout, func() (bool, error) {
		return d.IsReady(oc)
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("timed out waiting for pod %s get up", d.PodName())
	}
	return err
}

// WaitUntilAllHelpersAreUp waits until all helpers are ready to serve requests.
func WaitUntilAllHelpersAreUp(oc *CLI, helpers []Database) error {
	for _, m := range helpers {
		if err := WaitUntilUp(oc, m, 3*time.Minute); err != nil {
			return err
		}
	}
	return nil
}

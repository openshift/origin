package util

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
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

	// TestRemoteLogin tests whether it is possible to remote login to hostAddress.
	TestRemoteLogin(oc *CLI, hostAddress string) error
}

// ReplicaSet interface allows to interact with database on multiple nodes.
type ReplicaSet interface {
	// QueryPrimary queries the database on primary node as a regular user.
	QueryPrimary(oc *CLI, query string) (string, error)
}

// WaitForQueryOutputSatisfies will execute the query multiple times, until the
// specified predicate function is return true.
func WaitForQueryOutputSatisfies(oc *CLI, d Database, timeout time.Duration, admin bool, query string, predicate func(string) bool) error {
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
		if _, ok := err.(*ExitError); ok {
			// Ignore exit errors
			return false, nil
		}
		if _, ok := err.(*exec.ExitError); ok {
			// Ignore exit errors
			return false, nil
		}
		if err != nil {
			e2e.Logf("failing immediately with error: %v, type=%v", err, reflect.TypeOf(err))
			return false, err
		}
		if predicate(out) {
			return true, nil
		}
		return false, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("timed out waiting for query: %q", query)
	}
	return err
}

// WaitForQueryOutputContains will execute the query multiple times, until the
// specified substring is found in the results. This function should be used for
// testing replication, since it might take some time until the data is propagated
// to slaves.
func WaitForQueryOutputContains(oc *CLI, d Database, timeout time.Duration, admin bool, query, resultSubstr string) error {
	return WaitForQueryOutputSatisfies(oc, d, timeout, admin, query, func(resultOutput string) bool {
		return strings.Contains(resultOutput, resultSubstr)
	})
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

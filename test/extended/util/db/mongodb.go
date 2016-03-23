package db

import (
	"errors"
	"fmt"

	"github.com/openshift/origin/test/extended/util"
)

// MongoDB is a MongoDB helper for executing commands.
type MongoDB struct {
	podName string
}

// NewMongoDB creates a new util.Database instance.
func NewMongoDB(podName string) util.Database {
	return &MongoDB{
		podName: podName,
	}
}

// PodName implements Database.
func (m MongoDB) PodName() string {
	return m.podName
}

// IsReady pings the MongoDB server.
func (m MongoDB) IsReady(oc *util.CLI) (bool, error) {
	return isReady(
		oc,
		m.podName,
		`mongo --quiet --eval '{"ping", 1}'`,
		"1",
	)
}

// Query executes a query as an ordinary user and returns the result.
func (m MongoDB) Query(oc *util.CLI, query string) (string, error) {
	return executeShellCommand(
		oc,
		m.podName,
		fmt.Sprintf(`mongo --quiet "$MONGODB_DATABASE" --username "$MONGODB_USER" --password "$MONGODB_PASSWORD" --eval '%s'`, query),
	)
}

// QueryPrivileged queries the database as a privileged user.
func (m MongoDB) QueryPrivileged(oc *util.CLI, query string) (string, error) {
	return "", errors.New("not implemented")
}

// TestRemoteLogin tests whether it is possible to remote login to hostAddress.
func (m MongoDB) TestRemoteLogin(oc *util.CLI, hostAddress string) error {
	return errors.New("not implemented")
}

// // QueryPrimary queries the database on primary node as a regular user.
func (m MongoDB) QueryPrimary(oc *util.CLI, query string) (string, error) {
	return executeShellCommand(
		oc,
		m.podName,
		fmt.Sprintf(
			`mongo --quiet "$MONGODB_DATABASE" --username "$MONGODB_USER" --password "$MONGODB_PASSWORD" --host "$MONGODB_REPLICA_NAME/localhost" --eval '%s'`,
			query,
		),
	)
}

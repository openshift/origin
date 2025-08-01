package db

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

// PostgreSQL is a PostgreSQL helper for executing commands.
type PostgreSQL struct {
	podName       string
	masterPodName string
}

// NewPostgreSQL creates a new util.Database instance.
func NewPostgreSQL(podName, masterPodName string) util.Database {
	if masterPodName == "" {
		masterPodName = podName
	}
	return &PostgreSQL{
		podName:       podName,
		masterPodName: masterPodName,
	}
}

// PodName implements Database.
func (m PostgreSQL) PodName() string {
	return m.podName
}

// IsReady pings the PostgreSQL server.
func (m PostgreSQL) IsReady(oc *exutil.CLI) (bool, error) {
	conf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return false, err
	}
	out, err := oc.Run("exec").Args(m.podName, "-c", conf.Container, "--", "bash", "-c",
		"psql postgresql://postgres@127.0.0.1 -x -c \"SELECT 1;\"").Output()
	if err != nil {
		switch err.(type) {
		case *util.ExitError, *exec.ExitError:
			return false, nil
		default:
			return false, err
		}
	}
	return strings.Contains(out, "-[ RECORD 1 ]\n?column? | 1"), nil
}

// Query executes an SQL query as an ordinary user and returns the result.
func (m PostgreSQL) Query(oc *exutil.CLI, query string) (string, error) {
	container, err := firstContainerName(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return "", err
	}
	masterConf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.masterPodName)
	if err != nil {
		return "", err
	}
	return oc.Run("exec").Args(m.podName, "-c", container, "--", "bash", "-c",
		fmt.Sprintf("psql postgres://%s:%s@127.0.0.1/%s -x -c \"%s\"",
			masterConf.Env["POSTGRESQL_USER"], masterConf.Env["POSTGRESQL_PASSWORD"],
			masterConf.Env["POSTGRESQL_DATABASE"], query)).Output()
}

// QueryPrivileged executes an SQL query as a root user and returns the result.
func (m PostgreSQL) QueryPrivileged(oc *exutil.CLI, query string) (string, error) {
	container, err := firstContainerName(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return "", err
	}
	masterConf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.masterPodName)
	if err != nil {
		return "", err
	}
	return oc.Run("exec").Args(m.podName, "-c", container, "--", "bash", "-c",
		fmt.Sprintf("psql postgres://postgres:%s@127.0.0.1/%s -x -c \"%s\"",
			masterConf.Env["POSTGRESQL_ADMIN_PASSWORD"],
			masterConf.Env["POSTGRESQL_DATABASE"], query)).Output()
}

// TestRemoteLogin will test whether we can login through to a remote database.
func (m PostgreSQL) TestRemoteLogin(oc *exutil.CLI, hostAddress string) error {
	container, err := firstContainerName(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return err
	}
	masterConf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.masterPodName)
	if err != nil {
		return err
	}
	err = oc.Run("exec").Args(m.podName, "-c", container, "--", "bash", "-c",
		fmt.Sprintf("psql postgres://%s:%s@%s/%s -x -c \"SELECT 1;\"",
			masterConf.Env["POSTGRESQL_USER"], masterConf.Env["POSTGRESQL_PASSWORD"],
			hostAddress, masterConf.Env["POSTGRESQL_DATABASE"])).Execute()
	return err
}

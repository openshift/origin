package db

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/openshift/origin/test/extended/util"
)

// MySQL is a MySQL helper for executing commands.
type MySQL struct {
	podName       string
	masterPodName string
}

// NewMysql creates a new util.Database instance.
func NewMysql(podName, masterPodName string) util.Database {
	if masterPodName == "" {
		masterPodName = podName
	}
	return &MySQL{
		podName:       podName,
		masterPodName: masterPodName,
	}
}

// PodName implements Database.
func (m MySQL) PodName() string {
	return m.podName
}

// IsReady pings the MySQL server.
func (m MySQL) IsReady(oc *util.CLI) (bool, error) {
	conf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return false, err
	}
	out, err := oc.Run("exec").Args(m.podName, "-c", conf.Container, "--", "bash", "-c",
		"mysqladmin -h localhost -uroot ping").Output()
	if err != nil {
		switch err.(type) {
		case *util.ExitError, *exec.ExitError:
			return false, nil
		default:
			return false, err
		}
	}
	return strings.Contains(out, "mysqld is alive"), nil
}

// Query executes an SQL query as an ordinary user and returns the result.
func (m MySQL) Query(oc *util.CLI, query string) (string, error) {
	container, err := firstContainerName(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return "", err
	}
	masterConf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.masterPodName)
	if err != nil {
		return "", err
	}
	return oc.Run("exec").Args(m.podName, "-c", container, "--", "bash", "-c",
		fmt.Sprintf("mysql -h 127.0.0.1 -u%s -p%s -e \"%s\" %s",
			masterConf.Env["MYSQL_USER"], masterConf.Env["MYSQL_PASSWORD"], query,
			masterConf.Env["MYSQL_DATABASE"])).Output()
}

// QueryPrivileged executes an SQL query as a root user and returns the result.
func (m MySQL) QueryPrivileged(oc *util.CLI, query string) (string, error) {
	container, err := firstContainerName(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return "", err
	}
	masterConf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.masterPodName)
	if err != nil {
		return "", err
	}
	return oc.Run("exec").Args(m.podName, "-c", container, "--", "bash", "-c",
		fmt.Sprintf("mysql -h 127.0.0.1 -uroot -e \"%s\" %s",
			query, masterConf.Env["MYSQL_DATABASE"])).Output()
}

// TestRemoteLogin will test whether we can login through to a remote database.
func (m MySQL) TestRemoteLogin(oc *util.CLI, hostAddress string) error {
	container, err := firstContainerName(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.podName)
	if err != nil {
		return err
	}
	masterConf, err := getPodConfig(oc.KubeClient().CoreV1().Pods(oc.Namespace()), m.masterPodName)
	if err != nil {
		return err
	}
	err = oc.Run("exec").Args(m.podName, "-c", container, "--", "bash", "-c",
		fmt.Sprintf("mysql -h %s -u%s -p%s -e \"SELECT 1;\" %s",
			hostAddress, masterConf.Env["MYSQL_USER"], masterConf.Env["MYSQL_PASSWORD"],
			masterConf.Env["MYSQL_DATABASE"])).Execute()
	return err
}

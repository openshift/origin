package util

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

// Database interface allows testing database images
type Database interface {
	// GetPodName returns the name of the Pod this helper is bound to
	GetPodName() string

	// IsReady indicates whether the underlying Pod is ready for queries
	IsReady(oc *CLI) (bool, error)

	// Query queries the database as a regular user
	Query(oc *CLI, query string) (string, error)

	// Query queries the database as a privileged user
	QueryPrivileged(oc *CLI, query string) (string, error)

	// TestRemoteLogin tests weather it is possible to remote login to hostAddress
	TestRemoteLogin(oc *CLI, hostAddress string) error
}

// MySQL is a MySQL helper for executing commands
type MySQL struct {
	PodName       string
	MasterPodName string
}

// PodConfig holds configuration for a pod
type PodConfig struct {
	Container string
	Env       map[string]string
}

// NewMysql queries OpenShift for a pod with given name, saving environment
// variables like username and password for easier use.
func NewMysql(c kclient.PodInterface, podName, masterPodName string) Database {
	if masterPodName == "" {
		masterPodName = podName
	}
	return &MySQL{
		PodName:       podName,
		MasterPodName: masterPodName,
	}
}

func (m MySQL) GetPodName() string {
	return m.PodName
}

func GetPodConfig(c kclient.PodInterface, podName string) (conf *PodConfig, err error) {
	pod, err := c.Get(podName)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string)
	for _, container := range pod.Spec.Containers {
		for _, e := range container.Env {
			env[e.Name] = e.Value
		}
	}
	return &PodConfig{pod.Spec.Containers[0].Name, env}, nil
}

// IsReady pings the MySQL server
func (m MySQL) IsReady(oc *CLI) (bool, error) {
	conf, err := GetPodConfig(oc.KubeREST().Pods(oc.Namespace()), m.PodName)
	if err != nil {
		return false, err
	}
	out, err := oc.Run("exec").Args(m.PodName, "-c", conf.Container, "--", "bash", "-c",
		"mysqladmin -h 127.0.0.1 -uroot ping").Output()
	if err != nil {
		switch err.(type) {
		case *exec.ExitError:
			return false, nil
		default:
			return false, err
		}
	}
	return out == "mysqld is alive", nil
}

// Query executes an SQL query as an ordinary user and returns the result.
func (m MySQL) Query(oc *CLI, query string) (string, error) {
	conf, err := GetPodConfig(oc.KubeREST().Pods(oc.Namespace()), m.MasterPodName)
	if err != nil {
		return "", err
	}
	return oc.Run("exec").Args(m.PodName, "-c", conf.Container, "--", "bash", "-c",
		fmt.Sprintf("mysql -h 127.0.0.1 -u%s -p%s -e \"%s\" %s",
			conf.Env["MYSQL_USER"], conf.Env["MYSQL_PASSWORD"], query,
			conf.Env["MYSQL_DATABASE"])).Output()
}

// QueryPrivileged executes an SQL query as a root user and returns the result.
func (m MySQL) QueryPrivileged(oc *CLI, query string) (string, error) {
	conf, err := GetPodConfig(oc.KubeREST().Pods(oc.Namespace()), m.MasterPodName)
	if err != nil {
		return "", err
	}
	return oc.Run("exec").Args(m.PodName, "-c", conf.Container, "--", "bash", "-c",
		fmt.Sprintf("mysql -h 127.0.0.1 -uroot -e \"%s\" %s",
			query, conf.Env["MYSQL_DATABASE"])).Output()
}

// TestRemoteLogin will test whether we can login through to a remote database.
func (m MySQL) TestRemoteLogin(oc *CLI, hostAddress string) error {
	conf, err := GetPodConfig(oc.KubeREST().Pods(oc.Namespace()), m.PodName)
	if err != nil {
		return err
	}
	confi, err := GetPodConfig(oc.KubeREST().Pods(oc.Namespace()), m.MasterPodName)
	if err != nil {
		return err
	}
	_, err = oc.Run("exec").Args(m.PodName, "-c", conf.Container, "--", "bash", "-c",
		fmt.Sprintf("mysql -h %s -u%s -p%s -e \"SELECT 1;\" %s",
			hostAddress, confi.Env["MYSQL_USER"], confi.Env["MYSQL_PASSWORD"],
			confi.Env["MYSQL_DATABASE"])).Output()
	return err
}

// WaitForQueryOutput will execute the query multiple times, until the
// specified substring is found in the results. This function should be used for
// testing replication, since it might take some time untill the data is propagated
// to slaves.
func WaitForQueryOutput(oc *CLI, d Database, timeout time.Duration, admin bool, query, resultSubstr string) error {
	return wait.Poll(5*time.Second, timeout, func() (bool, error) {
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
}

// WaitUntilUp continuously waits for the server to become ready, up until timeout.
func WaitUntilUp(oc *CLI, d Database, timeout time.Duration) error {
	return wait.Poll(2*time.Second, timeout, func() (bool, error) {
		return d.IsReady(oc)
	})
}

// WaitUntilAllHelpersAreUp waits until all helpers are ready to serve requests
func WaitUntilAllHelpersAreUp(oc *CLI, helpers []Database) error {
	for _, m := range helpers {
		if err := WaitUntilUp(oc, m, 180*time.Second); err != nil {
			return err
		}
	}
	return nil
}

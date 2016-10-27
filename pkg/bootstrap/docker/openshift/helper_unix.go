// +build !windows

package openshift

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

func CheckSocat() error {
	_, err := exec.LookPath("socat")
	if err != nil {
		return err
	}
	return nil
}

func KillExistingSocat() error {
	_, err := os.Stat(SocatPidFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pidStr, err := ioutil.ReadFile(SocatPidFile)
	if err != nil {
		return err
	}
	defer os.Remove(SocatPidFile)
	pid, err := strconv.Atoi(string(pidStr))
	if err != nil {
		return err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func SaveSocatPid(pid int) error {
	parentDir := filepath.Dir(SocatPidFile)
	err := os.MkdirAll(parentDir, 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(SocatPidFile, []byte(strconv.Itoa(pid)), 0644)
}

func (h *Helper) startSocatTunnel() error {
	// Previous process should have been killed with
	// 'oc cluster down', call again here in case it wasn't
	err := KillExistingSocat()
	if err != nil {
		glog.V(1).Infof("error: cannot kill socat: %v", err)
	}
	cmd := exec.Command("socat", "TCP-L:8443,reuseaddr,fork,backlog=20,bind=127.0.0.1", "SYSTEM:\"docker exec -i origin socat - TCP\\:localhost\\:8443,nodelay\"")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	if err != nil {
		return errors.NewError("cannot start socat tunnel").WithCause(err)
	}
	glog.V(1).Infof("Started socat with pid: %d", cmd.Process.Pid)
	return SaveSocatPid(cmd.Process.Pid)
}

package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
)

var (
	reLeaseID *regexp.Regexp
)

type Controllers struct {
	cmd         *exec.Cmd
	listenPort  int
	lease       string
	leaseErr    error
	leaseLock   sync.Mutex
	leaseNotify *sync.Cond
	configPath  string
	outputDir   string
}

func init() {
	reLeaseID = regexp.MustCompile(`(?i)acquire controller lease as ([[:alnum:]-_]+),`)
}

type NullWriter int

func (NullWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func NewControllers(listenPort int, configPath string, outputDir string) *Controllers {
	cs := &Controllers{listenPort: listenPort, configPath: configPath, outputDir: outputDir}
	cs.cmd = exec.Command("openshift", "--loglevel=5", "start", "master", "controllers",
		"--config="+configPath, "--listen="+cs.ListenURL())
	cs.cmd.Env = append(cs.cmd.Env, "OPENSHIFT_ON_PANIC=crash")
	cs.cmd.Dir = outputDir
	for _, v := range []string{"PATH", "USER", "HOME", "LOGNAME"} {
		cs.cmd.Env = append(cs.cmd.Env, v+"="+os.Getenv(v))
	}
	cs.leaseNotify = sync.NewCond(&cs.leaseLock)
	return cs
}

func (cs *Controllers) ListenPort() int {
	return cs.listenPort
}

func (cs *Controllers) ListenURL() string {
	return fmt.Sprintf("https://127.0.0.1:%d", cs.listenPort)
}

func (cs *Controllers) String() string {
	state := "alive"
	if cs.Exited() {
		state = "dead"
	}
	return fmt.Sprintf("%T instance (pid=%d, %s)", *cs, cs.cmd.Process.Pid, state)
}

func (cs *Controllers) Start() error {
	var logWriter *bufio.Writer
	stderr, err := cs.cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdout, err := cs.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cs.cmd.Start(); err != nil {
		return err
	}
	if cs.outputDir != "" {
		logFilePath := fmt.Sprintf("%s/logs/os-controllers-%d.log", cs.outputDir, cs.cmd.Process.Pid)
		fmt.Fprintf(g.GinkgoWriter, "Starting new controllers instance listening on port %d, logging to file %s\n", cs.listenPort, logFilePath)
		fileWriter, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		logWriter = bufio.NewWriter(fileWriter)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "Starting new controllers instance listening on port %d\n", cs.listenPort)
		logWriter = bufio.NewWriter(new(NullWriter))
	}
	// copy stdout
	go func() {
		io.Copy(logWriter, stdout)
	}()
	// process and copy stderr
	go func() {
		errBuffed := bufio.NewReader(stderr)
		for {
			line, err := errBuffed.ReadBytes('\n')
			if err != nil {
				cs.leaseLock.Lock()
				defer cs.leaseLock.Unlock()
				if err == io.EOF {
					cs.leaseErr = fmt.Errorf("%s: unexpectedly terminated", cs.String())
				} else {
					cs.leaseErr = fmt.Errorf("%s: failed to read stderr: %v", cs.String(), err)
				}
				cs.leaseNotify.Broadcast()
				return
			}
			logWriter.Write(line)
			submatch := reLeaseID.FindSubmatch(line)
			if len(submatch) > 1 && len(submatch[1]) > 0 {
				fmt.Fprintf(g.GinkgoWriter, "%s tries to acquire controllers lease as %q\n", cs.String(), submatch[1])
				cs.leaseLock.Lock()
				cs.lease = string(submatch[1])
				cs.leaseNotify.Broadcast()
				cs.leaseLock.Unlock()
				logWriter.Flush()
				io.Copy(logWriter, errBuffed)
				break
			}
		}
	}()
	return nil
}

func (cs *Controllers) GetLeaseID(wait bool) (string, error) {
	cs.leaseLock.Lock()
	defer cs.leaseLock.Unlock()
	if wait {
		for cs.lease == "" && cs.leaseErr == nil {
			cs.leaseNotify.Wait()
		}
	}
	return cs.lease, cs.leaseErr
}

func (cs *Controllers) Kill() error {
	fmt.Fprintf(g.GinkgoWriter, "Killing %s\n", cs.String())
	err := cs.cmd.Process.Kill()
	if err == nil {
		err = cs.cmd.Wait()
	}
	return err
}

func (cs *Controllers) Wait() error {
	return cs.cmd.Wait()
}

func (cs *Controllers) WaitWithTimeout(timeout time.Duration) error {
	ec := make(chan error)
	go func() {
		err := cs.Wait()
		ec <- err
	}()
	select {
	case err := <-ec:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("Timeout occured while waiting for %s to terminate", cs.String())
	}
}

func (cs *Controllers) Exited() bool {
	return cs.cmd != nil && cs.cmd.ProcessState != nil
}

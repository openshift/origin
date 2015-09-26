package util

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"
	g "github.com/onsi/ginkgo"
	configapi "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

const (
	EtcdLeasePath = `/openshift.io/leases/controllers`
)

// CMonitor allows to start and stop controllers instances,
// distinguish currently active one by looking up leaseID in etcd storage and
// allows for updating and deleting of the lease.
type CMonitor struct {
	configPath      string
	listenPortStart int
	OutputDir       string
	LeaseTTL        uint64
	EtcdClient      *etcdclient.Client
	alive           []*Controllers
	allocatedPorts  *list.List
}

func NewCMonitor(configPath string, numControllers, listenPortStart int, outputDir string) (*CMonitor, error) {
	if listenPortStart <= 0 {
		return nil, fmt.Errorf("Expected listenPortStart > 0, not %d", listenPortStart)
	}

	masterConfig, err := configapi.ReadAndResolveMasterConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read and resolve master config %q: %v", configPath, err)
	}
	leaseTTL := masterConfig.ControllerLeaseTTL
	if leaseTTL <= 0 {
		return nil, fmt.Errorf("Expected ControllerLeaseTTL > 0, not %d", leaseTTL)
	}
	etcdc, err := etcdclient.NewTLSClient(
		[]string{"https://" + masterConfig.EtcdConfig.Address},
		masterConfig.EtcdClientInfo.ClientCert.CertFile,
		masterConfig.EtcdClientInfo.ClientCert.KeyFile,
		masterConfig.EtcdClientInfo.CA)
	if err != nil {
		return nil, fmt.Errorf("Failed to instantiate etcd client for %q: %v", masterConfig.EtcdConfig.Address, err)
	}

	if outputDir == "" && os.Getenv("TMPDIR") != "" {
		outputDir = os.Getenv("TMPDIR")
	}
	logsDir := filepath.Join(outputDir, "logs")
	if err := os.MkdirAll(logsDir, 0644); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("Failed to create log dir: %v", err)
	}
	g.GinkgoWriter.Write([]byte(fmt.Sprintf("Logging controllers outputs to %s\n", logsDir)))

	mgr := &CMonitor{
		configPath:      configPath,
		listenPortStart: listenPortStart,
		OutputDir:       outputDir,
		LeaseTTL:        uint64(leaseTTL),
		EtcdClient:      etcdc,
		allocatedPorts:  list.New(),
	}
	for i := 0; i < numControllers; i++ {
		ctrls := NewControllers(mgr.allocateNewPort(), configPath, outputDir)
		if err := ctrls.Start(); err != nil {
			for j := 0; j < len(mgr.alive); j++ {
				mgr.alive[j].Kill()
			}
			return nil, err
		}
		mgr.alive = append(mgr.alive, ctrls)
	}
	return mgr, nil
}

func (m *CMonitor) allocateNewPort() int {
	prev := m.listenPortStart - 1
	for elem := m.allocatedPorts.Front(); elem != nil; elem = elem.Next() {
		if prev < elem.Value.(int)-1 {
			m.allocatePort(prev + 1)
			return prev + 1
		}
		prev = elem.Value.(int)
	}
	m.allocatePort(prev + 1)
	return prev + 1
}

func (m *CMonitor) allocatePort(port int) {
	for elem := m.allocatedPorts.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(int) == port {
			return
		}
		if elem.Value.(int) > port {
			m.allocatedPorts.InsertBefore(port, elem)
			break
		}
	}
	m.allocatedPorts.PushBack(port)
}

func (m *CMonitor) freePort(port int) {
	for elem := m.allocatedPorts.Front(); elem != m.allocatedPorts.Back(); elem = elem.Next() {
		if elem.Value.(int) == port {
			m.allocatedPorts.Remove(elem)
			break
		}
		if elem.Value.(int) > port {
			break
		}
	}
}

func (m *CMonitor) markDead(ctrlList ...*Controllers) {
	for _, ctrls := range ctrlList {
		for i := 0; i < len(m.alive); i++ {
			if m.alive[i].cmd == ctrls.cmd {
				ctrls.Wait()
				m.freePort(ctrls.ListenPort())
				m.alive = append(m.alive[0:i], m.alive[i+1:]...)
				break
			}
		}
	}
}

// checkAlive iterates over a list of running instances of controllers and
// releases anu terminated. It returns True if at least one such instance was
// found.
func (m *CMonitor) checkAlive() bool {
	modified := false
	for i := 0; i < len(m.alive); {
		if m.alive[i].Exited() {
			m.markDead(m.alive[i])
			modified = true
			continue
		}
		i++
	}
	return modified
}

func (m *CMonitor) Len() int {
	m.checkAlive()
	return len(m.alive)
}

func (m *CMonitor) StartNewInstance() (*Controllers, error) {
	ctrls := NewControllers(m.allocateNewPort(), m.configPath, m.OutputDir)
	if err := ctrls.Start(); err != nil {
		return nil, err
	}
	m.alive = append(m.alive, ctrls)
	return ctrls, nil
}

func (m *CMonitor) GetActive() (*Controllers, error) {
	latest, err := m.EtcdClient.Get(EtcdLeasePath, false, false)
	if err != nil {
		return nil, fmt.Errorf("Failed to obtain a lease: %v", err)
	}
	leaseID := latest.Node.Value
	for i := 0; i < len(m.alive); {
		ctrls := m.alive[i]
		if ctrls.Exited() {
			m.markDead(ctrls)
			continue
		}
		lid, err := ctrls.GetLeaseID(false)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "Failed to get a leaseID of %s: %v\n", ctrls.String(), err)
		} else if leaseID == lid {
			return ctrls, nil
		}
		i++
	}
	return nil, nil
}

func (m *CMonitor) WaitForActive(timeout time.Duration) (*Controllers, time.Duration, error) {
	var latestResp *etcdclient.Response
	start := time.Now()
	findInstance := func(leaseID string) *Controllers {
		var lid string
		for i := 0; i < len(m.alive); {
			if m.alive[i].Exited() {
				m.markDead(m.alive[i])
				continue
			}
			ec := make(chan error)
			go func() {
				var err error
				lid, err = m.alive[i].GetLeaseID(true)
				ec <- err
			}()
			select {
			case err := <-ec:
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "Failed to get a leaseID of %s: %v\n", m.alive[i], err)
				} else if leaseID == lid {
					return m.alive[i]
				}
			case <-time.After(start.Add(timeout).Sub(time.Now())):
				return nil
			}
			i++
		}
		leases := []string{}
		for i := 0; i < len(m.alive); i++ {
			if lease, err := m.alive[i].GetLeaseID(false); err != nil {
				leases = append(leases, lease)
			}
		}
		fmt.Fprintf(g.GinkgoWriter, "Current LeaseID %q does not belong to any controllers instance running: %s\n", leaseID, strings.Join(leases, ", "))
		return nil
	}

	latestResp, err := m.EtcdClient.Get(EtcdLeasePath, false, false)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "key not found") {
		return nil, time.Now().Sub(start), fmt.Errorf("Failed to obtain a lease: %v", err)
	}
	if err == nil {
		if ctrls := findInstance(latestResp.Node.Value); ctrls != nil {
			return ctrls, time.Now().Sub(start), nil
		}
	}
	now := time.Now()
	if start.Add(timeout).Before(now) {
		return nil, now.Sub(start), fmt.Errorf("timeout (%s) occured while waiting for a lease to be set", timeout.String())
	}

Loop:
	for {
		stopChan := make(chan bool)
		responseChan := make(chan *etcdclient.Response)
		errChan := make(chan error)
		index := uint64(0)
		if latestResp != nil {
			index = latestResp.Node.ModifiedIndex + 1
		}
		go func() {
			response, err := m.EtcdClient.Watch(EtcdLeasePath, index, false, nil, stopChan)
			if err != nil {
				errChan <- err
				return
			}
			responseChan <- response
		}()
		select {
		case err := <-errChan:
			return nil, time.Now().Sub(start), err
		case resp := <-responseChan:
			if ctrls := findInstance(resp.Node.Value); ctrls != nil {
				return ctrls, time.Now().Sub(start), nil
			}
			latestResp = resp
		case <-time.After(start.Add(timeout).Sub(time.Now())):
			// Stop the watch and wait for goroutine to terminate
			close(stopChan)
			select {
			case <-errChan:
			case resp := <-responseChan:
				if ctrls := findInstance(resp.Node.Value); ctrls != nil {
					return ctrls, time.Now().Sub(start), nil
				}
			}
			break Loop
		}
	}
	return nil, time.Now().Sub(start), fmt.Errorf("timeout (%s) occured while waiting for an activation of controllers instance", timeout.String())
}

func (m *CMonitor) GetAlive() []*Controllers {
	m.checkAlive()
	res := make([]*Controllers, len(m.alive))
	copy(res, m.alive)
	return res
}

func (m *CMonitor) GetInactive() []*Controllers {
	inactive := []*Controllers{}
	active, _ := m.GetActive()
	for i := 0; i < len(m.alive); {
		ctrls := m.alive[i]
		if ctrls.Exited() {
			m.markDead(ctrls)
			continue
		}
		if active == nil || ctrls.cmd != active.cmd {
			inactive = append(inactive, m.alive[i])
		}
		i++
	}
	return inactive
}

func (m *CMonitor) ReleaseControllers(ctrList ...*Controllers) {
	ctrlsToString := func(l []*Controllers) string {
		cs := make([]string, 0, len(m.alive))
		for _, c := range l {
			cs = append(cs, c.String())
		}
		return strings.Join(cs, ", ")
	}
	fmt.Fprintf(g.GinkgoWriter, "Releasing controllers: (%s)\n", ctrlsToString(ctrList))
	for _, ctrls := range ctrList {
		if !ctrls.Exited() {
			ctrls.Kill()
		}
		m.markDead(ctrls)
	}
}

func (m *CMonitor) DeleteLease() error {
	fmt.Fprintf(g.GinkgoWriter, "Deleting current controllers lease\n")
	_, err := m.EtcdClient.Delete(EtcdLeasePath, false)
	return err
}

func (m *CMonitor) SetLeaseID(leaseID string) error {
	fmt.Fprintf(g.GinkgoWriter, "Setting current controllers leaseID to %q\n", leaseID)
	_, err := m.EtcdClient.Set(EtcdLeasePath, leaseID, m.LeaseTTL)
	return err
}

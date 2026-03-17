package compat_otp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gebn/bmc"
	"github.com/gebn/bmc/pkg/ipmi"
	o "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	BMPoweredOn  = "poweredon"
	BMPoweredOff = "poweredoff"
)

// RDU2Hosts holds the collection of RDU2 hosts and provides methods to manage them
type RDU2Hosts struct {
	hostsMap map[string]*RDU2Host
	mu       sync.RWMutex
}

var (
	rdu2HostsSingleton *RDU2Hosts
	singletonMu        sync.Mutex
)

// RDU2Host models the RDU2 host (partial representation)
type RDU2Host struct {
	Name             string `yaml:"name"`
	BmcAddress       string `yaml:"bmc_address"`
	BmcUser          string `yaml:"bmc_user"`
	BmcPassword      string `yaml:"bmc_pass"`
	BmcForwardedPort uint16 `yaml:"bmc_forwarded_port"`
	Host             string `yaml:"host"`
	JumpHost         string `yaml:"-"`
	MacAddress       string `yaml:"mac"`
	RedfishScheme    string `yaml:"redfish_scheme"`
	RedfishBaseURI   string `yaml:"redfish_base_uri"`
}

// newRDU2Hosts creates a new RDU2Hosts instance by reading the hosts.yaml file
func newRDU2Hosts() (*RDU2Hosts, error) {
	sharedDir := os.Getenv("SHARED_DIR")
	if sharedDir == "" {
		return nil, fmt.Errorf("SHARED_DIR is not set")
	}
	hostsFilePath := filepath.Join(sharedDir, "hosts.yaml")

	yamlBytes, err := os.ReadFile(hostsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts.yaml: %w", err)
	}

	// Unmarshal the yaml into a slice of RDU2Host objects
	var hostsData []RDU2Host
	dec := yaml.NewDecoder(bytes.NewReader(yamlBytes))
	dec.KnownFields(true)
	err = dec.Decode(&hostsData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hosts.yaml: %w", err)
	}

	if len(hostsData) == 0 {
		return nil, fmt.Errorf("hosts.yaml contains no hosts")
	}

	// Convert slice to map of name to RDU2Host objects to allow lookup by name
	hostsMap := make(map[string]*RDU2Host, len(hostsData))
	for i := range hostsData {
		if hostsData[i].Name == "" {
			return nil, fmt.Errorf("hosts.yaml entry at index %d has empty name", i)
		}
		if _, exists := hostsMap[hostsData[i].Name]; exists {
			return nil, fmt.Errorf("duplicate host name %q in hosts.yaml", hostsData[i].Name)
		}
		hostsMap[hostsData[i].Name] = &hostsData[i]
	}

	return &RDU2Hosts{
		hostsMap: hostsMap,
	}, nil
}

// copyMap returns a deep copy of the hosts map to prevent external modifications
func (r *RDU2Hosts) copyMap() map[string]*RDU2Host {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hostsCopy := make(map[string]*RDU2Host, len(r.hostsMap))
	for k, v := range r.hostsMap {
		if v == nil {
			hostsCopy[k] = nil
			continue
		}
		hostCopy := *v
		hostsCopy[k] = &hostCopy
	}
	return hostsCopy
}

// GetRDU2HostsList returns the singleton instance of RDU2Hosts map
// It initializes the singleton on first call by reading the hosts.yaml file
// Returns a copy of the map to prevent external modifications
func GetRDU2HostsList() (map[string]*RDU2Host, error) {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	if rdu2HostsSingleton != nil {
		return rdu2HostsSingleton.copyMap(), nil
	}

	s, err := newRDU2Hosts()
	if err != nil {
		return nil, err
	}
	rdu2HostsSingleton = s
	return s.copyMap(), nil
}

// StopUPIbaremetalInstance power off the BM machine
func (h *RDU2Host) StopUPIbaremetalInstance() error {
	e2e.Logf("UPI baremetal instance :: %v - %v - %v :: Shutting Down", h.Name, h.Host, h.BmcAddress)
	return h.ipmiExec(ipmi.ChassisControlPowerOff)
}

// StartUPIbaremetalInstance power on the BM machine
func (h *RDU2Host) StartUPIbaremetalInstance() error {
	e2e.Logf("UPI baremetal instance :: %v - %v - %v :: Powering on", h.Name, h.Host, h.BmcAddress)
	return h.ipmiExec(ipmi.ChassisControlPowerOn)
}

// GetMachinePowerStatus returns the power status of the BM Machine
func (h *RDU2Host) GetMachinePowerStatus() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	transport, err := h.newTransport(ctx)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to connect to BMC")
	defer transport.Close()

	e2e.Logf("connected to %v (%v - %v - %v) over IPMI v%v", transport.Address(), h.Name, h.Host, h.BmcAddress, transport.Version())
	sess, err := h.newSession(ctx, transport)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create IPMI session")
	defer sess.Close(ctx)
	status, err := sess.GetChassisStatus(ctx)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to get chassis status:: %v :: %v",
		h.BmcAddress, err))
	if status.PoweredOn {
		e2e.Logf("UPI baremetal instance :: %v - %v - %v :: poweredOn", h.Name, h.Host, h.BmcAddress)
		return BMPoweredOn, nil
	}
	e2e.Logf("UPI baremetal instance :: %v - %v - %v :: poweredOff", h.Name, h.Host, h.BmcAddress)
	return BMPoweredOff, nil
}

// newTransport creates a new IPMI transport. It is the caller's responsibility to close it.
func (h *RDU2Host) newTransport(ctx context.Context) (bmc.SessionlessTransport, error) {
	transport, err := bmc.Dial(ctx, fmt.Sprintf("%s:%d", h.JumpHost, h.BmcForwardedPort))
	o.Expect(err).NotTo(o.HaveOccurred())
	return transport, nil
}

// newSession creates a new IPMI session. It is the caller's responsibility to close it.
func (h *RDU2Host) newSession(ctx context.Context, transport bmc.SessionlessTransport) (bmc.Session, error) {
	sess, err := transport.NewSession(ctx, &bmc.SessionOpts{
		Username:          h.BmcUser,
		Password:          []byte(h.BmcPassword),
		MaxPrivilegeLevel: ipmi.PrivilegeLevelAdministrator,
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return sess, nil
}

// ipmiExec executes the specified IPMI command on the BMC
func (h *RDU2Host) ipmiExec(cmd ipmi.ChassisControl) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	transport, err := h.newTransport(ctx)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to connect to BMC")
	defer transport.Close()

	e2e.Logf("connected to %v (%v - %v - %v) over IPMI v%v", transport.Address(), h.Name, h.Host, h.BmcAddress, transport.Version())
	sess, err := h.newSession(ctx, transport)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create IPMI session")
	defer sess.Close(ctx)

	privup := &ipmi.SetSessionPrivilegeLevelCmd{
		Req: ipmi.SetSessionPrivilegeLevelReq{
			PrivilegeLevel: ipmi.PrivilegeLevelAdministrator,
		},
	}
	err = bmc.ValidateResponse(sess.SendCommand(ctx, privup))
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to set privilege level")

	if err := sess.ChassisControl(ctx, cmd); err != nil {
		return err
	}
	return nil
}

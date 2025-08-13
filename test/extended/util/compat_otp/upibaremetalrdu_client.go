package compat_otp

import (
	"context"
	"fmt"
	"time"

	"github.com/gebn/bmc"
	"github.com/gebn/bmc/pkg/ipmi"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	BMPoweredOn  = "poweredon"
	BMPoweredOff = "poweredoff"
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

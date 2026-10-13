package compat_otp

import (
	"context"
	"flag"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"

	//"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Vmware is object
type Vmware struct {
	GovmomiURL string
}

// A ByName is vm type object
type ByName []mo.VirtualMachine

func (n ByName) Len() int           { return len(n) }
func (n ByName) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n ByName) Less(i, j int) bool { return n[i].Name < n[j].Name }

// A creating represents constants ...
const (
	PropRuntimePowerState = "summary.runtime.powerState"
	PropConfigTemplate    = "summary.config.template"
)

// Login represents connect and log in to ESX or vCenter ...
func (vmware *Vmware) Login() (*Vmware, *govmomi.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flag.Parse()

	// Parse URL from string
	u, err := url.Parse(vmware.GovmomiURL)
	if err != nil {
		e2e.Logf("Error parsing vmware url")
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	// Connect and log in to ESX or vCenter
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		e2e.Logf("Error in login, please check vmware url\n")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return vmware, c
}

// GetVspheresInstance represents to get vmware instance.
func (vmware *Vmware) GetVspheresInstance(c *govmomi.Client, vmInstance string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pc := property.DefaultCollector(c.Client)
	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmInstance)
	if err != nil {
		return "", err
	}

	var vms []mo.VirtualMachine
	err = pc.RetrieveOne(ctx, vm.Reference(), []string{"name"}, &vms)
	if err != nil {
		return "", err
	}

	e2e.Logf("Virtual machines found: %v", vms[0].Name)
	return vms[0].Name, nil
}

// GetVspheresInstanceState represents get instance state.
func (vmware *Vmware) GetVspheresInstanceState(c *govmomi.Client, vmInstance string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pc := property.DefaultCollector(c.Client)
	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmInstance)
	if err != nil {
		return "", err
	}

	var vms []mo.VirtualMachine
	err = pc.RetrieveOne(ctx, vm.Reference(), []string{"summary"}, &vms)
	if err != nil {
		return "", err
	}

	e2e.Logf("%s: %s\n", vms[0].Summary.Config.Name, vms[0].Summary.Runtime.PowerState)
	return string(vms[0].Summary.Runtime.PowerState), nil
}

// StopVsphereInstance represents stopping instance ...
func (vmware *Vmware) StopVsphereInstance(c *govmomi.Client, vmInstance string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pc := property.DefaultCollector(c.Client)
	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmInstance)
	if err != nil {
		return err
	}
	// power off VM after some time
	go func() {
		time.Sleep(time.Millisecond * 100)
		vm.PowerOff(ctx)
	}()

	return property.Wait(ctx, pc, vm.Reference(), []string{"runtime.powerState"}, func(changes []types.PropertyChange) bool {
		for _, change := range changes {
			state := change.Val.(types.VirtualMachinePowerState)
			e2e.Logf("%v", state)
			if state == types.VirtualMachinePowerStatePoweredOff {
				return true
			}
		}
		// continue polling
		return false
	})
}

// StartVsphereInstance represents starting instance ...
func (vmware *Vmware) StartVsphereInstance(c *govmomi.Client, vmInstance string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pc := property.DefaultCollector(c.Client)
	vm, err := find.NewFinder(c.Client).VirtualMachine(ctx, vmInstance)
	if err != nil {
		return err
	}

	go func() {
		// power on VM after some time
		time.Sleep(time.Millisecond * 100)
		vm.PowerOn(ctx)
	}()

	return property.Wait(ctx, pc, vm.Reference(), []string{"runtime.powerState"}, func(changes []types.PropertyChange) bool {
		for _, change := range changes {
			state := change.Val.(types.VirtualMachinePowerState)
			e2e.Logf("%v", state)
			if state == types.VirtualMachinePowerStatePoweredOn {
				return true
			}
		}
		// continue polling
		return false
	})
}

// GetVsphereConnectionLogout get connection logout
func (vmware *Vmware) GetVsphereConnectionLogout(c *govmomi.Client) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errLogout := c.Logout(ctx)
	if errLogout != nil {
		return errLogout
	}
	return nil
}

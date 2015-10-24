package firewalld

import (
	"fmt"

	"github.com/godbus/dbus"
)

type Interface struct {
	obj         *dbus.Object
	reloadFuncs []func()
}

const (
	IPv4 = "ipv4"
	IPv6 = "ipv6"
)

const (
	firewalldName      = "org.fedoraproject.FirewallD1"
	firewalldPath      = "/org/fedoraproject/FirewallD1"
	firewalldInterface = "org.fedoraproject.FirewallD1"
)

func New() *Interface {
	bus, err := dbus.SystemBus()
	if err != nil {
		return &Interface{}
	}

	fw := &Interface{
		obj: bus.Object(firewalldName, dbus.ObjectPath(firewalldPath)),
	}

	go fw.dbusSignalHandler(bus)

	return fw
}

func (fw *Interface) IsRunning() bool {
	if fw.obj == nil {
		return false
	}

	var zone string
	err := fw.obj.Call(firewalldInterface+".getDefaultZone", 0).Store(&zone)
	return err == nil
}

func (fw *Interface) AddRule(ipv, table, chain string, priority int, rule []string) error {
	return fw.obj.Call(firewalldInterface+".direct.addRule", 0, ipv, table, chain, int32(priority), rule).Store()
}

func (fw *Interface) EnsureRule(ipv, table, chain string, priority int, rule []string) error {
	var exists bool

	err := fw.obj.Call(firewalldInterface+".direct.queryRule", 0, ipv, table, chain, int32(priority), rule).Store(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return fw.AddRule(ipv, table, chain, priority, rule)
}

func (fw *Interface) AddReloadFunc(reloadFunc func()) {
	fw.reloadFuncs = append(fw.reloadFuncs, reloadFunc)
}

func (fw *Interface) dbusSignalHandler(bus *dbus.Conn) {
	rule := fmt.Sprintf("type='signal',sender='%s',path='%s',interface='%s',member='Reloaded'", firewalldName, firewalldPath, firewalldInterface)
	bus.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	rule = fmt.Sprintf("type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged',path='/org/freedesktop/DBus',sender='org.freedesktop.DBus',arg0='%s'", firewalldName)
	bus.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	signal := make(chan *dbus.Signal, 10)
	bus.Signal(signal)

	for s := range signal {
		if s.Name == "org.freedesktop.DBus.NameOwnerChanged" {
			name := s.Body[0].(string)
			new_owner := s.Body[2].(string)

			if name != firewalldName || len(new_owner) == 0 {
				continue
			}

			// FirewallD startup (specifically the part where it deletes
			// all existing iptables rules) may not yet be complete when
			// we get this signal, so make a dummy request to it to
			// synchronize.
			fw.obj.Call(firewalldInterface+".getDefaultZone", 0)

			fw.reload()
		} else if s.Name == firewalldInterface+".Reloaded" {
			fw.reload()
		}
	}
}

func (fw *Interface) reload() {
	for _, f := range fw.reloadFuncs {
		f()
	}
}

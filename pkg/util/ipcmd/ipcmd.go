// Package ipcmd provides a wrapper around the "ip" command.
package ipcmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util/exec"
)

var addressRegexp *regexp.Regexp

func init() {
	addressRegexp = regexp.MustCompile("inet ([0-9.]*/[0-9]*) ")
}

type Transaction struct {
	execer exec.Interface
	link   string
	err    error
}

// NewTransaction begins a new transaction for a given interface. If an error
// occurs at any step in the transaction, it will be recorded until
// EndTransaction(), and any further calls on the transaction will be ignored.
func NewTransaction(execer exec.Interface, link string) *Transaction {
	return &Transaction{execer: execer, link: link}
}

func (tx *Transaction) exec(args []string) (string, error) {
	if tx.err != nil {
		return "", tx.err
	}

	ipcmdPath, err := tx.execer.LookPath("ip")
	if err != nil {
		tx.err = fmt.Errorf("ip is not installed")
		return "", tx.err
	}

	glog.V(5).Infof("Executing: %s %s", ipcmdPath, strings.Join(args, " "))
	var output []byte
	output, tx.err = tx.execer.Command(ipcmdPath, args...).CombinedOutput()
	if tx.err != nil {
		glog.V(5).Infof("Error executing %s: %s", ipcmdPath, string(output))
	}
	return string(output), tx.err
}

// AddLink creates the interface associated with the transaction, optionally
// with additional properties.
func (tx *Transaction) AddLink(args ...string) {
	tx.exec(append([]string{"link", "add", tx.link}, args...))
}

// DeleteLink deletes the interface associated with the transaction. (It is an
// error if the interface does not exist.)
func (tx *Transaction) DeleteLink() {
	tx.exec([]string{"link", "del", tx.link})
}

// SetLink sets the indicated properties on the interface.
func (tx *Transaction) SetLink(args ...string) {
	tx.exec(append([]string{"link", "set", tx.link}, args...))
}

// AddAddress adds an address to the interface.
func (tx *Transaction) AddAddress(cidr string, args ...string) {
	tx.exec(append([]string{"addr", "add", cidr, "dev", tx.link}, args...))
}

// DeleteAddress deletes an address from the interface. (It is an error if the
// address does not exist.)
func (tx *Transaction) DeleteAddress(cidr string, args ...string) {
	tx.exec(append([]string{"addr", "del", cidr, "dev", tx.link}, args...))
}

// GetAddresses returns the IPv4 addresses associated with the interface. Since
// this function has a return value, it also returns an error immediately if an
// error occurs.
func (tx *Transaction) GetAddresses() ([]string, error) {
	out, err := tx.exec(append([]string{"addr", "show", "dev", tx.link}))
	if err != nil {
		return nil, err
	}

	matches := addressRegexp.FindAllStringSubmatch(out, -1)
	addrs := make([]string, len(matches))
	for i, match := range matches {
		addrs[i] = match[1]
	}
	return addrs, nil
}

// AddRoute adds a route to the interface.
func (tx *Transaction) AddRoute(cidr string, args ...string) {
	tx.exec(append([]string{"route", "add", cidr, "dev", tx.link}, args...))
}

// DeleteRoute deletes a route from the interface. (It is an error if the route
// does not exist.)
func (tx *Transaction) DeleteRoute(cidr string, args ...string) {
	tx.exec(append([]string{"route", "del", cidr, "dev", tx.link}, args...))
}

// GetRoutes returns the IPv4 routes associated with the interface (as an array
// of route descriptions in the format output by "ip route show"). Since this
// function has a return value, it also returns an error immediately if an error
// occurs.
func (tx *Transaction) GetRoutes() ([]string, error) {
	out, err := tx.exec(append([]string{"route", "show", "dev", tx.link}))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	return lines[:len(lines)-1], nil
}

// AddSlave adds the indicated slave interface to the bridge, bond, or team
// interface associated with the transaction.
func (tx *Transaction) AddSlave(slave string) {
	tx.exec([]string{"link", "set", slave, "master", tx.link})
}

// AddSlave remotes the indicated slave interface from the bridge, bond, or team
// interface associated with the transaction. (No error occurs if the interface
// is not actually a slave of the transaction interface.)
func (tx *Transaction) DeleteSlave(slave string) {
	tx.exec([]string{"link", "set", slave, "nomaster"})
}

// IgnoreError causes any error on the transaction to be discarded, in case you
// don't care about errors from a particular command and want further commands
// to be executed regardless.
func (tx *Transaction) IgnoreError() {
	tx.err = nil
}

// EndTransaction ends a transaction and returns any error that occurred during
// the transaction. You should not use the transaction again after calling this
// function.
func (tx *Transaction) EndTransaction() error {
	err := tx.err
	tx.err = nil
	return err
}

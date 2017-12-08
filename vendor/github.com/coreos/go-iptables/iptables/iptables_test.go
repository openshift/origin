// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iptables

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"
)

func TestProto(t *testing.T) {
	ipt, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if ipt.Proto() != ProtocolIPv4 {
		t.Fatalf("Expected default protocol IPv4, got %v", ipt.Proto())
	}

	ip4t, err := NewWithProtocol(ProtocolIPv4)
	if err != nil {
		t.Fatalf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
	}
	if ip4t.Proto() != ProtocolIPv4 {
		t.Fatalf("Expected protocol IPv4, got %v", ip4t.Proto())
	}

	ip6t, err := NewWithProtocol(ProtocolIPv6)
	if err != nil {
		t.Fatalf("NewWithProtocol(ProtocolIPv6) failed: %v", err)
	}
	if ip6t.Proto() != ProtocolIPv6 {
		t.Fatalf("Expected protocol IPv6, got %v", ip6t.Proto())
	}
}

func randChain(t *testing.T) string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		t.Fatalf("Failed to generate random chain name: %v", err)
	}

	return "TEST-" + n.String()
}

func contains(list []string, value string) bool {
	for _, val := range list {
		if val == value {
			return true
		}
	}
	return false
}

// mustTestableIptables returns a list of ip(6)tables handles with various
// features enabled & disabled, to test compatability.
// We used to test noWait as well, but that was removed as of iptables v1.6.0
func mustTestableIptables() []*IPTables {
	ipt, err := New()
	if err != nil {
		panic(fmt.Sprintf("New failed: %v", err))
	}
	ip6t, err := NewWithProtocol(ProtocolIPv6)
	if err != nil {
		panic(fmt.Sprintf("NewWithProtocol(ProtocolIPv6) failed: %v", err))
	}
	ipts := []*IPTables{ipt, ip6t}

	// ensure we check one variant without built-in checking
	if ipt.hasCheck {
		i := *ipt
		i.hasCheck = false
		ipts = append(ipts, &i)

		i6 := *ip6t
		i6.hasCheck = false
		ipts = append(ipts, &i6)
	} else {
		panic("iptables on this machine is too old -- missing -C")
	}
	return ipts
}

func TestChain(t *testing.T) {
	for _, ipt := range mustTestableIptables() {
		runChainTests(t, ipt)
	}
}

func runChainTests(t *testing.T, ipt *IPTables) {
	t.Logf("testing %s (hasWait=%t, hasCheck=%t)", ipt.path, ipt.hasWait, ipt.hasCheck)

	chain := randChain(t)

	// Saving the list of chains before executing tests
	originaListChain, err := ipt.ListChains("filter")
	if err != nil {
		t.Fatalf("ListChains of Initial failed: %v", err)
	}

	// chain shouldn't exist, this will create new
	err = ipt.ClearChain("filter", chain)
	if err != nil {
		t.Fatalf("ClearChain (of missing) failed: %v", err)
	}

	// chain should be in listChain
	listChain, err := ipt.ListChains("filter")
	if err != nil {
		t.Fatalf("ListChains failed: %v", err)
	}
	if !contains(listChain, chain) {
		t.Fatalf("ListChains doesn't contain the new chain %v", chain)
	}

	// chain now exists
	err = ipt.ClearChain("filter", chain)
	if err != nil {
		t.Fatalf("ClearChain (of empty) failed: %v", err)
	}

	// put a simple rule in
	err = ipt.Append("filter", chain, "-s", "0/0", "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// can't delete non-empty chain
	err = ipt.DeleteChain("filter", chain)
	if err == nil {
		t.Fatalf("DeleteChain of non-empty chain did not fail")
	}

	err = ipt.ClearChain("filter", chain)
	if err != nil {
		t.Fatalf("ClearChain (of non-empty) failed: %v", err)
	}

	// rename the chain
	newChain := randChain(t)
	err = ipt.RenameChain("filter", chain, newChain)
	if err != nil {
		t.Fatalf("RenameChain failed: %v", err)
	}

	// chain empty, should be ok
	err = ipt.DeleteChain("filter", newChain)
	if err != nil {
		t.Fatalf("DeleteChain of empty chain failed: %v", err)
	}

	// check that chain is fully gone and that state similar to initial one
	listChain, err = ipt.ListChains("filter")
	if err != nil {
		t.Fatalf("ListChains failed: %v", err)
	}
	if !reflect.DeepEqual(originaListChain, listChain) {
		t.Fatalf("ListChains mismatch: \ngot  %#v \nneed %#v", originaListChain, listChain)
	}
}

func TestRules(t *testing.T) {
	for _, ipt := range mustTestableIptables() {
		runRulesTests(t, ipt)
	}
}

func runRulesTests(t *testing.T, ipt *IPTables) {
	t.Logf("testing %s (hasWait=%t, hasCheck=%t)", getIptablesCommand(ipt.Proto()), ipt.hasWait, ipt.hasCheck)

	var address1, address2, subnet1, subnet2 string
	if ipt.Proto() == ProtocolIPv6 {
		address1 = "2001:db8::1/128"
		address2 = "2001:db8::2/128"
		subnet1 = "2001:db8:a::/48"
		subnet2 = "2001:db8:b::/48"
	} else {
		address1 = "203.0.113.1/32"
		address2 = "203.0.113.2/32"
		subnet1 = "192.0.2.0/24"
		subnet2 = "198.51.100.0/24"
	}

	chain := randChain(t)

	// chain shouldn't exist, this will create new
	err := ipt.ClearChain("filter", chain)
	if err != nil {
		t.Fatalf("ClearChain (of missing) failed: %v", err)
	}

	err = ipt.Append("filter", chain, "-s", subnet1, "-d", address1, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	err = ipt.AppendUnique("filter", chain, "-s", subnet1, "-d", address1, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("AppendUnique failed: %v", err)
	}

	err = ipt.Append("filter", chain, "-s", subnet2, "-d", address1, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	err = ipt.Insert("filter", chain, 2, "-s", subnet2, "-d", address2, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	err = ipt.Insert("filter", chain, 1, "-s", subnet1, "-d", address2, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	err = ipt.Delete("filter", chain, "-s", subnet1, "-d", address2, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	err = ipt.Append("filter", chain, "-s", address1, "-d", subnet2, "-j", "ACCEPT")
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	rules, err := ipt.List("filter", chain)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	expected := []string{
		"-N " + chain,
		"-A " + chain + " -s " + subnet1 + " -d " + address1 + " -j ACCEPT",
		"-A " + chain + " -s " + subnet2 + " -d " + address2 + " -j ACCEPT",
		"-A " + chain + " -s " + subnet2 + " -d " + address1 + " -j ACCEPT",
		"-A " + chain + " -s " + address1 + " -d " + subnet2 + " -j ACCEPT",
	}

	if !reflect.DeepEqual(rules, expected) {
		t.Fatalf("List mismatch: \ngot  %#v \nneed %#v", rules, expected)
	}

	rules, err = ipt.ListWithCounters("filter", chain)
	if err != nil {
		t.Fatalf("ListWithCounters failed: %v", err)
	}

	expected = []string{
		"-N " + chain,
		"-A " + chain + " -s " + subnet1 + " -d " + address1 + " -c 0 0 -j ACCEPT",
		"-A " + chain + " -s " + subnet2 + " -d " + address2 + " -c 0 0 -j ACCEPT",
		"-A " + chain + " -s " + subnet2 + " -d " + address1 + " -c 0 0 -j ACCEPT",
		"-A " + chain + " -s " + address1 + " -d " + subnet2 + " -c 0 0 -j ACCEPT",
	}

	if !reflect.DeepEqual(rules, expected) {
		t.Fatalf("ListWithCounters mismatch: \ngot  %#v \nneed %#v", rules, expected)
	}

	stats, err := ipt.Stats("filter", chain)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	opt := "--"
	if ipt.proto == ProtocolIPv6 {
		opt = "  "
	}

	expectedStats := [][]string{
		{"0", "0", "ACCEPT", "all", opt, "*", "*", subnet1, address1, ""},
		{"0", "0", "ACCEPT", "all", opt, "*", "*", subnet2, address2, ""},
		{"0", "0", "ACCEPT", "all", opt, "*", "*", subnet2, address1, ""},
		{"0", "0", "ACCEPT", "all", opt, "*", "*", address1, subnet2, ""},
	}

	if !reflect.DeepEqual(stats, expectedStats) {
		t.Fatalf("Stats mismatch: \ngot  %#v \nneed %#v", stats, expectedStats)
	}

	// Clear the chain that was created.
	err = ipt.ClearChain("filter", chain)
	if err != nil {
		t.Fatalf("Failed to clear test chain: %v", err)
	}

	// Delete the chain that was created
	err = ipt.DeleteChain("filter", chain)
	if err != nil {
		t.Fatalf("Failed to delete test chain: %v", err)
	}
}

// TestError checks that we're OK when iptables fails to execute
func TestError(t *testing.T) {
	ipt, err := New()
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	chain := randChain(t)
	_, err = ipt.List("filter", chain)
	if err == nil {
		t.Fatalf("no error with invalid params")
	}
	switch e := err.(type) {
	case *Error:
		// OK
	default:
		t.Fatalf("expected type iptables.Error, got %t", e)
	}

	// Now set an invalid binary path
	ipt.path = "/does-not-exist"

	_, err = ipt.ListChains("filter")

	if err == nil {
		t.Fatalf("no error with invalid ipt binary")
	}

	switch e := err.(type) {
	case *os.PathError:
		// OK
	default:
		t.Fatalf("expected type os.PathError, got %t", e)
	}
}

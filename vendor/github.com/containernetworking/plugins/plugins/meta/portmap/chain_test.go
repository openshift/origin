// Copyright 2017 CNI authors
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

package main

import (
	"fmt"
	"math/rand"
	"runtime"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const TABLE = "filter" // We'll monkey around here

// TODO: run these tests in a new namespace
var _ = Describe("chain tests", func() {
	var testChain chain
	var ipt *iptables.IPTables
	var cleanup func()

	BeforeEach(func() {

		// Save a reference to the original namespace,
		// Add a new NS
		currNs, err := ns.GetCurrentNS()
		Expect(err).NotTo(HaveOccurred())

		testNs, err := ns.NewNS()
		Expect(err).NotTo(HaveOccurred())

		tlChainName := fmt.Sprintf("cni-test-%d", rand.Intn(10000000))
		chainName := fmt.Sprintf("cni-test-%d", rand.Intn(10000000))

		testChain = chain{
			table:       TABLE,
			name:        chainName,
			entryRule:   []string{"-d", "203.0.113.1"},
			entryChains: []string{tlChainName},
		}

		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
		Expect(err).NotTo(HaveOccurred())

		runtime.LockOSThread()
		err = testNs.Set()
		Expect(err).NotTo(HaveOccurred())

		err = ipt.ClearChain(TABLE, tlChainName) // This will create the chain
		if err != nil {
			currNs.Set()
			Expect(err).NotTo(HaveOccurred())
		}

		cleanup = func() {
			if ipt == nil {
				return
			}
			ipt.ClearChain(TABLE, testChain.name)
			ipt.ClearChain(TABLE, tlChainName)
			ipt.DeleteChain(TABLE, testChain.name)
			ipt.DeleteChain(TABLE, tlChainName)
			currNs.Set()
		}

	})

	It("creates and destroys a chain", func() {
		defer cleanup()

		tlChainName := testChain.entryChains[0]

		// add an extra rule to the test chain to make sure it's not touched
		err := ipt.Append(TABLE, tlChainName, "-m", "comment", "--comment",
			"canary value", "-j", "ACCEPT")
		Expect(err).NotTo(HaveOccurred())

		// Create the chain
		chainRules := [][]string{
			{"-m", "comment", "--comment", "test 1", "-j", "RETURN"},
			{"-m", "comment", "--comment", "test 2", "-j", "RETURN"},
		}
		err = testChain.setup(ipt, chainRules)
		Expect(err).NotTo(HaveOccurred())

		// Verify the chain exists
		ok := false
		chains, err := ipt.ListChains(TABLE)
		Expect(err).NotTo(HaveOccurred())
		for _, chain := range chains {
			if chain == testChain.name {
				ok = true
				break
			}
		}
		if !ok {
			Fail("Could not find created chain")
		}

		// Check that the entry rule was created
		haveRules, err := ipt.List(TABLE, tlChainName)
		Expect(err).NotTo(HaveOccurred())
		Expect(haveRules).To(Equal([]string{
			"-N " + tlChainName,
			"-A " + tlChainName + " -d 203.0.113.1/32 -j " + testChain.name,
			"-A " + tlChainName + ` -m comment --comment "canary value" -j ACCEPT`,
		}))

		// Check that the chain and rule was created
		haveRules, err = ipt.List(TABLE, testChain.name)
		Expect(err).NotTo(HaveOccurred())
		Expect(haveRules).To(Equal([]string{
			"-N " + testChain.name,
			"-A " + testChain.name + ` -m comment --comment "test 1" -j RETURN`,
			"-A " + testChain.name + ` -m comment --comment "test 2" -j RETURN`,
		}))

		err = testChain.teardown(ipt)
		Expect(err).NotTo(HaveOccurred())

		tlRules, err := ipt.List(TABLE, tlChainName)
		Expect(err).NotTo(HaveOccurred())
		Expect(tlRules).To(Equal([]string{
			"-N " + tlChainName,
			"-A " + tlChainName + ` -m comment --comment "canary value" -j ACCEPT`,
		}))

		chains, err = ipt.ListChains(TABLE)
		Expect(err).NotTo(HaveOccurred())
		for _, chain := range chains {
			if chain == testChain.name {
				Fail("chain was not deleted")
			}
		}
	})

	It("creates chains idempotently", func() {
		defer cleanup()

		// Create the chain
		chainRules := [][]string{
			{"-m", "comment", "--comment", "test", "-j", "RETURN"},
		}
		err := testChain.setup(ipt, chainRules)
		Expect(err).NotTo(HaveOccurred())

		// Create it again!
		err = testChain.setup(ipt, chainRules)
		Expect(err).NotTo(HaveOccurred())

		// Make sure there are only two rules
		// (the first rule is an -N because go-iptables
		rules, err := ipt.List(TABLE, testChain.name)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(rules)).To(Equal(2))

	})

	It("deletes chains idempotently", func() {
		defer cleanup()

		// Create the chain
		chainRules := [][]string{
			{"-m", "comment", "--comment", "test", "-j", "RETURN"},
		}
		err := testChain.setup(ipt, chainRules)
		Expect(err).NotTo(HaveOccurred())

		err = testChain.teardown(ipt)
		Expect(err).NotTo(HaveOccurred())

		chains, err := ipt.ListChains(TABLE)
		for _, chain := range chains {
			if chain == testChain.name {
				Fail("Chain was not deleted")
			}
		}

		err = testChain.teardown(ipt)
		Expect(err).NotTo(HaveOccurred())
		chains, err = ipt.ListChains(TABLE)
		for _, chain := range chains {
			if chain == testChain.name {
				Fail("Chain was not deleted")
			}
		}
	})
})

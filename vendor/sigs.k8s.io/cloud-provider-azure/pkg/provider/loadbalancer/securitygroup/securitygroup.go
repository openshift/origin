/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package securitygroup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/go-logr/logr"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/cloud-provider-azure/pkg/consts"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer/fnutil"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer/iputil"
)

const (
	SecurityRuleNamePrefix = "k8s-azure-lb"
	SecurityRuleNameSep    = "_"
)

// Refer: https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/azure-subscription-service-limits?toc=%2Fazure%2Fvirtual-network%2Ftoc.json#azure-resource-manager-virtual-networking-limits
const (
	MaxSecurityRulesPerGroup              = 1_000
	MaxSecurityRuleSourceIPsPerGroup      = 4_000
	MaxSecurityRuleDestinationIPsPerGroup = 4_000
)

const (
	ServiceTagInternet = "Internet"
)

var (
	ErrInvalidSecurityGroup                                = fmt.Errorf("invalid SecurityGroup object")
	ErrSecurityRulePriorityExhausted                       = fmt.Errorf("security rule priority exhausted")
	ErrSecurityRuleSourceAddressesNotFromSameIPFamily      = fmt.Errorf("security rule source addresses must be from the same IP family")
	ErrSecurityRuleDestinationAddressesNotFromSameIPFamily = fmt.Errorf("security rule destination addresses must be from the same IP family")
	ErrSecurityRuleSourceAndDestinationNotFromSameIPFamily = fmt.Errorf("security rule source addresses and destination addresses must be from the same IP family")
)

// RuleHelper manages security rules within a security group.
type RuleHelper struct {
	logger   logr.Logger
	sg       *network.SecurityGroup
	snapshot []byte

	// helper map to store the security rules
	// name -> security rule
	rules      map[string]*network.SecurityRule
	priorities map[int32]string
}

func NewSecurityGroupHelper(logger logr.Logger, sg *network.SecurityGroup) (*RuleHelper, error) {
	if sg == nil ||
		sg.Name == nil ||
		sg.SecurityGroupPropertiesFormat == nil ||
		sg.SecurityRules == nil {
		// In the real world, this should never happen.
		// Anyway, defensively check it.
		return nil, ErrInvalidSecurityGroup
	}
	var (
		rules      = make(map[string]*network.SecurityRule, len(*sg.SecurityGroupPropertiesFormat.SecurityRules))
		priorities = make(map[int32]string, len(*sg.SecurityGroupPropertiesFormat.SecurityRules))
	)
	for i := range *sg.SecurityGroupPropertiesFormat.SecurityRules {
		rule := (*sg.SecurityGroupPropertiesFormat.SecurityRules)[i]
		rules[*rule.Name] = &rule
		priorities[*rule.Priority] = *rule.Name
	}

	snapshot := makeSecurityGroupSnapshot(sg)

	return &RuleHelper{
		logger: logger.WithName("RuleHelper"),
		sg:     sg,

		rules:      rules,
		priorities: priorities,
		snapshot:   snapshot,
	}, nil
}

type rulePriorityPrefer string

const (
	rulePriorityPreferFromStart rulePriorityPrefer = "from_start"
	rulePriorityPreferFromEnd   rulePriorityPrefer = "from_end"
)

// nextRulePriority returns the next available priority for a new rule.
// It takes a preference for whether to start from the beginning or end of the priority range.
func (helper *RuleHelper) nextRulePriority(prefer rulePriorityPrefer) (int32, error) {
	var (
		init, end = consts.LoadBalancerMinimumPriority, consts.LoadBalancerMaximumPriority
		delta     = 1
	)
	if prefer == rulePriorityPreferFromEnd {
		init, end, delta = end-1, init-1, -1
	}

	for init != end {
		p := int32(init)
		if _, found := helper.priorities[p]; found {
			init += delta
			continue
		}
		return p, nil
	}

	return 0, ErrSecurityRulePriorityExhausted
}

// getOrCreateRule returns an existing rule or create a new one if it doesn't exist.
func (helper *RuleHelper) getOrCreateRule(name string, priorityPrefer rulePriorityPrefer) (*network.SecurityRule, error) {
	logger := helper.logger.WithName("getOrCreateRule").WithValues("rule-name", name)

	if rule, found := helper.rules[name]; found {
		logger.V(4).Info("Found an existing rule", "priority", *rule.Priority)
		return rule, nil
	}

	priority, err := helper.nextRulePriority(priorityPrefer)
	if err != nil {
		// NOTE: right now it won't happen because the number of rules is limited.
		//       maxPriority[4096] - minPriority[500] > maxSecurityRulesPerGroup[1000]
		helper.logger.Error(err, "Failed to get an available rule priority")
		return nil, err
	}
	rule := &network.SecurityRule{
		Name: ptr.To(name),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Priority: ptr.To(priority),
		},
	}

	helper.rules[name] = rule
	helper.priorities[priority] = name

	logger.V(4).Info("Adding a new rule", "rule-name", name, "priority", priority)

	return rule, nil
}

// addAllowRule adds a rule that allows certain traffic.
func (helper *RuleHelper) addAllowRule(
	protocol network.SecurityRuleProtocol,
	ipFamily iputil.Family,
	srcPrefixes []string,
	dstPrefixes []string,
	dstPorts []int32,
) error {
	name := GenerateAllowSecurityRuleName(protocol, ipFamily, srcPrefixes, dstPorts)
	rule, err := helper.getOrCreateRule(name, rulePriorityPreferFromStart)
	if err != nil {
		return err
	}
	dstPortRanges := fnutil.Map(func(p int32) string { return strconv.FormatInt(int64(p), 10) }, dstPorts)
	sort.Strings(dstPortRanges)

	rule.Protocol = protocol
	rule.Access = network.SecurityRuleAccessAllow
	rule.Direction = network.SecurityRuleDirectionInbound
	{
		// Source
		if len(srcPrefixes) == 1 {
			rule.SourceAddressPrefix = ptr.To(srcPrefixes[0])
		} else {
			rule.SourceAddressPrefixes = ptr.To(srcPrefixes)
		}
		rule.SourcePortRange = ptr.To("*")
	}
	{
		// Destination
		addresses := append(ListDestinationPrefixes(rule), dstPrefixes...)
		SetDestinationPrefixes(rule, addresses)
		rule.DestinationPortRanges = ptr.To(dstPortRanges)
	}

	helper.logger.V(4).Info("Patched a rule for allow", "rule-name", name)

	return nil
}

// AddRuleForAllowedServiceTag adds a rule for traffic from a certain service tag.
func (helper *RuleHelper) AddRuleForAllowedServiceTag(
	serviceTag string,
	protocol network.SecurityRuleProtocol,
	dstAddresses []netip.Addr,
	dstPorts []int32,
) error {
	if !iputil.AreAddressesFromSameFamily(dstAddresses) {
		return ErrSecurityRuleDestinationAddressesNotFromSameIPFamily
	}

	var (
		ipFamily    = iputil.FamilyOfAddr(dstAddresses[0])
		srcPrefixes = []string{serviceTag}
		dstPrefixes = fnutil.Map(func(ip netip.Addr) string { return ip.String() }, dstAddresses)
	)

	helper.logger.V(4).Info("Patching a rule for allowed service tag", "ip-family", ipFamily)

	return helper.addAllowRule(protocol, ipFamily, srcPrefixes, dstPrefixes, dstPorts)
}

// AddRuleForAllowedIPRanges adds a rule for traffic from certain IP ranges.
func (helper *RuleHelper) AddRuleForAllowedIPRanges(
	ipRanges []netip.Prefix,
	protocol network.SecurityRuleProtocol,
	dstAddresses []netip.Addr,
	dstPorts []int32,
) error {
	if !iputil.ArePrefixesFromSameFamily(ipRanges) {
		return ErrSecurityRuleSourceAddressesNotFromSameIPFamily
	}
	if !iputil.AreAddressesFromSameFamily(dstAddresses) {
		return ErrSecurityRuleDestinationAddressesNotFromSameIPFamily
	}
	if ipRanges[0].Addr().Is4() != dstAddresses[0].Is4() {
		return ErrSecurityRuleSourceAndDestinationNotFromSameIPFamily
	}

	var (
		ipFamily    = iputil.FamilyOfAddr(ipRanges[0].Addr())
		srcPrefixes = fnutil.Map(func(ip netip.Prefix) string { return ip.String() }, ipRanges)
		dstPrefixes = fnutil.Map(func(ip netip.Addr) string { return ip.String() }, dstAddresses)
	)

	helper.logger.V(4).Info("Patching a rule for allowed IP ranges", "ip-family", ipFamily)

	return helper.addAllowRule(protocol, ipFamily, srcPrefixes, dstPrefixes, dstPorts)
}

// AddRuleForDenyAll adds a rule to deny all traffic from the given destination addresses.
// NOTE:
// This rule is to limit the traffic inside the VNet.
// The traffic out of the VNet is already limited by rule `DenyAllInBound`.
func (helper *RuleHelper) AddRuleForDenyAll(dstAddresses []netip.Addr) error {
	if !iputil.AreAddressesFromSameFamily(dstAddresses) {
		return ErrSecurityRuleDestinationAddressesNotFromSameIPFamily
	}

	var (
		ipFamily = iputil.FamilyOfAddr(dstAddresses[0])
		ruleName = GenerateDenyAllSecurityRuleName(ipFamily)
	)

	helper.logger.V(4).Info("Patching a rule for deny all", "ip-family", ipFamily)

	rule, err := helper.getOrCreateRule(ruleName, rulePriorityPreferFromEnd)
	if err != nil {
		return err
	}
	rule.Protocol = network.SecurityRuleProtocolAsterisk
	rule.Access = network.SecurityRuleAccessDeny
	rule.Direction = network.SecurityRuleDirectionInbound
	{
		// Source
		rule.SourceAddressPrefix = ptr.To("*")
		rule.SourcePortRange = ptr.To("*")
	}
	{
		// Destination
		addresses := fnutil.Map(func(ip netip.Addr) string { return ip.String() }, dstAddresses)
		addresses = append(addresses, ListDestinationPrefixes(rule)...)
		SetDestinationPrefixes(rule, addresses)
		rule.DestinationPortRange = ptr.To("*")
	}

	helper.logger.V(4).Info("Patched a rule for deny all", "rule-name", ptr.To(rule.Name))

	return nil
}

// RemoveDestinationFromRules removes the given destination addresses from rules that match the given protocol and ports is in the retainDstPorts list.
// It may add a new rule if the original rule needs to be split.
func (helper *RuleHelper) RemoveDestinationFromRules(
	protocol network.SecurityRuleProtocol,
	dstPrefixes []string,
	retainDstPorts []int32,
) error {
	logger := helper.logger.WithName("RemoveDestinationFromRules").WithValues("protocol", protocol, "num-dst-prefixes", len(dstPrefixes))
	logger.V(10).Info("Cleaning destination from SecurityGroup")

	for _, rule := range helper.rules {
		if rule.Priority == nil {
			continue
		}
		priority := *rule.Priority
		if priority < consts.LoadBalancerMinimumPriority || consts.LoadBalancerMaximumPriority < priority {
			logger.V(4).Info("Skip rule with not-in-range priority", "rule-name", *rule.Name, "priority", priority)
			continue
		}

		if rule.Protocol != protocol {
			continue
		}

		if err := helper.removeDestinationFromRule(rule, dstPrefixes, retainDstPorts); err != nil {
			logger.Error(err, "Failed to remove destination from rule", "rule-name", *rule.Name)
		}
	}

	return nil
}

func (helper *RuleHelper) removeDestinationFromRule(rule *network.SecurityRule, prefixes []string, retainDstPorts []int32) error {
	logger := helper.logger.WithName("removeDestinationFromRule").
		WithValues("security-rule-name", rule.Name)

	var (
		prefixIndex     = fnutil.IndexSet(prefixes) // Used to check whether the prefix should be removed.
		currentPrefixes = ListDestinationPrefixes(rule)

		expectedPrefixes = fnutil.RemoveIf(func(p string) bool { return prefixIndex[p] }, currentPrefixes) // The prefixes to keep.
		targetPrefixes   = fnutil.Intersection(currentPrefixes, prefixes)                                  // The prefixes to remove.
	)

	// Clean DenyAll rule
	if rule.Access == network.SecurityRuleAccessDeny && len(retainDstPorts) == 0 {
		// Update the prefixes
		SetDestinationPrefixes(rule, expectedPrefixes)

		return nil
	}

	// Clean Allow rule
	currentPorts, err := ListDestinationPortRanges(rule)
	if err != nil {
		// Skip the rule with invalid destination port ranges.
		// NOTE: cloud-provider would not create allow rules with `*` or `4000-5000` as destination port ranges.
		logger.Info("Skip because it contains `*` or port-ranges as destination port ranges.")
		return nil
	}
	var (
		expectedPorts = fnutil.Intersection(currentPorts, retainDstPorts) // The ports to keep.
	)

	if len(targetPrefixes) == 0 || len(currentPorts) == len(expectedPorts) {
		return nil
	}

	// Update the prefixes
	SetDestinationPrefixes(rule, expectedPrefixes)

	if len(expectedPorts) == 0 {
		// No additional ports are expected, no more actions are needed.
		return nil
	}

	// There are additional ports are expected, need to create a new rule for them.
	addr, err := netip.ParseAddr(prefixes[0])
	if err != nil {
		logger.Error(err, "Failed to parse dst IP address", "dst-ip", prefixes[0])
		return fmt.Errorf("parse prefix as IP address %q: %w", prefixes[0], err)
	}
	ipFamily := iputil.FamilyOfAddr(addr)
	return helper.addAllowRule(rule.Protocol, ipFamily, ListSourcePrefixes(rule), prefixes, expectedPorts)
}

// SecurityGroup returns the underlying SecurityGroup object and a bool indicating whether any changes were made to the RuleHelper.
func (helper *RuleHelper) SecurityGroup() (*network.SecurityGroup, bool, error) {
	var (
		rv    = helper.sg
		rules = make([]network.SecurityRule, 0, len(helper.rules))
	)
	for _, r := range helper.rules {
		var (
			dstAddresses = ListDestinationPrefixes(r)
			dstASGs      = ptr.Deref(r.DestinationApplicationSecurityGroups, []network.ApplicationSecurityGroup{})
		)

		if len(dstAddresses) == 0 && len(dstASGs) == 0 {
			// Skip the rule without destination prefixes.
			continue
		}
		rules = append(rules, *r)
	}

	rv.SecurityRules = &rules

	var (
		snapshot = makeSecurityGroupSnapshot(rv)
		updated  = !bytes.Equal(helper.snapshot, snapshot)
		nRules   int
		nSrcIPs  int
		nDstIPs  int
	)
	{
		// Check whether the SecurityGroup exceeds the limit.
		for _, rule := range *rv.SecurityRules {
			nRules++
			if rule.SourceAddressPrefixes != nil {
				nSrcIPs += len(*rule.SourceAddressPrefixes)
			}
			if rule.SourceAddressPrefix != nil {
				nSrcIPs++
			}
			if rule.DestinationAddressPrefixes != nil {
				nDstIPs += len(*rule.DestinationAddressPrefixes)
			}
			if rule.DestinationAddressPrefix != nil {
				nDstIPs++
			}
		}
		helper.logger.V(10).Info("Checking the number of rules and IP addresses", "num-rules", nRules, "num-src-ips", nSrcIPs, "num-dst-ips", nDstIPs)
		if nRules > MaxSecurityRulesPerGroup {
			return nil, false, fmt.Errorf("exceeds the maximum number of rules (%d > %d)", nRules, MaxSecurityRulesPerGroup)
		}
		if nSrcIPs > MaxSecurityRuleSourceIPsPerGroup {
			return nil, false, fmt.Errorf("exceeds the maximum number of source IP addresses (%d > %d)", nSrcIPs, MaxSecurityRuleSourceIPsPerGroup)
		}
		if nDstIPs > MaxSecurityRuleDestinationIPsPerGroup {
			return nil, false, fmt.Errorf("exceeds the maximum number of destination IP addresses (%d > %d)", nDstIPs, MaxSecurityRuleDestinationIPsPerGroup)
		}
	}

	return rv, updated, nil
}

// makeSecurityGroupSnapshot returns a byte array as the snapshot of the given SecurityGroup.
// It's used to check if the SecurityGroup had been changed.
func makeSecurityGroupSnapshot(sg *network.SecurityGroup) []byte {
	sort.SliceStable(*sg.SecurityGroupPropertiesFormat.SecurityRules, func(i, j int) bool {
		return *(*sg.SecurityGroupPropertiesFormat.SecurityRules)[i].Priority < *(*sg.SecurityGroupPropertiesFormat.SecurityRules)[j].Priority
	})
	snapshot, _ := json.Marshal(sg)
	return snapshot
}

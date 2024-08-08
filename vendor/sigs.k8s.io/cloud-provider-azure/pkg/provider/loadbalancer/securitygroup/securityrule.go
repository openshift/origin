/*
Copyright 2024 The Kubernetes Authors.

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
	"crypto/md5" //nolint:gosec
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer/fnutil"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer/iputil"
)

// GenerateAllowSecurityRuleName returns the AllowInbound rule name based on the given rule properties.
func GenerateAllowSecurityRuleName(
	protocol network.SecurityRuleProtocol,
	ipFamily iputil.Family,
	srcPrefixes []string,
	dstPorts []int32,
) string {
	var ruleID string
	{
		dstPortRanges := fnutil.Map(func(p int32) string { return strconv.FormatInt(int64(p), 10) }, dstPorts)
		// Generate rule ID from protocol, source prefixes and destination port ranges.
		sort.Strings(srcPrefixes)
		sort.Strings(dstPortRanges)

		v := strings.Join([]string{
			string(protocol),
			strings.Join(srcPrefixes, ","),
			strings.Join(dstPortRanges, ","),
		}, "_")

		h := md5.New() //nolint:gosec
		h.Write([]byte(v))

		ruleID = fmt.Sprintf("%x", h.Sum(nil))
	}

	return strings.Join([]string{SecurityRuleNamePrefix, "allow", string(ipFamily), ruleID}, SecurityRuleNameSep)
}

// GenerateDenyAllSecurityRuleName returns the DenyInbound rule name based on the given rule properties.
func GenerateDenyAllSecurityRuleName(ipFamily iputil.Family) string {
	return strings.Join([]string{SecurityRuleNamePrefix, "deny-all", string(ipFamily)}, SecurityRuleNameSep)
}

// NormalizeSecurityRuleAddressPrefixes normalizes the given rule address prefixes.
func NormalizeSecurityRuleAddressPrefixes(vs []string) []string {
	// Remove redundant addresses.
	indexes := make(map[string]bool, len(vs))
	for _, v := range vs {
		indexes[v] = true
	}
	rv := make([]string, 0, len(indexes))
	for k := range indexes {
		rv = append(rv, k)
	}
	sort.Strings(rv)
	return rv
}

// NormalizeDestinationPortRanges normalizes the given destination port ranges.
func NormalizeDestinationPortRanges(dstPorts []int32) []string {
	rv := fnutil.Map(func(p int32) string { return strconv.FormatInt(int64(p), 10) }, dstPorts)
	sort.Strings(rv)
	return rv
}

func ListSourcePrefixes(r *network.SecurityRule) []string {
	var rv []string
	if r.SourceAddressPrefix != nil {
		rv = append(rv, *r.SourceAddressPrefix)
	}
	if r.SourceAddressPrefixes != nil {
		rv = append(rv, *r.SourceAddressPrefixes...)
	}
	return rv
}

func ListDestinationPrefixes(r *network.SecurityRule) []string {
	var rv []string
	if r.DestinationAddressPrefix != nil {
		rv = append(rv, *r.DestinationAddressPrefix)
	}
	if r.DestinationAddressPrefixes != nil {
		rv = append(rv, *r.DestinationAddressPrefixes...)
	}
	return rv
}

func SetDestinationPrefixes(r *network.SecurityRule, prefixes []string) {
	ps := NormalizeSecurityRuleAddressPrefixes(prefixes)
	if len(ps) == 1 {
		r.DestinationAddressPrefix = &ps[0]
		r.DestinationAddressPrefixes = nil
	} else {
		r.DestinationAddressPrefix = nil
		r.DestinationAddressPrefixes = &ps
	}
}

func ListDestinationPortRanges(r *network.SecurityRule) ([]int32, error) {
	var values []string
	if r.DestinationPortRange != nil {
		values = append(values, *r.DestinationPortRange)
	}
	if r.DestinationPortRanges != nil {
		values = append(values, *r.DestinationPortRanges...)
	}

	rv := make([]int32, 0, len(values))
	for _, v := range values {
		p, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse port range %q: %w", v, err)
		}
		rv = append(rv, int32(p))
	}

	return rv, nil
}

func ProtocolFromKubernetes(p v1.Protocol) (network.SecurityRuleProtocol, error) {
	switch p {
	case v1.ProtocolTCP:
		return network.SecurityRuleProtocolTCP, nil
	case v1.ProtocolUDP:
		return network.SecurityRuleProtocolUDP, nil
	case v1.ProtocolSCTP:
		return network.SecurityRuleProtocolAsterisk, nil
	}
	return "", fmt.Errorf("unsupported protocol %s", p)
}

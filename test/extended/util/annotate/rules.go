package main

// Rules defined here are additive to the rules already defined for
// kube e2e tests in openshift/kubernetes. The kube rules are
// vendored via the following file:
//
//   vendor/k8s.io/kubernetes/openshift-hack/e2e/annotate/rules.go
//
// Rules that are needed to pass the upstream e2e test suite in a
// "default OCP CI" configuration (eg, AWS or GCP, openshift-sdn) must
// be added to openshift/kubernetes to allow CI to pass there, and
// then vendored back into origin. Rules that only apply to
// "non-default" configurations (other clouds, other network
// providers) should be added here.

var (
	testMaps = map[string][]string{
		// tests that require a local host
		"[Local]": {
			// Doesn't work on scaled up clusters
			`\[Feature:ImagePrune\]`,
		},
		// alpha features that are not gated
		"[Disabled:Alpha]": {},
		// tests for features that are not implemented in openshift
		"[Disabled:Unimplemented]": {},
		// tests that rely on special configuration that we do not yet support
		"[Disabled:SpecialConfig]": {},
		// tests that are known broken and need to be fixed upstream or in openshift
		// always add an issue here
		"[Disabled:Broken]": {
			`should idle the service and DeploymentConfig properly`,       // idling with a single service and DeploymentConfig
			`should answer endpoint and wildcard queries for the cluster`, // currently not supported by dns operator https://github.com/openshift/cluster-dns-operator/issues/43

			// https://bugzilla.redhat.com/show_bug.cgi?id=1988272
			`\[sig-network\] Networking should provide Internet connection for containers \[Feature:Networking-IPv6\]`,
			`\[sig-network\] Networking should provider Internet connection for containers using DNS`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1908645
			`\[sig-network\] Networking Granular Checks: Services should function for service endpoints using hostNetwork`,
			`\[sig-network\] Networking Granular Checks: Services should function for pod-Service\(hostNetwork\)`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1952460
			`\[sig-network\] Firewall rule control plane should not expose well-known ports`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1952457
			`\[sig-node\] crictl should be able to run crictl on the node`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1945091
			`\[Feature:GenericEphemeralVolume\]`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1953478
			`\[sig-storage\] Dynamic Provisioning Invalid AWS KMS key should report an error and create no PV`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1957894
			`\[sig-node\] Container Runtime blackbox test when running a container with a new image should be able to pull from private registry with secret`,

			// The new NetworkPolicy test suite is extremely resource
			// intensive and causes itself and other concurrently-running
			// tests to be flaky.
			// https://bugzilla.redhat.com/show_bug.cgi?id=1980141
			`\[sig-network\] Netpol `,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1996128
			`\[sig-network\] \[Feature:IPv6DualStack\] should have ipv4 and ipv6 node podCIDRs`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=2004074
			`\[sig-network-edge\]\[Feature:Idling\] Unidling should work with TCP \(while idling\)`,
		},
		// tests that may work, but we don't support them
		"[Disabled:Unsupported]": {
			// Skip vSphere-specific storage tests. The standard in-tree storage tests for vSphere
			// (prefixed with `In-tree Volumes [Driver: vsphere]`) are enough for testing this plugin.
			// https://bugzilla.redhat.com/show_bug.cgi?id=2019115
			`\[sig-storage\].*\[Feature:vsphere\]`,
		},
		// tests too slow to be part of conformance
		"[Slow]": {},
		// tests that are known flaky
		"[Flaky]": {
			`openshift mongodb replication creating from a template`, // flaking on deployment
		},
		// tests that must be run without competition
		"[Serial]": {},
		// tests that can't be run in parallel with a copy of itself
		"[Serial:Self]": {
			`\[sig-network\] HostPort validates that there is no conflict between pods with same hostPort but different hostIP and protocol`,
		},
		"[Skipped:azure]": {},
		"[Skipped:ovirt]": {},
		"[Skipped:gce]":   {},

		// These tests are skipped when openshift-tests needs to use a proxy to reach the
		// cluster -- either because the test won't work while proxied, or because the test
		// itself is testing a functionality using it's own proxy.
		"[Skipped:Proxy]": {
			// These tests setup their own proxy, which won't work when we need to access the
			// cluster through a proxy.
			`\[sig-cli\] Kubectl client Simple pod should support exec through an HTTP proxy`,
			`\[sig-cli\] Kubectl client Simple pod should support exec through kubectl proxy`,

			// Kube currently uses the x/net/websockets pkg, which doesn't work with proxies.
			// See: https://github.com/kubernetes/kubernetes/pull/103595
			`\[sig-node\] Pods should support retrieving logs from the container over websockets`,
			`\[sig-cli\] Kubectl Port forwarding With a server listening on localhost should support forwarding over websockets`,
			`\[sig-cli\] Kubectl Port forwarding With a server listening on 0.0.0.0 should support forwarding over websockets`,
			`\[sig-node\] Pods should support remote command execution over websockets`,
		},
		"[Skipped:SingleReplicaTopology]": {
			`\[sig-apps\] Daemon set \[Serial\] should rollback without unnecessary restarts \[Conformance\]`,
			`\[sig-node\] NoExecuteTaintManager Single Pod \[Serial\] doesn't evict pod with tolerations from tainted nodes`,
			`\[sig-node\] NoExecuteTaintManager Single Pod \[Serial\] eventually evict pod with finite tolerations from tainted nodes`,
			`\[sig-node\] NoExecuteTaintManager Single Pod \[Serial\] evicts pods from tainted nodes`,
			`\[sig-node\] NoExecuteTaintManager Single Pod \[Serial\] removing taint cancels eviction \[Disruptive\] \[Conformance\]`,
			`\[sig-node\] NoExecuteTaintManager Multiple Pods \[Serial\] evicts pods with minTolerationSeconds \[Disruptive\] \[Conformance\]`,
			`\[sig-node\] NoExecuteTaintManager Multiple Pods \[Serial\] only evicts pods without tolerations from tainted nodes`,
			`\[sig-cli\] Kubectl client Kubectl taint \[Serial\] should remove all the taints with the same key off a node`,
		},

		// Tests that don't pass without internet access
		"[Skipped:Disconnected]": {
			// Internet access required
			`\[sig-builds\]\[Feature:Builds\] clone repository using git:// protocol  should clone using git:// if no proxy is configured`,
			`\[sig-builds\]\[Feature:Builds\] result image should have proper labels set  S2I build from a template should create a image from "test-s2i-build.json" template with proper Docker labels`,
			`\[sig-builds\]\[Feature:Builds\] s2i build with a quota  Building from a template should create an s2i build with a quota and run it`,
			`\[sig-builds\]\[Feature:Builds\] s2i build with a root user image should create a root build and pass with a privileged SCC`,
			`\[sig-builds\]\[Feature:Builds\]\[timing\] capture build stages and durations  should record build stages and durations for docker`,
			`\[sig-builds\]\[Feature:Builds\]\[timing\] capture build stages and durations  should record build stages and durations for s2i`,
			`\[sig-builds\]\[Feature:Builds\]\[valueFrom\] process valueFrom in build strategy environment variables  should successfully resolve valueFrom in s2i build environment variables`,
			`\[sig-builds\]\[Feature:Builds\]\[volumes\] should mount given secrets and configmaps into the build pod for source strategy builds`,
			`\[sig-builds\]\[Feature:Builds\]\[volumes\] should mount given secrets and configmaps into the build pod for docker strategy builds`,
			`\[sig-cli\] oc debug ensure it works with image streams`,
			`\[sig-cli\] oc builds complex build start-build`,
			`\[sig-cli\] oc builds complex build webhooks CRUD`,
			`\[sig-cli\] oc builds new-build`,
			`\[sig-devex\] check registry.redhat.io is available and samples operator can import sample imagestreams run sample related validations`,
			`\[sig-devex\]\[Feature:Templates\] templateinstance readiness test  should report failed soon after an annotated objects has failed`,
			`\[sig-devex\]\[Feature:Templates\] templateinstance readiness test  should report ready soon after all annotated objects are ready`,
			`\[sig-operator\] an end user can use OLM can subscribe to the operator`,
			`\[sig-network\] Networking should provide Internet connection for containers`,
		},

		// tests that don't pass under openshift-sdn NetworkPolicy mode are specified
		// in the rules file in openshift/kubernetes, not here.

		// tests that don't pass under openshift-sdn multitenant mode
		"[Skipped:Network/OpenShiftSDN/Multitenant]": {
			`\[Feature:NetworkPolicy\]`, // not compatible with multitenant mode
		},
		// tests that don't pass under OVN Kubernetes
		"[Skipped:Network/OVNKubernetes]": {
			// https://jira.coreos.com/browse/SDN-510: OVN-K doesn't support session affinity
			`\[sig-network\] Networking Granular Checks: Services should function for client IP based session affinity: http`,
			`\[sig-network\] Networking Granular Checks: Services should function for client IP based session affinity: udp`,
			`\[sig-network\] Services should be able to switch session affinity for NodePort service`,
			`\[sig-network\] Services should be able to switch session affinity for service with type clusterIP`,
			`\[sig-network\] Services should have session affinity work for NodePort service`,
			`\[sig-network\] Services should have session affinity work for service with type clusterIP`,
			`\[sig-network\] Services should have session affinity timeout work for NodePort service`,
			`\[sig-network\] Services should have session affinity timeout work for service with type clusterIP`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1996097 - [Feature:IPv6DualStack] tests are failing in dualstack
			// https://jira.coreos.com/browse/SDN-510: OVN-K doesn't support session affinity
			`\[sig-network\] \[Feature:IPv6DualStack\] Granular Checks: Services Secondary IP Family \[LinuxOnly\] should function for client IP based session affinity: http`,
			`\[sig-network\] \[Feature:IPv6DualStack\] Granular Checks: Services Secondary IP Family \[LinuxOnly\] should function for client IP based session affinity: udp`,

			// ovn-kubernetes does not support named ports
			`NetworkPolicy.*named port`,

			// ovn-kubernetes does not support internal traffic policy
			`\[Feature:ServiceInternalTrafficPolicy\]`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1989169: unidling tests are flaky under ovn-kubernetes
			`Unidling should work with TCP`,
			`Unidling should handle many TCP connections`,
		},
		"[Skipped:ibmcloud]": {
			// skip Gluster tests (not supported on ROKS worker nodes)
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825009 - e2e: skip Glusterfs-related tests upstream for rhel7 worker nodes
			`\[Driver: gluster\]`,
			`GlusterFS`,
			`GlusterDynamicProvisioner`,

			// Nodes in ROKS have access to secrets in the cluster to handle encryption
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825013 - ROKS: worker nodes have access to secrets in the cluster
			`\[sig-auth\] \[Feature:NodeAuthorizer\] Getting a non-existent configmap should exit with the Forbidden error, not a NotFound error`,
			`\[sig-auth\] \[Feature:NodeAuthorizer\] Getting a non-existent secret should exit with the Forbidden error, not a NotFound error`,
			`\[sig-auth\] \[Feature:NodeAuthorizer\] Getting a secret for a workload the node has access to should succeed`,
			`\[sig-auth\] \[Feature:NodeAuthorizer\] Getting an existing configmap should exit with the Forbidden error`,
			`\[sig-auth\] \[Feature:NodeAuthorizer\] Getting an existing secret should exit with the Forbidden error`,

			// Access to node external address is blocked from pods within a ROKS cluster by Calico
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825016 - e2e: NodeAuthenticator tests use both external and internal addresses for node
			`\[sig-auth\] \[Feature:NodeAuthenticator\] The kubelet's main port 10250 should reject requests with no credentials`,
			`\[sig-auth\] \[Feature:NodeAuthenticator\] The kubelet can delegate ServiceAccount tokens to the API server`,

			// Calico is allowing the request to timeout instead of returning 'REFUSED'
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825021 - ROKS: calico SDN results in a request timeout when accessing services with no endpoints
			`\[sig-network\] Services should be rejected when no endpoints exist`,

			// Mode returned by RHEL7 worker contains an extra character not expected by the test: dgtrwx vs dtrwx
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825024 - e2e: Failing test - HostPath should give a volume the correct mode
			`\[sig-storage\] HostPath should give a volume the correct mode`,

			// Currently ibm-master-proxy-static and imbcloud-block-storage-plugin tolerate all taints
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825027
			`\[Feature:Platform\] Managed cluster should ensure control plane operators do not make themselves unevictable`,
		},
	}

	// labelExcludes temporarily block tests out of a specific suite
	labelExcludes = map[string][]string{}
)

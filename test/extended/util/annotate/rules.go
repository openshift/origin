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
		// tests that rely on special configuration that we do not yet support
		// tests that are known broken and need to be fixed upstream or in openshift
		// always add an issue here
		"[Disabled:Broken]": {
			`should answer endpoint and wildcard queries for the cluster`, // currently not supported by dns operator https://github.com/openshift/cluster-dns-operator/issues/43
			// https://bugzilla.redhat.com/show_bug.cgi?id=1908677
			`SCTP \[Feature:SCTP\] \[LinuxOnly\] should create a Pod with SCTP HostPort`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1908645
			`\[sig-network\] Networking Granular Checks: Services should function for service endpoints using hostNetwork`,
		},
		// tests that may work, but we don't support them
		"[Disabled:Unsupported]": {},
		// tests too slow to be part of conformance
		"[Slow]": {},
		// tests that are known flaky
		"[Flaky]": {
			`openshift mongodb replication creating from a template`, // flaking on deployment
		},
		// tests that must be run without competition
		"[Serial]":        {},
		"[Skipped:azure]": {},
		"[Skipped:ovirt]": {},
		"[Skipped:gce]":   {},

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

			// ovn-kubernetes does not support named ports
			`NetworkPolicy.*named port`,
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
		"[Skipped:Disruptive]": {
			// DR testing induces ungraceful termination by forcefully shutting down nodes
			`\[sig-api-machinery\]\[Feature:APIServer\]\[Late\] kube-apiserver terminates within graceful termination period`,
			`\[sig-api-machinery\]\[Feature:APIServer\]\[Late\] kubelet terminates kube-apiserver gracefully`,

			// DR testing results in an excessive amount of alerts
			`\[sig-instrumentation\]\[Late\] Alerts shouldn't exceed the 500 series limit of total series sent via telemetry from each cluster`,

			// Machine API deletion does not properly handle stopped instances on AWS or GCP
			// https://bugzilla.redhat.com/show_bug.cgi?id=1905709
			`\[sig-cluster-lifecycle\]\[Feature:DisasterRecovery\]\[Disruptive\] \[Feature:NodeRecovery\] Cluster should survive master and worker failure and recover with machine health checks`,
		},
	}

	// labelExcludes temporarily block tests out of a specific suite
	labelExcludes = map[string][]string{}
)

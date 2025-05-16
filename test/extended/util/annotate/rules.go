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
		"[Disabled:SpecialConfig]": {
			`\[Feature:Audit\]`,      // Needs special configuration
			`\[Feature:ImageQuota\]`, // Quota isn't turned on by default, we should do that and then reenable these tests
		},
		// tests that are known broken and need to be fixed upstream or in openshift
		// always add an issue here
		"[Disabled:Broken]": {
			`should idle the service and DeploymentConfig properly`,       // idling with a single service and DeploymentConfig
			`should answer endpoint and wildcard queries for the cluster`, // currently not supported by dns operator https://github.com/openshift/cluster-dns-operator/issues/43

			// https://bugzilla.redhat.com/show_bug.cgi?id=1945091
			`\[Feature:GenericEphemeralVolume\]`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1996128
			`\[sig-network\] \[Feature:IPv6DualStack\] should have ipv4 and ipv6 node podCIDRs`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=2004074
			`\[sig-network-edge\]\[Feature:Idling\] Unidling \[apigroup:apps.openshift.io\]\[apigroup:route.openshift.io\] should work with TCP \(while idling\)`,
			`\[sig-network-edge\]\[Feature:Idling\] Unidling with Deployments \[apigroup:route.openshift.io\] should work with TCP \(while idling\)`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=2070929
			`\[sig-network\]\[Feature:EgressIP\]\[apigroup:operator.openshift.io\] \[internal-targets\]`,

			// https://issues.redhat.com/browse/OCPBUGS-967
			`\[sig-network\] IngressClass \[Feature:Ingress\] should prevent Ingress creation if more than 1 IngressClass marked as default`,

			// https://issues.redhat.com/browse/OCPBUGS-3339
			`\[sig-devex\]\[Feature:ImageEcosystem\]\[mysql\]\[Slow\] openshift mysql image Creating from a template should instantiate the template`,
			`\[sig-devex\]\[Feature:ImageEcosystem\]\[mariadb\]\[Slow\] openshift mariadb image Creating from a template should instantiate the template`,

			// https://issues.redhat.com/browse/OCPBUGS-37799
			`\[sig-builds\]\[Feature:Builds\]\[Slow\] can use private repositories as build input build using an HTTP token should be able to clone source code via an HTTP token \[apigroup:build.openshift.io\]`,
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
		"[Serial]": {
			`\[Disruptive\]`,
			`\[sig-network\]\[Feature:EgressIP\]`,
		},
		// tests that can't be run in parallel with a copy of itself
		"[Serial:Self]": {
			`\[sig-network\] HostPort validates that there is no conflict between pods with same hostPort but different hostIP and protocol`,
		},
		// These tests are skipped when openshift-tests needs to use a proxy to reach the
		// cluster -- either because the test won't work while proxied, or because the test
		// itself is testing a functionality using it's own proxy.
		"[Skipped:Proxy]": {
			// These tests are flacky and require internet access
			// See https://bugzilla.redhat.com/show_bug.cgi?id=2019375
			`\[sig-builds\]\[Feature:Builds\] build can reference a cluster service with a build being created from new-build should be able to run a build that references a cluster service`,
			`\[sig-builds\]\[Feature:Builds\] oc new-app should succeed with a --name of 58 characters`,
			`\[sig-arch\] Only known images used by tests`,
		},
		"[Skipped:SingleReplicaTopology]": {
			`should be scheduled on different nodes`,
		},

		"[Feature:Networking-IPv4]": {
			`\[sig-network\]\[Feature:Router\]\[apigroup:route.openshift.io\] when FIPS is disabled the HAProxy router should serve routes when configured with a 1024-bit RSA key`,
		},

		// Tests that don't pass on disconnected, either due to requiring
		// internet access for GitHub (e.g. many of the s2i builds), or
		// because of pullthrough not supporting ICSP (https://bugzilla.redhat.com/show_bug.cgi?id=1918376)
		"[Skipped:Disconnected]": {
			// Internet access required
			`\[sig-builds\]\[Feature:Builds\] clone repository using git:// protocol should clone using git:// if no proxy is configured`,
			`\[sig-builds\]\[Feature:Builds\] result image should have proper labels set S2I build from a template should create a image from "test-s2i-build.json" template with proper Docker labels`,
			`\[sig-builds\]\[Feature:Builds\] s2i build with a quota Building from a template should create an s2i build with a quota and run it`,
			`\[sig-builds\]\[Feature:Builds\] s2i build with a root user image should create a root build and pass with a privileged SCC`,
			`\[sig-builds\]\[Feature:Builds\]\[timing\] capture build stages and durations should record build stages and durations for docker`,
			`\[sig-builds\]\[Feature:Builds\]\[timing\] capture build stages and durations should record build stages and durations for s2i`,
			`\[sig-builds\]\[Feature:Builds\]\[valueFrom\] process valueFrom in build strategy environment variables should successfully resolve valueFrom in s2i build environment variables`,
			`\[sig-builds\]\[Feature:Builds\]\[volumes\] should mount given secrets and configmaps into the build pod for source strategy builds`,
			`\[sig-builds\]\[Feature:Builds\]\[volumes\] should mount given secrets and configmaps into the build pod for docker strategy builds`,
			`\[sig-builds\]\[Feature:Builds\]\[pullsearch\] docker build where the registry is not specified Building from a Dockerfile whose FROM image ref does not specify the image registry should create a docker build that has buildah search from our predefined list of image registries and succeed`,
			`\[sig-cli\] oc debug ensure it works with image streams`,
			`\[sig-cli\] oc builds complex build start-build`,
			`\[sig-cli\] oc builds complex build webhooks CRUD`,
			`\[sig-cli\] oc builds new-build`,
			`\[sig-devex\] check registry.redhat.io is available and samples operator can import sample imagestreams run sample related validations`,
			`\[sig-devex\]\[Feature:Templates\] templateinstance readiness test should report failed soon after an annotated objects has failed`,
			`\[sig-devex\]\[Feature:Templates\] templateinstance readiness test should report ready soon after all annotated objects are ready`,
			`\[sig-operator\] an end user can use OLM can subscribe to the operator`,
			`\[sig-imageregistry\]\[Serial\] Image signature workflow can push a signed image to openshift registry and verify it`,

			// Need to access non-cached images like ruby and mongodb
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with multiple image change triggers should run a successful deployment with a trigger used by different containers`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with multiple image change triggers should run a successful deployment with multiple triggers`,
			`\[sig-apps\] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the AlwaysAllow policy`,
			`\[sig-apps\] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the IfHealthyBudget policy`,

			// ICSP
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs should adhere to Three Laws of Controllers`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs adoption will orphan all RCs and adopt them back when recreated`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs generation should deploy based on a status version bump`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs keep the deployer pod invariant valid should deal with cancellation after deployer pod succeeded`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs paused should disable actions on deployments`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs rolled back should rollback to an older deployment`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs should respect image stream tag reference policy resolve the image pull spec`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs viewing rollout history should print the rollout history`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs when changing image change trigger should successfully trigger from an updated image`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs when run iteratively should only deploy the last deployment`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs when tagging images should successfully tag the deployed image`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with custom deployments should run the custom deployment steps`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with enhanced status should include various info in status`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with env in params referencing the configmap should expand the config map key to a value`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with failing hook should get all logs from retried hooks`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with minimum ready seconds set should not transition the deployment to Complete before satisfied`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with revision history limits should never persist more old deployments than acceptable after being observed by the controller`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs with test deployments should run a deployment to completion and then scale to zero`,
			`\[sig-apps\]\[Feature:DeploymentConfig\] deploymentconfigs won't deploy RC with unresolved images when patched with empty image`,
			`\[sig-arch\] Managed cluster should expose cluster services outside the cluster`,
			`\[sig-arch\]\[Early\] Managed cluster should \[apigroup:config.openshift.io\] start all core operators`,
			`\[sig-auth\]\[Feature:SecurityContextConstraints\] TestPodDefaultCapabilities`,
			`\[sig-builds\]\[Feature:Builds\] Multi-stage image builds should succeed`,
			`\[sig-builds\]\[Feature:Builds\] Optimized image builds should succeed`,
			`\[sig-builds\]\[Feature:Builds\] build can reference a cluster service with a build being created from new-build should be able to run a build that references a cluster service`,
			`\[sig-builds\]\[Feature:Builds\] build have source revision metadata started build should contain source revision information`,
			`\[sig-builds\]\[Feature:Builds\] build with empty source started build should build even with an empty source in build config`,
			`\[sig-builds\]\[Feature:Builds\] build without output image building from templates should create an image from a S2i template without an output image reference defined`,
			`\[sig-builds\]\[Feature:Builds\] build without output image building from templates should create an image from a docker template without an output image reference defined`,
			`\[sig-builds\]\[Feature:Builds\] custom build with buildah being created from new-build should complete build with custom builder image`,
			`\[sig-builds\]\[Feature:Builds\] imagechangetriggers imagechangetriggers should trigger builds of all types`,
			`\[sig-builds\]\[Feature:Builds\] oc new-app should fail with a --name longer than 58 characters`,
			`\[sig-builds\]\[Feature:Builds\] oc new-app should succeed with a --name of 58 characters`,
			`\[sig-builds\]\[Feature:Builds\] oc new-app should succeed with an imagestream`,
			`\[sig-builds\]\[Feature:Builds\] prune builds based on settings in the buildconfig buildconfigs should have a default history limit set when created via the group api`,
			`\[sig-builds\]\[Feature:Builds\] prune builds based on settings in the buildconfig should prune builds after a buildConfig change`,
			`\[sig-builds\]\[Feature:Builds\] prune builds based on settings in the buildconfig should prune canceled builds based on the failedBuildsHistoryLimit setting`,
			`\[sig-builds\]\[Feature:Builds\] prune builds based on settings in the buildconfig should prune completed builds based on the successfulBuildsHistoryLimit setting`,
			`\[sig-builds\]\[Feature:Builds\] prune builds based on settings in the buildconfig should prune errored builds based on the failedBuildsHistoryLimit setting`,
			`\[sig-builds\]\[Feature:Builds\] prune builds based on settings in the buildconfig should prune failed builds based on the failedBuildsHistoryLimit setting`,
			`\[sig-builds\]\[Feature:Builds\] result image should have proper labels set Docker build from a template should create a image from "test-docker-build.json" template with proper Docker labels`,
			`\[sig-builds\]\[Feature:Builds\] verify /run filesystem contents are writeable using a simple Docker Strategy Build`,
			`\[sig-builds\]\[Feature:Builds\] verify /run filesystem contents do not have unexpected content using a simple Docker Strategy Build`,
			`\[sig-builds\]\[Feature:Builds\]\[pullsecret\] docker build using a pull secret Building from a template should create a docker build that pulls using a secret run it`,
			`\[sig-builds\]\[Feature:Builds\]\[valueFrom\] process valueFrom in build strategy environment variables should fail resolving unresolvable valueFrom in docker build environment variable references`,
			`\[sig-builds\]\[Feature:Builds\]\[valueFrom\] process valueFrom in build strategy environment variables should fail resolving unresolvable valueFrom in sti build environment variable references`,
			`\[sig-builds\]\[Feature:Builds\]\[valueFrom\] process valueFrom in build strategy environment variables should successfully resolve valueFrom in docker build environment variables`,
			`\[sig-builds\]\[Feature:Builds\]\[pullsearch\] docker build where the registry is not specified Building from a Dockerfile whose FROM image ref does not specify the image registry should create a docker build that has buildah search from our predefined list of image registries and succeed`,
			`\[sig-cli\] CLI can run inside of a busybox container`,
			`\[sig-cli\] oc debug deployment configs from a build`,
			`\[sig-cli\] oc debug deployment from a build`,
			`\[sig-cli\] oc rsh specific flags should work well when access to a remote shell`,
			`\[sig-cli\] oc builds get buildconfig`,
			`\[sig-cli\] oc builds patch buildconfig`,
			`\[sig-cluster-lifecycle\] Pods cannot access the /config/master API endpoint`,
			`\[sig-imageregistry\]\[Feature:ImageAppend\] Image append should create images by appending them`,
			`\[sig-imageregistry\]\[Feature:ImageExtract\] Image extract should extract content from an image`,
			`\[sig-imageregistry\]\[Feature:ImageInfo\] Image info should display information about images`,
			`\[sig-imageregistry\]\[Feature:ImageLayers\] Image layer subresource should return layers from tagged images`,
			`\[sig-imageregistry\]\[Feature:ImageTriggers\] Annotation trigger reconciles after the image is overwritten`,
			`\[sig-imageregistry\]\[Feature:Image\] oc tag should change image reference for internal images`,
			`\[sig-imageregistry\]\[Feature:Image\] oc tag should work when only imagestreams api is available`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should have a AlertmanagerReceiversNotConfigured alert in firing state`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should have important platform topology metrics`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should have non-Pod host cAdvisor metrics`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should provide ingress metrics`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should provide named network metrics`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should report telemetry \[Serial\] \[Late\]`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster should start and expose a secured proxy and unsecured metrics`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster shouldn't have failing rules evaluation`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster shouldn't report any alerts in firing state apart from Watchdog and AlertmanagerReceiversNotConfigured \[Early\]`,
			`\[sig-instrumentation\] Prometheus \[apigroup:image.openshift.io\] when installed on the cluster when using openshift-sdn should be able to get the sdn ovs flows`,
			`\[sig-instrumentation\]\[Late\] OpenShift alerting rules \[apigroup:image.openshift.io\] should have a valid severity label`,
			`\[sig-instrumentation\]\[Late\] OpenShift alerting rules \[apigroup:image.openshift.io\] should have description and summary annotations`,
			`\[sig-instrumentation\]\[Late\] OpenShift alerting rules \[apigroup:image.openshift.io\] should have a runbook_url annotation if the alert is critical`,
			`\[sig-instrumentation\]\[Late\] Alerts should have a Watchdog alert in firing state the entire cluster run`,
			`\[sig-instrumentation\]\[Late\] Alerts shouldn't exceed the 500 series limit of total series sent via telemetry from each cluster`,
			`\[sig-instrumentation\]\[Late\] Alerts shouldn't report any alerts in firing or pending state apart from Watchdog and AlertmanagerReceiversNotConfigured and have no gaps in Watchdog firing`,
			`\[sig-instrumentation\]\[sig-builds\]\[Feature:Builds\] Prometheus when installed on the cluster should start and expose a secured proxy and verify build metrics`,
			`\[sig-network-edge\]\[Conformance\]\[Area:Networking\]\[Feature:Router\] The HAProxy router should be able to connect to a service that is idled because a GET on the route will unidle it`,
			`\[sig-network\]\[Feature:Router\] The HAProxy router should enable openshift-monitoring to pull metrics`,
			`\[sig-network\]\[Feature:Router\] The HAProxy router should expose a health check on the metrics port`,
			`\[sig-network\]\[Feature:Router\] The HAProxy router should expose prometheus metrics for a route`,
			`\[sig-network\]\[Feature:Router\] The HAProxy router should expose the profiling endpoints`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:route.openshift.io\] The HAProxy router should override the route host for overridden domains with a custom value`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:route.openshift.io\] The HAProxy router should override the route host with a custom value`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:operator.openshift.io\] The HAProxy router should respond with 503 to unrecognized hosts`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:route.openshift.io\] The HAProxy router should run even if it has no access to update status`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:image.openshift.io\] The HAProxy router should serve a route that points to two services and respect weights`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:operator.openshift.io\] The HAProxy router should serve routes that were created from an ingress`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:route.openshift.io\] The HAProxy router should serve the correct routes when scoped to a single namespace and label set`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:operator.openshift.io\] The HAProxy router should set Forwarded headers appropriately`,
			`\[sig-network\]\[Feature:Router\]\[apigroup:route.openshift.io\]\[apigroup:operator.openshift.io\] The HAProxy router should support reencrypt to services backed by a serving certificate automatically`,
			`\[sig-node\] Managed cluster should report ready nodes the entire duration of the test run`,
			`\[sig-node\]\[Feature:Builds\]\[apigroup:build.openshift.io\] zstd:chunked Image should successfully run date command`,
			`\[sig-storage\]\[Late\] Metrics should report short attach times`,
			`\[sig-storage\]\[Late\] Metrics should report short mount times`,
		},
		"[Skipped:ibmroks]": {
			// skip Gluster tests (not supported on ROKS worker nodes)
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825009 - e2e: skip Glusterfs-related tests upstream for rhel7 worker nodes
			`\[Driver: gluster\]`,
			`GlusterFS`,
			`GlusterDynamicProvisioner`,

			// Currently ibm-master-proxy-static and imbcloud-block-storage-plugin tolerate all taints
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825027
			`\[Feature:Platform\] Managed cluster should ensure control plane operators do not make themselves unevictable`,
		},
		// Tests which can't be run/don't make sense to run against a cluster with all optional capabilities disabled
		"[Skipped:NoOptionalCapabilities]": {
			// This test requires a valid console url which doesn't exist when the optional console capability is disabled.
			`\[sig-cli\] oc basics can show correct whoami result with console`,

			// Image Registry Skips:
			// Requires ImageRegistry to upload appended images
			`\[sig-imageregistry\]\[Feature:ImageAppend\] Image append should create images by appending them`,
			// Requires ImageRegistry to redirect blob pull
			`\[sig-imageregistry\] Image registry \[apigroup:route.openshift.io\] should redirect on blob pull`,
			// Requires ImageRegistry service to be active for OCM to be able to create pull secrets
			`\[sig-devex\]\[Feature:OpenShiftControllerManager\] TestAutomaticCreationOfPullSecrets \[apigroup:config.openshift.io\]`,
			`\[sig-devex\]\[Feature:OpenShiftControllerManager\] TestDockercfgTokenDeletedController \[apigroup:image.openshift.io\]`,

			// These tests run against OLM which does not exist when the optional OLM capability is disabled.
			`\[sig-operator\] OLM should Implement packages API server and list packagemanifest info with namespace not NULL`,
			`\[sig-operator\] OLM should be installed with`,
			`\[sig-operator\] OLM should have imagePullPolicy:IfNotPresent on thier deployments`,
			`\[sig-operator\] an end user can use OLM`,
			`\[sig-arch\] ocp payload should be based on existing source OLM`,
		},
	}
)

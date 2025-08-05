package extensions

import (
	"fmt"

	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

// addEnvironmentSelectors adds the environmentSelector field to appropriate specs to facilitate including or excluding
// them based on attributes of the cluster they are running on
func addEnvironmentSelectors(specs et.ExtensionTestSpecs) {
	filterByPlatform(specs)
	filterByExternalConnectivity(specs)
	filterByTopology(specs)
	filterByNoOptionalCapabilities(specs)
	filterByNetwork(specs)
	filterByNetworkStack(specs)
}

// filterByPlatform is a helper function to do, simple, "NameContains" filtering on tests by platform
func filterByPlatform(specs et.ExtensionTestSpecs) {
	var platformExclusions = map[string][]string{
		"ibmroks": {
			// skip Gluster tests (not supported on ROKS worker nodes)
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825009 - e2e: skip Glusterfs-related tests upstream for rhel7 worker nodes
			"[Driver: gluster]",
			"GlusterFS",
			"GlusterDynamicProvisioner",

			// Currently ibm-master-proxy-static and imbcloud-block-storage-plugin tolerate all taints
			// https://bugzilla.redhat.com/show_bug.cgi?id=1825027
			"[Feature:Platform] Managed cluster should ensure control plane operators do not make themselves unevictable",
		},
	}

	for p, exclusions := range platformExclusions {
		var selectFunctions []et.SelectFunction
		for _, exclusion := range exclusions {
			selectFunctions = append(selectFunctions, et.NameContains(exclusion))
		}

		specs.SelectAny(selectFunctions).
			Exclude(et.PlatformEquals(p)).
			AddLabel(fmt.Sprintf("[Skipped:%s]", p))
	}
}

// filterByExternalConnectivity is a helper function to do, simple, "NameContains" filtering on tests by external connectivity
func filterByExternalConnectivity(specs et.ExtensionTestSpecs) {
	var externalConnectivityExclusions = map[string][]string{
		// Tests that don't pass on disconnected, either due to requiring
		// internet access for GitHub (e.g. many of the s2i builds), or
		// because of pullthrough not supporting ICSP (https://bugzilla.redhat.com/show_bug.cgi?id=1918376)
		"Disconnected": {
			// Internet access required
			"[sig-builds][Feature:Builds] clone repository using git:// protocol should clone using git:// if no proxy is configured",
			"[sig-builds][Feature:Builds] result image should have proper labels set S2I build from a template should create a image from \"test-s2i-build.json\" template with proper Docker labels",
			"[sig-builds][Feature:Builds] s2i build with a quota Building from a template should create an s2i build with a quota and run it",
			"[sig-builds][Feature:Builds] s2i build with a root user image should create a root build and pass with a privileged SCC",
			"[sig-builds][Feature:Builds][timing] capture build stages and durations should record build stages and durations for docker",
			"[sig-builds][Feature:Builds][timing] capture build stages and durations should record build stages and durations for s2i",
			"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should successfully resolve valueFrom in s2i build environment variables",
			"[sig-builds][Feature:Builds][volumes] should mount given secrets and configmaps into the build pod for source strategy builds",
			"[sig-builds][Feature:Builds][volumes] should mount given secrets and configmaps into the build pod for docker strategy builds",
			"[sig-builds][Feature:Builds][pullsearch] docker build where the registry is not specified Building from a Dockerfile whose FROM image ref does not specify the image registry should create a docker build that has buildah search from our predefined list of image registries and succeed",
			"[sig-cli] oc debug ensure it works with image streams",
			"[sig-cli] oc builds complex build start-build",
			"[sig-cli] oc builds complex build webhooks CRUD",
			"[sig-cli] oc builds new-build",
			"[sig-devex] check registry.redhat.io is available and samples operator can import sample imagestreams run sample related validations",
			"[sig-devex][Feature:Templates] templateinstance readiness test should report failed soon after an annotated objects has failed",
			"[sig-devex][Feature:Templates] templateinstance readiness test should report ready soon after all annotated objects are ready",
			"[sig-operator] an end user can use OLM can subscribe to the operator",
			"[sig-imageregistry][Serial] Image signature workflow can push a signed image to openshift registry and verify it",

			// Need to access non-cached images like ruby and mongodb
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with multiple image change triggers should run a successful deployment with a trigger used by different containers",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with multiple image change triggers should run a successful deployment with multiple triggers",
			"[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the AlwaysAllow policy",
			"[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the IfHealthyBudget policy",

			// ICSP
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs should adhere to Three Laws of Controllers",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs adoption will orphan all RCs and adopt them back when recreated",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs generation should deploy based on a status version bump",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs keep the deployer pod invariant valid should deal with cancellation after deployer pod succeeded",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs paused should disable actions on deployments",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs rolled back should rollback to an older deployment",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs should respect image stream tag reference policy resolve the image pull spec",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs viewing rollout history should print the rollout history",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when changing image change trigger should successfully trigger from an updated image",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when run iteratively should only deploy the last deployment",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when tagging images should successfully tag the deployed image",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with custom deployments should run the custom deployment steps",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with enhanced status should include various info in status",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with env in params referencing the configmap should expand the config map key to a value",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with failing hook should get all logs from retried hooks",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with minimum ready seconds set should not transition the deployment to Complete before satisfied",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with revision history limits should never persist more old deployments than acceptable after being observed by the controller",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with test deployments should run a deployment to completion and then scale to zero",
			"[sig-apps][Feature:DeploymentConfig] deploymentconfigs won't deploy RC with unresolved images when patched with empty image",
			"[sig-arch] Managed cluster should expose cluster services outside the cluster",
			"[sig-arch][Early] Managed cluster should [apigroup:config.openshift.io] start all core operators",
			"[sig-auth][Feature:SecurityContextConstraints] TestPodDefaultCapabilities",
			"[sig-builds][Feature:Builds] Multi-stage image builds should succeed",
			"[sig-builds][Feature:Builds] Optimized image builds should succeed",
			"[sig-builds][Feature:Builds] build can reference a cluster service with a build being created from new-build should be able to run a build that references a cluster service",
			"[sig-builds][Feature:Builds] build have source revision metadata started build should contain source revision information",
			"[sig-builds][Feature:Builds] build with empty source started build should build even with an empty source in build config",
			"[sig-builds][Feature:Builds] build without output image building from templates should create an image from a S2i template without an output image reference defined",
			"[sig-builds][Feature:Builds] build without output image building from templates should create an image from a docker template without an output image reference defined",
			"[sig-builds][Feature:Builds] custom build with buildah being created from new-build should complete build with custom builder image",
			"[sig-builds][Feature:Builds] imagechangetriggers imagechangetriggers should trigger builds of all types",
			"[sig-builds][Feature:Builds] oc new-app should fail with a --name longer than 58 characters",
			"[sig-builds][Feature:Builds] oc new-app should succeed with a --name of 58 characters",
			"[sig-builds][Feature:Builds] oc new-app should succeed with an imagestream",
			"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig buildconfigs should have a default history limit set when created via the group api",
			"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune builds after a buildConfig change",
			"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune canceled builds based on the failedBuildsHistoryLimit setting",
			"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune completed builds based on the successfulBuildsHistoryLimit setting",
			"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune errored builds based on the failedBuildsHistoryLimit setting",
			"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune failed builds based on the failedBuildsHistoryLimit setting",
			"[sig-builds][Feature:Builds] result image should have proper labels set Docker build from a template should create a image from \"test-docker-build.json\" template with proper Docker labels",
			"[sig-builds][Feature:Builds] verify /run filesystem contents are writeable using a simple Docker Strategy Build",
			"[sig-builds][Feature:Builds] verify /run filesystem contents do not have unexpected content using a simple Docker Strategy Build",
			"[sig-builds][Feature:Builds][pullsecret] docker build using a pull secret Building from a template should create a docker build that pulls using a secret run it",
			"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should fail resolving unresolvable valueFrom in docker build environment variable references",
			"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should fail resolving unresolvable valueFrom in sti build environment variable references",
			"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should successfully resolve valueFrom in docker build environment variables",
			"[sig-builds][Feature:Builds][pullsearch] docker build where the registry is not specified Building from a Dockerfile whose FROM image ref does not specify the image registry should create a docker build that has buildah search from our predefined list of image registries and succeed",
			"[sig-cli] CLI can run inside of a busybox container",
			"[sig-cli] oc debug deployment configs from a build",
			"[sig-cli] oc debug deployment from a build",
			"[sig-cli] oc rsh specific flags should work well when access to a remote shell",
			"[sig-cli] oc builds get buildconfig",
			"[sig-cli] oc builds patch buildconfig",
			"[sig-cluster-lifecycle] Pods cannot access the /config/master API endpoint",
			"[sig-imageregistry][Feature:ImageAppend] Image append should create images by appending them",
			"[sig-imageregistry][Feature:ImageExtract] Image extract should extract content from an image",
			"[sig-imageregistry][Feature:ImageInfo] Image info should display information about images",
			"[sig-imageregistry][Feature:ImageLayers] Image layer subresource should return layers from tagged images",
			"[sig-imageregistry][Feature:ImageTriggers] Annotation trigger reconciles after the image is overwritten",
			"[sig-imageregistry][Feature:Image] oc tag should change image reference for internal images",
			"[sig-imageregistry][Feature:Image] oc tag should work when only imagestreams api is available",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should have a AlertmanagerReceiversNotConfigured alert in firing state",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should have important platform topology metrics",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should have non-Pod host cAdvisor metrics",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should provide ingress metrics",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should provide named network metrics",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should report telemetry [Serial] [Late]",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should start and expose a secured proxy and unsecured metrics",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster shouldn't have failing rules evaluation",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster shouldn't report any alerts in firing state apart from Watchdog and AlertmanagerReceiversNotConfigured [Early]",
			"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster when using openshift-sdn should be able to get the sdn ovs flows",
			"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should have a valid severity label",
			"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should have description and summary annotations",
			"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should have a runbook_url annotation if the alert is critical",
			"[sig-instrumentation][Late] Alerts should have a Watchdog alert in firing state the entire cluster run",
			"[sig-instrumentation][Late] Alerts shouldn't exceed the 500 series limit of total series sent via telemetry from each cluster",
			"[sig-instrumentation][Late] Alerts shouldn't report any alerts in firing or pending state apart from Watchdog and AlertmanagerReceiversNotConfigured and have no gaps in Watchdog firing",
			"[sig-instrumentation][sig-builds][Feature:Builds] Prometheus when installed on the cluster should start and expose a secured proxy and verify build metrics",
			"[sig-network-edge][Conformance][Area:Networking][Feature:Router] The HAProxy router should be able to connect to a service that is idled because a GET on the route will unidle it",
			"[sig-network][Feature:Router] The HAProxy router should enable openshift-monitoring to pull metrics",
			"[sig-network][Feature:Router] The HAProxy router should expose a health check on the metrics port",
			"[sig-network][Feature:Router] The HAProxy router should expose prometheus metrics for a route",
			"[sig-network][Feature:Router] The HAProxy router should expose the profiling endpoints",
			"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should override the route host for overridden domains with a custom value",
			"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should override the route host with a custom value",
			"[sig-network][Feature:Router][apigroup:operator.openshift.io] The HAProxy router should respond with 503 to unrecognized hosts",
			"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should run even if it has no access to update status",
			"[sig-network][Feature:Router][apigroup:image.openshift.io] The HAProxy router should serve a route that points to two services and respect weights",
			"[sig-network][Feature:Router][apigroup:operator.openshift.io] The HAProxy router should serve routes that were created from an ingress",
			"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should serve the correct routes when scoped to a single namespace and label set",
			"[sig-network][Feature:Router][apigroup:operator.openshift.io] The HAProxy router should set Forwarded headers appropriately",
			"[sig-network][Feature:Router][apigroup:route.openshift.io][apigroup:operator.openshift.io] The HAProxy router should support reencrypt to services backed by a serving certificate automatically",
			"[sig-node] Managed cluster should report ready nodes the entire duration of the test run",
			"[sig-node][Feature:Builds][apigroup:build.openshift.io] zstd:chunked Image should successfully run date command",
			"[sig-storage][Late] Metrics should report short attach times",
			"[sig-storage][Late] Metrics should report short mount times",
		},
		// These tests are skipped when openshift-tests needs to use a proxy to reach the
		// cluster -- either because the test won't work while proxied, or because the test
		// itself is testing a functionality using it's own proxy.
		"Proxied": {
			// These tests are flacky and require internet access
			// See https://bugzilla.redhat.com/show_bug.cgi?id=2019375
			"[sig-builds][Feature:Builds] build can reference a cluster service with a build being created from new-build should be able to run a build that references a cluster service",
			"[sig-builds][Feature:Builds] oc new-app should succeed with a --name of 58 characters",
			"[sig-arch] Only known images used by tests",
		},
	}

	for connectivity, exclusions := range externalConnectivityExclusions {
		var selectFunctions []et.SelectFunction
		for _, exclusion := range exclusions {
			selectFunctions = append(selectFunctions, et.NameContains(exclusion))
		}

		specs.SelectAny(selectFunctions).
			Exclude(et.ExternalConnectivityEquals(connectivity)).
			AddLabel(fmt.Sprintf("[Skipped:%s]", connectivity))
	}
}

// filterByTopology is a helper function to do, simple, "NameContains" filtering on tests by topology
func filterByTopology(specs et.ExtensionTestSpecs) {
	var topologyExclusions = map[string][]string{
		"SingleReplica": {
			"should be scheduled on different nodes",
		},
	}

	for t, exclusions := range topologyExclusions {
		var selectFunctions []et.SelectFunction
		for _, exclusion := range exclusions {
			selectFunctions = append(selectFunctions, et.NameContains(exclusion))
		}

		specs.SelectAny(selectFunctions).
			Exclude(et.TopologyEquals(t)).
			AddLabel(fmt.Sprintf("[Skipped:%s]", t))
	}
}

// filterByNoOptionalCapabilities is a helper function to facilitate adding environment selectors for tests which can't
// be run/don't make sense to run against a cluster with all optional capabilities disabled
func filterByNoOptionalCapabilities(specs et.ExtensionTestSpecs) {
	var exclusions = []string{
		// This test requires a valid console url which doesn't exist when the optional console capability is disabled.
		"[sig-cli] oc basics can show correct whoami result with console",

		// Image Registry Skips:
		// Requires ImageRegistry to upload appended images
		"[sig-imageregistry][Feature:ImageAppend] Image append should create images by appending them",
		// Requires ImageRegistry to redirect blob pull
		"[sig-imageregistry] Image registry [apigroup:route.openshift.io] should redirect on blob pull",
		// Requires ImageRegistry service to be active for OCM to be able to create pull secrets
		"[sig-devex][Feature:OpenShiftControllerManager] TestAutomaticCreationOfPullSecrets [apigroup:config.openshift.io]",
		"[sig-devex][Feature:OpenShiftControllerManager] TestDockercfgTokenDeletedController [apigroup:image.openshift.io]",

		// These tests run against OLM which does not exist when the optional OLM capability is disabled.
		"[sig-operator] OLM should Implement packages API server and list packagemanifest info with namespace not NULL",
		"[sig-operator] OLM should be installed with",
		"[sig-operator] OLM should have imagePullPolicy:IfNotPresent on thier deployments",
		"[sig-operator] an end user can use OLM",
		"[sig-arch] ocp payload should be based on existing source OLM",
	}

	var selectFunctions []et.SelectFunction
	for _, exclusion := range exclusions {
		selectFunctions = append(selectFunctions, et.NameContains(exclusion))
	}
	specs.SelectAny(selectFunctions).
		Exclude(et.NoOptionalCapabilitiesExist()).
		AddLabel("[Skipped:NoOptionalCapabilities]")
}

// filterByNetwork is a helper function to do, simple, "NameContains" filtering on tests by network
func filterByNetwork(specs et.ExtensionTestSpecs) {
	var networkExclusions = map[string][]string{}

	for n, exclusions := range networkExclusions {
		var selectFunctions []et.SelectFunction
		for _, exclusion := range exclusions {
			selectFunctions = append(selectFunctions, et.NameContains(exclusion))
		}

		specs.SelectAny(selectFunctions).
			Exclude(et.NetworkEquals(n)).
			AddLabel(fmt.Sprintf("[Skipped:%s]", n))
	}
}

// filterByNetworkStack is a helper function to do, simple, "NameContains" filtering on tests by networkStack
func filterByNetworkStack(specs et.ExtensionTestSpecs) {
	var networkStackExclusions = map[string][]string{
		"ipv4": {
			"[sig-network][Feature:Router][apigroup:route.openshift.io] when FIPS is disabled the HAProxy router should serve routes when configured with a 1024-bit RSA key",
		},
	}

	for stack, exclusions := range networkStackExclusions {
		var selectFunctions []et.SelectFunction
		for _, exclusion := range exclusions {
			selectFunctions = append(selectFunctions, et.NameContains(exclusion))
		}

		specs.SelectAny(selectFunctions).
			Exclude(et.NetworkStackEquals(stack)).
			AddLabel(fmt.Sprintf("[Skipped:%s]", stack))
	}
}

package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	kclientset "k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/test/extended/cluster/metrics"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

const checkDeleteProjectInterval = 10 * time.Second
const checkDeleteProjectTimeout = 3 * time.Minute
const checkPodRunningTimeout = 5 * time.Minute

// TODO sjug: pass label via config
var podLabelMap = map[string]string{"purpose": "test"}
var rootDir string

var _ = g.Describe("[sig-scalability][Feature:Performance] Load cluster", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLIWithoutNamespace("cl")
		masterVertFixture = exutil.FixturePath("testdata", "cluster", "master-vert.yaml")
		_                 = exutil.FixturePath("testdata", "cluster", "quickstarts", "cakephp-mysql.json")
		_                 = exutil.FixturePath("testdata", "cluster", "quickstarts", "dancer-mysql.json")
		_                 = exutil.FixturePath("testdata", "cluster", "quickstarts", "django-postgresql.json")
		_                 = exutil.FixturePath("testdata", "cluster", "quickstarts", "nodejs-postgresql.json")
		_                 = exutil.FixturePath("testdata", "cluster", "quickstarts", "rails-postgresql.json")
	)

	var c kclientset.Interface
	g.BeforeEach(func() {
		var err error
		c = oc.AdminKubeClient()
		e2e.Logf("Undefined config file, using built-in config %v\n", masterVertFixture)
		path := strings.Split(masterVertFixture, "/")
		rootDir = strings.Join(path[:len(path)-5], "/")
		err = ParseConfig(masterVertFixture, true)
		if err != nil {
			e2e.Failf("Error parsing config: %v\n", err)
		}
	})

	g.It("should populate the cluster [Slow][Serial][apigroup:template.openshift.io][apigroup:apps.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		project := ConfigContext.ClusterLoader.Projects
		tuningSets := ConfigContext.ClusterLoader.TuningSets
		sync := ConfigContext.ClusterLoader.Sync
		if project == nil {
			e2e.Failf("Invalid config file.\nFile: %v", project)
		}

		var namespaces []string
		var steps []metrics.StepDuration
		//totalPods := 0 // Keep track of how many pods for stepping
		// TODO sjug: add concurrency
		testStartTime := time.Now()
		for _, p := range project {
			// Find tuning if we have it
			tuning := GetTuningSet(tuningSets, p.Tuning)
			if tuning != nil {
				e2e.Logf("Our tuning set is: %v", tuning)
			}
			for j := 0; j < p.Number; j++ {
				var allArgs []string
				if p.NodeSelector != "" {
					allArgs = append(allArgs, "--node-selector")
					allArgs = append(allArgs, p.NodeSelector)
				}
				nsName := fmt.Sprintf("%s%d", p.Basename, j)
				allArgs = append(allArgs, nsName)

				projectExists, err := ProjectExists(oc, nsName)
				o.Expect(err).NotTo(o.HaveOccurred())
				if !projectExists {
					e2e.Logf("Project %s does not exist.", nsName)
				}

				switch p.IfExists {
				case IF_EXISTS_REUSE:
					e2e.Logf("Configuration requested reuse of project %v", nsName)
				case IF_EXISTS_DELETE:
					e2e.Logf("Configuration requested deletion of project %v", nsName)
					if projectExists {
						err = DeleteProject(oc, nsName, checkDeleteProjectInterval, checkDeleteProjectTimeout)
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				default:
					e2e.Failf("Unsupported ifexists value '%v' for project %v", p.IfExists, project)
				}

				if p.IfExists == IF_EXISTS_REUSE && projectExists {
					// do nothing
				} else {
					// Create namespaces as defined in Cluster Loader config
					err = oc.Run("adm", "new-project").Args(allArgs...).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("%d/%d : Created new namespace: %v", j+1, p.Number, nsName)
				}

				// label namespace nsName
				if p.Labels != nil {
					_, err = SetNamespaceLabels(c, nsName, p.Labels)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				namespaces = append(namespaces, nsName)

				// Create config maps
				if p.Configmaps != nil {
					// Configmaps defined, create them
					err := CreateConfigmaps(oc, c, nsName, p.Configmaps)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				// Create secrets
				if p.Secrets != nil {
					// Secrets defined, create them
					err := CreateSecrets(oc, c, nsName, p.Secrets)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				// Create templates as defined
				for _, template := range p.Templates {
					var rateDelay, stepPause time.Duration
					if tuning != nil {
						if tuning.Templates.RateLimit.Delay != 0 {
							rateDelay = tuning.Templates.RateLimit.Delay
						}
						if tuning.Templates.Stepping.Pause != 0 {
							stepPause = tuning.Templates.Stepping.Pause
						}
					}
					step := metrics.NewTemplateStepDuration(rateDelay, stepPause)
					err := CreateTemplates(oc, c, nsName, template, tuning, &step)
					o.Expect(err).NotTo(o.HaveOccurred())
					steps = append(steps, step)
				}

				// This is too familiar, create pods
				for _, pod := range p.Pods {
					var path string
					var err error
					if pod.File != "" {
						// Parse Pod file into struct
						path, err = mkPath(pod.File)
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					config, err := ParsePods(path)
					o.Expect(err).NotTo(o.HaveOccurred())

					// Check if environment variables are defined in CL config
					if pod.Parameters == nil {
						e2e.Logf("Pod environment variables will not be modified.")
					} else {
						// Override environment variables for Pod using ConfigMap
						configMapName := InjectConfigMap(c, nsName, pod.Parameters, config)
						// Cleanup ConfigMap at some point after the Pods are created
						defer func() {
							_ = c.CoreV1().ConfigMaps(nsName).Delete(context.Background(), configMapName, metav1.DeleteOptions{})
						}()
					}
					// TODO sjug: pass label via config
					podLabelMap := map[string]string{"purpose": "test"}
					var rateDelay, stepPause time.Duration
					if tuning != nil {
						if tuning.Pods.RateLimit.Delay != 0 {
							rateDelay = tuning.Pods.RateLimit.Delay
						}
						if tuning.Pods.Stepping.Pause != 0 {
							stepPause = tuning.Pods.Stepping.Pause
						}
					}
					step := metrics.NewPodStepDuration(rateDelay, stepPause)
					err = pod.CreatePods(c, nsName, podLabelMap, config.Spec, tuning, &step)
					steps = append(steps, step)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			}
		}

		if sync.Running {
			timeout, err := time.ParseDuration(sync.Timeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, ns := range namespaces {
				err := SyncRunningPods(c, ns, sync.Selectors, timeout)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		if sync.Server.Enabled {
			var podCount PodCount
			err := Server(&podCount, sync.Server.Port, false)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		if sync.Succeeded {
			timeout, err := time.ParseDuration(sync.Timeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, ns := range namespaces {
				err := SyncSucceededPods(c, ns, sync.Selectors, timeout)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		if err := postCreateWait(oc, namespaces); err != nil {
			e2e.Failf("Error in postCreateWait: %v", err)
		}

		// Calculate and log test duration
		m := []metrics.Metrics{metrics.NewTestDuration("cluster-loader-test", testStartTime, time.Since(testStartTime), steps)}
		err := metrics.LogMetrics(m)
		o.Expect(err).NotTo(o.HaveOccurred())

		// If config context set to cleanup on completion
		if ConfigContext.ClusterLoader.Cleanup == true {
			for _, ns := range namespaces {
				e2e.Logf("Deleting project %s", ns)
				err := oc.AsAdmin().KubeClient().CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})
})

// postCreateWait looks for RCs, pods, builds, and DCs to ensure they're in a good state in each namespace
func postCreateWait(oc *util.CLI, namespaces []string) error {
	// Wait for builds and deployments to complete
	for _, ns := range namespaces {
		rcList, err := oc.AdminKubeClient().CoreV1().ReplicationControllers(ns).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("Error listing RCs: %v", err)
		}
		rcCount := len(rcList.Items)
		if rcCount > 0 {
			e2e.Logf("Waiting for %d RCs in namespace %s", rcCount, ns)
			for _, rc := range rcList.Items {
				e2e.Logf("Waiting for RC: %s", rc.Name)
				err := waitForRCToStabilize(oc.AdminKubeClient(), ns, rc.Name, checkPodRunningTimeout)
				if err != nil {
					return fmt.Errorf("Error in waiting for RC to stabilize: %v", err)
				}
				err = WaitForRCReady(oc, ns, rc.Name, checkPodRunningTimeout)
				if err != nil {
					return fmt.Errorf("Error in waiting for RC to become ready: %v", err)
				}
			}
		}

		podLabels := exutil.ParseLabelsOrDie(mapToString(podLabelMap))
		podList, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{LabelSelector: podLabels.String()})
		if err != nil {
			return fmt.Errorf("Error in listing pods: %v", err)
		}
		podCount := len(podList.Items)
		if podCount > 0 {
			e2e.Logf("Waiting for %d pods in namespace %s", podCount, ns)
			c := oc.AdminKubeClient()
			pods, err := exutil.WaitForPods(c.CoreV1().Pods(ns), podLabels, exutil.CheckPodIsRunning, podCount, checkPodRunningTimeout)
			if err != nil {
				return fmt.Errorf("Error in pod wait: %v", err)
			} else if len(pods) < podCount {
				return fmt.Errorf("Only got %v out of %v pods in %s (timeout)", len(pods), podCount, checkPodRunningTimeout)
			}
			e2e.Logf("All pods in namespace %s running", ns)
		}

		buildList, err := oc.AsAdmin().BuildClient().BuildV1().Builds(ns).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("Error in listing builds: %v", err)
		}
		e2e.Logf("Build List: %+v", buildList)
		if len(buildList.Items) > 0 {
			// Get first build name
			buildName := buildList.Items[0].Name
			e2e.Logf("Waiting for build: %q", buildName)
			err = exutil.WaitForABuild(oc.AsAdmin().BuildClient().BuildV1().Builds(ns), buildName, nil, nil, nil)
			if err != nil {
				exutil.DumpBuildLogs(buildName, oc)
				return fmt.Errorf("Error in waiting for build: %v", err)
			}
			e2e.Logf("Build %q completed", buildName)

		}

		dcList, err := oc.AsAdmin().AppsClient().AppsV1().DeploymentConfigs(ns).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("Error listing DeploymentConfigs: %v", err)
		}
		if len(dcList.Items) > 0 {
			// Get first deployment config name
			deploymentName := dcList.Items[0].Name
			e2e.Logf("Waiting for deployment: %q", deploymentName)
			err = exutil.WaitForDeploymentConfig(oc.AdminKubeClient(), oc.AsAdmin().AppsClient().AppsV1(), ns, deploymentName, 1, true, oc)
			if err != nil {
				return fmt.Errorf("Error in waiting for DeploymentConfigs: %v", err)
			}
			e2e.Logf("Deployment %q completed", deploymentName)
		}
	}
	return nil
}

// mkPath returns fully qualfied file path as a string
func mkPath(filename string) (string, error) {
	// Use absolute path if provided in config
	if filepath.IsAbs(filename) {
		return filename, nil
	}
	// Handle an empty filename.
	if filename == "" {
		return "", fmt.Errorf("no template file defined!")
	}

	var searchPaths []string
	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	searchPaths = append(searchPaths, filepath.Join(workingDir, filename))

	for _, v := range searchPaths {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
	}

	return "", fmt.Errorf("unable to find pod/template file %s\n", filename)
}

// waitForRCToStabilize waits till the RC has a matching generation/replica count between spec and status.
func waitForRCToStabilize(c clientset.Interface, ns, name string, timeout time.Duration) error {
	options := metav1.ListOptions{FieldSelector: fields.Set{
		"metadata.name":      name,
		"metadata.namespace": ns,
	}.AsSelector().String()}
	w, err := c.CoreV1().ReplicationControllers(ns).Watch(context.Background(), options)
	if err != nil {
		return err
	}
	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()
	_, err = watchtools.UntilWithoutRetry(ctx, w, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, apierrs.NewNotFound(schema.GroupResource{Resource: "replicationcontrollers"}, "")
		}
		switch rc := event.Object.(type) {
		case *v1.ReplicationController:
			if rc.Name == name && rc.Namespace == ns &&
				rc.Generation <= rc.Status.ObservedGeneration &&
				*(rc.Spec.Replicas) == rc.Status.Replicas {
				return true, nil
			}
			e2e.Logf("Waiting for rc %s to stabilize, generation %v observed generation %v spec.replicas %d status.replicas %d",
				name, rc.Generation, rc.Status.ObservedGeneration, *(rc.Spec.Replicas), rc.Status.Replicas)
		}
		return false, nil
	})
	return err
}

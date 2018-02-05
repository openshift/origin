package cluster

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	metrics "github.com/openshift/origin/test/extended/cluster/metrics"
	exutil "github.com/openshift/origin/test/extended/util"
)

const checkDeleteProjectInterval = 10 * time.Second
const checkDeleteProjectTimeout = 3 * time.Minute
const deploymentRunTimeout = 5 * time.Minute
const testResultFile = "/tmp/TestResult"

var (
	rootDir string
)

var _ = g.Describe("[Feature:Performance][Serial][Slow] Load cluster", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLI("cl", exutil.KubeConfigPath())
		masterVertFixture = exutil.FixturePath("testdata", "cluster", "master-vert.yaml")
		_                 = exutil.FixturePath("..", "..", "examples", "quickstarts", "cakephp-mysql.json")
		_                 = exutil.FixturePath("..", "..", "examples", "quickstarts", "dancer-mysql.json")
		_                 = exutil.FixturePath("..", "..", "examples", "quickstarts", "django-postgresql.json")
		_                 = exutil.FixturePath("..", "..", "examples", "quickstarts", "nodejs-mongodb.json")
		_                 = exutil.FixturePath("..", "..", "examples", "quickstarts", "rails-postgresql.json")
	)

	var c kclientset.Interface
	g.BeforeEach(func() {
		var err error
		c = oc.AdminKubeClient()
		viperConfig := e2e.TestContext.Viper
		if viperConfig == "e2e" {
			e2e.Logf("Undefined config file, using built-in config %v\n", masterVertFixture)
			path := strings.Split(masterVertFixture, "/")
			rootDir = strings.Join(path[:len(path)-5], "/")
			err = ParseConfig(masterVertFixture, true)
		} else {
			e2e.Logf("Using config %v\n", viperConfig)
			err = ParseConfig(viperConfig, false)
		}
		if err != nil {
			e2e.Failf("Error parsing config: %v\n", err)
		}
	})

	g.It("should load the cluster", func() {
		projects := ConfigContext.ClusterLoader.Projects
		if projects == nil {
			e2e.Failf("Invalid config file.\nFile: %v", projects)
		}
		syncConfig := ConfigContext.ClusterLoader.Sync
		testStartTime := time.Now()

		namespaces, err := process(oc, c, projects)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
		} else {
			fmt.Printf("Created the following namespaces: %+v\n", namespaces)
		}

		if syncConfig.Running {
			timeout, err := time.ParseDuration(syncConfig.Timeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, ns := range namespaces {
				err := SyncRunningPods(c, ns, syncConfig.Selectors, timeout)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		if syncConfig.Server.Enabled {
			var podCount PodCount
			err := Server(&podCount, syncConfig.Server.Port, false)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		if syncConfig.Succeeded {
			timeout, err := time.ParseDuration(syncConfig.Timeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, ns := range namespaces {
				err := SyncSucceededPods(c, ns, syncConfig.Selectors, timeout)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		// Wait for builds and deployments to complete
		for _, ns := range namespaces {
			buildList, err := oc.InternalBuildClient().Build().Builds(ns).List(metav1.ListOptions{})
			if err != nil {
				e2e.Logf("Error listing builds: %v", err)
			}
			if len(buildList.Items) > 0 {
				buildName := buildList.Items[0].Name
				e2e.Logf("Waiting for build: %q", buildName)
				err = exutil.WaitForABuild(oc.InternalBuildClient().Build().Builds(ns), buildName, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(buildName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Build %q completed", buildName)

				// deploymentName is buildName without the -1 suffix
				deploymentName := buildName[:len(buildName)-2]
				e2e.Logf("Waiting for deployment: %q", deploymentName)
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), ns, deploymentName, 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Deployment %q completed", deploymentName)
			}
		}

		// Calculate and log test duration
		m := []metrics.Metrics{metrics.NewTestDuration("cluster-loader-test", testStartTime, time.Since(testStartTime))}
		err = metrics.LogMetrics(m)
		o.Expect(err).NotTo(o.HaveOccurred())

		// If config context set to cleanup on completion
		if ConfigContext.ClusterLoader.Cleanup == true {
			for _, ns := range namespaces {
				e2e.Logf("Deleting project %s", ns)
				err := oc.AsAdmin().KubeClient().CoreV1().Namespaces().Delete(ns, nil)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})
})

func process(oc *exutil.CLI, c kclientset.Interface, projects []ClusterLoaderType) ([]string, error) {
	var err error
	var wg sync.WaitGroup
	done := make(chan struct{})
	errChan := make(chan error, 1)
	projectSem := make(chan struct{}, 2)
	resultChan := make(chan string)
	namespaces := []string{}

	wg.Add(len(projects))
	for _, project := range projects {
		go processProject(oc, c, project, projectSem, &wg, resultChan, errChan, done)
	}
	go func() {
		wg.Wait()
		close(resultChan)
	}()
loop:
	for {
		select {
		case err = <-errChan:
			close(done)
		case <-done:
			for range resultChan {
				// Do nothing.
			}
			return nil, err
		case result, ok := <-resultChan:
			if !ok {
				break loop
			}
			if result != "" {
				namespaces = append(namespaces, result)
			}
		}
	}
	return namespaces, nil
}

func processProject(oc *exutil.CLI, c kclientset.Interface, p ClusterLoaderType, projectSem chan struct{}, wg *sync.WaitGroup, resultChan chan string, errChan chan error, done chan struct{}) {
	defer wg.Done()
	select {
	case projectSem <- struct{}{}:
	case <-done:
		return // cancelled
	}
	objectSem := make(chan struct{}, 10)

	// Find tuning if we have it
	tuning := GetTuningSet(ConfigContext.ClusterLoader.TuningSets, p.Tuning)
	if tuning != nil {
		e2e.Logf("Our tuning set is: %v", tuning)
	}
	for j := 0; j < p.Number; j++ {
		var allArgs []string
		allArgs = append(allArgs, "--skip-config-write")
		nsName := fmt.Sprintf("%s%d", p.Basename, j)

		projectExists, err := ProjectExists(oc, nsName)

		resultChan <- nsName
		allArgs = append(allArgs, nsName)

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
			e2e.Failf("Unsupported ifexists value '%v' for project %v", p.IfExists, p)
		}

		if p.IfExists == IF_EXISTS_REUSE && projectExists {
			// do nothing
		} else {
			// Create namespaces as defined in Cluster Loader config
			if err := oc.Run("new-project").Args(allArgs...).Execute(); err != nil {
				errChan <- err
			}
			e2e.Logf("%d/%d : Created new namespace: %v", j+1, p.Number, nsName)
		}

		// Create config maps
		if p.Configmaps != nil {
			wg.Add(1)
			// Configmaps defined, create them
			go func(nsName string) {
				defer wg.Done()
				objectSem <- struct{}{}
				if err := CreateConfigmaps(oc, c, nsName, p.Configmaps); err != nil {
					errChan <- err
				}
				<-objectSem
			}(nsName)
		}

		// Create secrets
		if p.Secrets != nil {
			wg.Add(1)
			// Secrets defined, create them
			go func(nsName string) {
				defer wg.Done()
				objectSem <- struct{}{}
				if err := CreateSecrets(oc, c, nsName, p.Secrets); err != nil {
					errChan <- err
				}
				<-objectSem
			}(nsName)
		}

		// Create templates as defined
		for _, template := range p.Templates {
			wg.Add(1)
			go func(nsName string, template ClusterLoaderObjectType, tuning *TuningSetType) {
				defer wg.Done()
				objectSem <- struct{}{}
				if err := CreateTemplates(oc, c, nsName, template, tuning); err != nil {
					errChan <- err
				}
				<-objectSem
			}(nsName, template, tuning)
		}

		// This is too familiar, create pods
		for _, pod := range p.Pods {
			wg.Add(1)
			go func(nsName string, pod ClusterLoaderObjectType) {
				defer wg.Done()
				objectSem <- struct{}{}
				// Parse Pod file into struct
				config := ParsePods(mkPath(pod.File))
				// Check if environment variables are defined in CL config
				if pod.Parameters == nil {
					e2e.Logf("Pod environment variables will not be modified.")
				} else {
					// Override environment variables for Pod using ConfigMap
					configMapName := InjectConfigMap(c, nsName, pod.Parameters, config)
					// Cleanup ConfigMap at some point after the Pods are created
					defer func() {
						_ = c.Core().ConfigMaps(nsName).Delete(configMapName, nil)
					}()
				}
				// TODO sjug: pass label via config
				labels := map[string]string{"purpose": "test"}
				if err := CreatePods(c, pod.Basename, nsName, labels, config.Spec, pod.Number, tuning, &pod.Sync); err != nil {
					errChan <- err
				}
				<-objectSem
			}(nsName, pod)
		}
	}
	<-projectSem
}

func newProject(nsName string) *projectapi.Project {
	return &projectapi.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				oapi.OpenShiftDisplayName: nsName,
				//"openshift.io/node-selector": "purpose=test",
			},
		},
	}
}

// mkPath returns fully qualfied file location as a string
func mkPath(file string) string {
	// Handle an empty filename.
	if file == "" {
		e2e.Failf("No template file defined!")
	}
	if rootDir == "" {
		rootDir = "content"
	}
	return filepath.Join(rootDir+"/", file)
}

// appendIntToString appends an integer i to string s
func appendIntToString(s string, i int) string {
	return s + strconv.Itoa(i)
}

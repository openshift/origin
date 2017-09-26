package cluster

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
)

const deploymentRunTimeout = 5 * time.Minute
const testResultFile = "/tmp/TestResult"

var rootDir string

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
		project := ConfigContext.ClusterLoader.Projects
		tuningSets := ConfigContext.ClusterLoader.TuningSets
		if project == nil {
			e2e.Failf("Invalid config file.\nFile: %v", project)
		}

		var namespaces []string
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
				// Create namespaces as defined in Cluster Loader config
				nsName := fmt.Sprintf("%s%d", p.Basename, j)
				err := oc.Run("new-project").Args(nsName).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("%d/%d : Created new namespace: %v", j+1, p.Number, nsName)
				namespaces = append(namespaces, nsName)

				// Create templates as defined
				for _, template := range p.Templates {
					var allArgs []string
					templateFile := mkPath(template.File)
					e2e.Logf("We're loading file %v: ", templateFile)
					templateObj, err := testutil.GetTemplateFixture(templateFile)
					if err != nil {
						e2e.Failf("Cant read template config file. Error: %v", err)
					}
					allArgs = append(allArgs, templateObj.Name)

					if template.Parameters == (ParameterConfigType{}) {
						e2e.Logf("Pod environment variables will not be modified.")
					} else {
						params := convertVariablesToString(template.Parameters)
						allArgs = append(allArgs, params...)
					}

					config, err := oc.AdminTemplateClient().Template().Templates(nsName).Create(templateObj)
					e2e.Logf("Template %v created, config: %+v", templateObj.Name, config)

					err = oc.SetNamespace(nsName).Run("new-app").Args(allArgs...).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				// This is too familiar, create pods
				for _, pod := range p.Pods {
					// Parse Pod file into struct
					config := ParsePods(mkPath(pod.File))
					// Check if environment variables are defined in CL config
					if pod.Parameters == (ParameterConfigType{}) {
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
					CreatePods(c, pod.Basename, nsName, labels, config.Spec, pod.Number, tuning)
				}
			}
		}

		// Wait for builds and deployments to complete
		for _, ns := range namespaces {
			buildList, err := oc.BuildClient().Build().Builds(ns).List(metav1.ListOptions{})
			if err != nil {
				e2e.Logf("Error listing builds: %v", err)
			}
			if len(buildList.Items) > 0 {
				buildName := buildList.Items[0].Name
				e2e.Logf("Waiting for build: %q", buildName)
				err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(ns), buildName, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(buildName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Build %q completed", buildName)

				// deploymentName is buildName without the -1 suffix
				deploymentName := buildName[:len(buildName)-2]
				e2e.Logf("Waiting for deployment: %q", deploymentName)
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), ns, deploymentName, 1, oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Deployment %q completed", deploymentName)
			}
		}

		// Calculate and log test duration
		testDuration := time.Since(testStartTime)
		e2e.Logf("Cluster loading duration: %s", testDuration)
		err := writeJSONToDisk(TestResult{testDuration}, testResultFile)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for pods to be running
		//for _, ns := range namespaces {
		//	label := labels.SelectorFromSet(labels.Set(map[string]string{"purpose": "test"}))
		//	err := testutils.WaitForPodsWithLabelRunning(c, ns, label)
		//	if err != nil {
		//		e2e.Failf("Got %v when trying to wait for the pods to start", err)
		//	}
		//	o.Expect(err).NotTo(o.HaveOccurred())
		//	e2e.Logf("All pods running in namespace %s.", ns)
		//}

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

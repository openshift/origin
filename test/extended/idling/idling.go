package idling

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

func createFixture(oc *exutil.CLI, path string) ([]string, []string, error) {
	output, err := oc.Run("create").Args("-f", path, "-o", "name").Output()
	if err != nil {
		return nil, nil, err
	}

	lines := strings.Split(output, "\n")

	resources := make([]string, 0, len(lines)-1)
	names := make([]string, 0, len(lines)-1)

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("expected type/name syntax, got: %q", line)
		}
		resources = append(resources, parts[0])
		names = append(names, parts[1])
	}

	return resources, names, nil
}

func checkSingleIdle(oc *exutil.CLI, idlingFile string, resources map[string][]string, resourceName string, kind string) {
	g.By("Idling the service")
	_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
	o.Expect(err).ToNot(o.HaveOccurred())

	g.By("Ensuring the scale is zero")
	objName := resources[resourceName][0]
	replicas, err := oc.Run("get").Args(resourceName+"/"+objName, "--output=jsonpath=\"{.spec.replicas}\"").Output()
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(replicas).To(o.ContainSubstring("0"))

	g.By("Fetching the service and checking the annotations are present")
	serviceName := resources["service"][0]
	endpoints, err := oc.KubeREST().Endpoints(oc.Namespace()).Get(serviceName)
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(endpoints.Annotations).To(o.HaveKey(unidlingapi.IdledAtAnnotation))
	o.Expect(endpoints.Annotations).To(o.HaveKey(unidlingapi.UnidleTargetAnnotation))

	g.By("Checking the idled-at time")
	idledAtAnnotation := endpoints.Annotations[unidlingapi.IdledAtAnnotation]
	idledAtTime, err := time.Parse(time.RFC3339, idledAtAnnotation)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(idledAtTime).To(o.BeTemporally("~", time.Now(), 5*time.Minute))

	g.By("Checking the idle targets")
	unidleTargetAnnotation := endpoints.Annotations[unidlingapi.UnidleTargetAnnotation]
	unidleTargets := []unidlingapi.RecordedScaleReference{}
	err = json.Unmarshal([]byte(unidleTargetAnnotation), &unidleTargets)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(unidleTargets).To(o.Equal([]unidlingapi.RecordedScaleReference{
		{
			Replicas: 2,
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Name: resources[resourceName][0],
				Kind: kind,
			},
		},
	}))
}

var _ = g.Describe("idling and uidling", func() {
	defer g.GinkgoRecover()
	var (
		oc                  = exutil.NewCLI("cli-idling", exutil.KubeConfigPath()).Verbose()
		echoServerFixture   = exutil.FixturePath("testdata", "idling-echo-server.yaml")
		echoServerRcFixture = exutil.FixturePath("testdata", "idling-echo-server-rc.yaml")
		framework           = oc.KubeFramework()
	)

	// path to the fixture
	var fixture string

	// path to the idling file
	var idlingFile string

	// map of all resources created from the fixtures
	var resources map[string][]string

	g.JustBeforeEach(func() {
		g.By("Creating the resources")
		rawResources, rawResourceNames, err := createFixture(oc, fixture)
		o.Expect(err).ToNot(o.HaveOccurred())

		resources = make(map[string][]string)
		for i, resource := range rawResources {
			resources[resource] = append(resources[resource], rawResourceNames[i])
		}

		g.By("Creating the idling file")
		serviceNames := resources["service"]

		targetFile, err := ioutil.TempFile(exutil.TestContext.OutputDir, "idling-services-")
		o.Expect(err).ToNot(o.HaveOccurred())
		defer targetFile.Close()
		idlingFile = targetFile.Name()
		_, err = targetFile.Write([]byte(strings.Join(serviceNames, "\n")))
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Waiting for the endpoints to exist")
		serviceName := resources["service"][0]
		g.By("Waiting for endpoints to be up")
		err = waitForEndpointsAvailable(oc, serviceName)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.AfterEach(func() {
		g.By("Cleaning up the idling file")
		os.Remove(idlingFile)
	})

	g.Describe("idling", func() {
		g.Context("with a single service and DeploymentConfig", func() {
			g.BeforeEach(func() {
				framework.BeforeEach()
				fixture = echoServerFixture
			})

			g.It("should idle the service and DeploymentConfig properly", func() {
				checkSingleIdle(oc, idlingFile, resources, "deploymentconfig", "DeploymentConfig")
			})
		})

		g.Context("with a single service and ReplicationController", func() {
			g.BeforeEach(func() {
				framework.BeforeEach()
				fixture = echoServerRcFixture
			})

			g.It("should idle the service and ReplicationController properly", func() {
				checkSingleIdle(oc, idlingFile, resources, "replicationcontroller", "ReplicationController")
			})
		})
	})
})

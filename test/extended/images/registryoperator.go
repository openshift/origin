package images

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	clientimageregistryv1 "github.com/openshift/client-go/imageregistry/clientset/versioned/typed/imageregistry/v1"
)

const (
	RegistryOperatorDeploymentNamespace = "openshift-image-registry"
	RegistryOperatorDeploymentName      = "cluster-image-registry-operator"
	ImageRegistryName                   = "image-registry"
	ImageRegistryResourceName           = "cluster"
	ImageRegistryOperatorResourceName   = "image-registry"
)

//Make sure Registry Operator is Available
func EnsureRegistryOperatorStatusIsAvailable(oc *exutil.CLI) (bool, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())

	err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("co", ImageRegistryOperatorResourceName).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	g.By("No error for Image Registry Operator")

	availablestatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", ImageRegistryOperatorResourceName, "-o=jsonpath={range .status.conditions[0]}{.status}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	progressingstatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", ImageRegistryOperatorResourceName, "-o=jsonpath={range .status.conditions[1]}{.status}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	degradestatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", ImageRegistryOperatorResourceName, "-o=jsonpath={range .status.conditions[2]}{.status}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if availablestatus == "True" && progressingstatus == "False" && degradestatus == "False" {
		g.By("Image registry operator is available")
	}

	return true, nil
}

func RegistryConfigClient(oc *exutil.CLI) clientimageregistryv1.ImageregistryV1Interface {
	return clientimageregistryv1.NewForConfigOrDie(oc.AdminConfig())
}

//Configure Image Registry Storage
func ConfigureImageRegistryStorageToEmptyDir(oc *exutil.CLI) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())

	config, err := RegistryConfigClient(oc).Configs().Get(
		context.Background(),
		ImageRegistryResourceName,
		metav1.GetOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("config is :\n%s", config)
	err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry.operator.openshift.io", ImageRegistryResourceName, "-p", `{"spec":{"replicas":1}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	time.Sleep(60 * time.Second)
	var hasstorage string
	if config.Status.Storage.EmptyDir != nil {
		e2e.Logf("Image Registry is already using EmptyDir")
	} else {
		switch {
		case config.Status.Storage.S3 != nil:
			hasstorage = "s3"
		case config.Status.Storage.Swift != nil:
			hasstorage = "swift"
		case config.Status.Storage.GCS != nil:
			hasstorage = "GCS"
		case config.Status.Storage.Azure != nil:
			hasstorage = "azure"
		case config.Status.Storage.PVC != nil:
			hasstorage = "pvc"
		default:
			e2e.Logf("Image Registry is using unknown storage type")
		}
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry.operator.openshift.io", ImageRegistryResourceName, "-p", `{"spec":{"storage":{"`+hasstorage+`":null,"emptyDir":{}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(60 * time.Second)
		err := oc.SetNamespace(RegistryOperatorDeploymentNamespace).AsAdmin().Run("rsh").Args(
			"deployments/image-registry", "mkdir", "--parents", "/registry/docker/registry").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("configs.imageregistry.operator.openshift.io", ImageRegistryResourceName, "--for=condition=Available").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Image Registry is using EmptyDir now")
	}
	return
}

func EnableRegistryPublicRoute(oc *exutil.CLI) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())

	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry.operator.openshift.io", ImageRegistryResourceName, "-p", `{"spec":{"defaultRoute":true}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	registryroute, err := oc.AsAdmin().Run("get").Args("route", "default-route", "-o=jsonpath={.spec.host}", "-n", RegistryOperatorDeploymentNamespace).Output()
	e2e.Logf("Default route for Image Registry is %s\n", registryroute)
}

func ConfigureImageRegistryToReadOnlyMode(oc *exutil.CLI) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())

	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry.operator.openshift.io", ImageRegistryResourceName, "-p", `{"spec":{"readOnly":true}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	time.Sleep(60 * time.Second)
	err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("configs.imageregistry.operator.openshift.io", ImageRegistryResourceName, "--for=condition=Available").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	deploy, err := oc.AdminKubeClient().AppsV1().Deployments(RegistryOperatorDeploymentNamespace).Get(context.Background(), ImageRegistryName, metav1.GetOptions{})
	found := false
	for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "REGISTRY_STORAGE_MAINTENANCE_READONLY" {
			if expected := "{enabled: true}"; env.Value != expected {
				e2e.Logf("%s: got %q, want %q", env.Name, env.Value, expected)
			} else {
				found = true
			}
		}
	}
	if !found {
		e2e.Logf("environment variable REGISTRY_STORAGE_MAINTENANCE_READONLY_ENABLED=true is not found")
	}
	//e2e.Logf("Image Registry is set to readyonly mode")
}

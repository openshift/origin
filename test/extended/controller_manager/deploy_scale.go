package controller_manager

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/api/apps"
	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/openshift/library-go/pkg/apps/appsutil"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("deployment-scale", exutil.KubeConfigPath())

	g.It("TestDeployScale", func() {
		namespace := oc.Namespace()
		adminConfig := oc.UserConfig()
		adminAppsClient := appsclient.NewForConfigOrDie(adminConfig)

		config := OkDeploymentConfig(0)
		config.Namespace = namespace
		config.Spec.Triggers = []appsv1.DeploymentTriggerPolicy{}
		config.Spec.Replicas = 1

		g.By("creating DeploymentConfig")
		dc, err := adminAppsClient.AppsV1().DeploymentConfigs(namespace).Create(config)
		o.Expect(err).NotTo(o.HaveOccurred())
		generation := dc.Generation

		{
			// Get scale subresource
			g.By("getting scale subresource")
			scalePath := fmt.Sprintf("/apis/apps.openshift.io/v1/namespaces/%s/deploymentconfigs/%s/scale", dc.Namespace, dc.Name)
			scale := &unstructured.Unstructured{}
			err := adminAppsClient.RESTClient().Get().AbsPath(scalePath).Do().Into(scale)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Ensure correct type
			o.Expect(scale.GetAPIVersion()).To(o.Equal("extensions/v1beta1"))
			scaleBytes, err := scale.MarshalJSON()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Ensure we can submit the same type back
			err = adminAppsClient.RESTClient().Put().AbsPath(scalePath).Body(scaleBytes).Do().Error()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		condition := func() (bool, error) {
			config, err := adminAppsClient.AppsV1().DeploymentConfigs(namespace).Get(dc.Name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			externalConfig := &appsv1.DeploymentConfig{}
			err = legacyscheme.Scheme.Convert(config, externalConfig, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
			return appsutil.HasSynced(externalConfig, generation), nil
		}
		g.By("waiting for DeploymentConfig to sync")
		err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, condition)
		o.Expect(err).NotTo(o.HaveOccurred())

		restMapper := restmapper.NewDeferredDiscoveryRESTMapper(discocache.NewMemCacheClient(adminAppsClient.Discovery()))
		restMapper.Reset()
		// we don't use cached discovery because DiscoveryScaleKindResolver does its own caching,
		// so we want to re-fetch every time when we actually ask for it
		scaleKindResolver := scale.NewDiscoveryScaleKindResolver(adminAppsClient.Discovery())
		scaleClient, err := scale.NewForConfig(adminConfig, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
		o.Expect(err).NotTo(o.HaveOccurred())

		scale, err := scaleClient.Scales(namespace).Get(apps.Resource("deploymentconfigs"), config.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scale.Spec.Replicas).To(o.Equal(1))

		g.By("scaling DeploymentConfig to 3 replicas")
		scaleUpdate := scale.DeepCopy()
		scaleUpdate.Spec.Replicas = 3
		updatedScale, err := scaleClient.Scales(namespace).Update(apps.Resource("deploymentconfigs"), scaleUpdate)
		// If this complains about "Scale" not being registered in "v1", check the kind overrides in the API registration in SubresourceGroupVersionKind
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(updatedScale.Spec.Replicas).To(o.Equal(3))

		persistedScale, err := scaleClient.Scales(namespace).Get(apps.Resource("deploymentconfigs"), config.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(persistedScale.Spec.Replicas).To(o.Equal(3))
	})
})

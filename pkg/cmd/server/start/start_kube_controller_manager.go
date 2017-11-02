package start

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controlleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/volume"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

// newKubeControllerContext provides a function which overrides the default and plugs a different set of informers in
func newKubeControllerContext(informers *informers) func(s *controlleroptions.CMServer, rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (controllerapp.ControllerContext, error) {
	oldContextFunc := controllerapp.CreateControllerContext
	return func(s *controlleroptions.CMServer, rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (controllerapp.ControllerContext, error) {
		ret, err := oldContextFunc(s, rootClientBuilder, clientBuilder, stop)
		if err != nil {
			return controllerapp.ControllerContext{}, err
		}

		// Overwrite the informers.  Since nothing accessed the existing informers that we're overwriting, they are inert.
		// TODO Remove this.  It keeps in-process memory utilization down, but we shouldn't do it.
		ret.InformerFactory = newGenericInformers(informers)

		return ret, nil
	}
}

func kubeControllerManagerAddFlags(cmserver *controlleroptions.CMServer) func(flags *pflag.FlagSet) {
	return func(flags *pflag.FlagSet) {
		cmserver.AddFlags(flags, controllerapp.KnownControllers(), controllerapp.ControllersDisabledByDefault.List())
	}
}

func newKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, recyclerImage string, dynamicProvisioningEnabled bool, controllerArgs map[string][]string) (*controlleroptions.CMServer, []func(), error) {
	cmdLineArgs := map[string][]string{}
	// deep-copy the input args to avoid mutation conflict.
	for k, v := range controllerArgs {
		cmdLineArgs[k] = append([]string{}, v...)
	}
	cleanupFunctions := []func(){}

	if _, ok := cmdLineArgs["controllers"]; !ok {
		cmdLineArgs["controllers"] = []string{
			"*", // start everything but the exceptions}
			// not used in openshift
			"-ttl",
			"-bootstrapsigner",
			"-tokencleaner",
			// we have to configure this separately until it is generic
			"-horizontalpodautoscaling",
			// we carry patches on this. For now....
			"-serviceaccount-token",
		}
	}
	if _, ok := cmdLineArgs["service-account-private-key-file"]; !ok {
		cmdLineArgs["service-account-private-key-file"] = []string{saPrivateKeyFile}
	}
	if _, ok := cmdLineArgs["root-ca-file"]; !ok {
		cmdLineArgs["root-ca-file"] = []string{saRootCAFile}
	}
	if _, ok := cmdLineArgs["kubeconfig"]; !ok {
		cmdLineArgs["kubeconfig"] = []string{kubeconfigFile}
	}
	if _, ok := cmdLineArgs["pod-eviction-timeout"]; !ok {
		cmdLineArgs["pod-eviction-timeout"] = []string{podEvictionTimeout}
	}
	if _, ok := cmdLineArgs["enable-dynamic-provisioning"]; !ok {
		cmdLineArgs["enable-dynamic-provisioning"] = []string{strconv.FormatBool(dynamicProvisioningEnabled)}
	}

	// disable serving http since we didn't used to expose it
	if _, ok := cmdLineArgs["port"]; !ok {
		cmdLineArgs["port"] = []string{"-1"}
	}

	// these force "default" values to match what we want
	if _, ok := cmdLineArgs["use-service-account-credentials"]; !ok {
		cmdLineArgs["use-service-account-credentials"] = []string{"true"}
	}
	if _, ok := cmdLineArgs["cluster-signing-cert-file"]; !ok {
		cmdLineArgs["cluster-signing-cert-file"] = []string{""}
	}
	if _, ok := cmdLineArgs["cluster-signing-key-file"]; !ok {
		cmdLineArgs["cluster-signing-key-file"] = []string{""}
	}
	if _, ok := cmdLineArgs["experimental-cluster-signing-duration"]; !ok {
		cmdLineArgs["experimental-cluster-signing-duration"] = []string{"720h"}
	}
	if _, ok := cmdLineArgs["leader-elect-retry-period"]; !ok {
		cmdLineArgs["leader-elect-retry-period"] = []string{"3s"}
	}
	if _, ok := cmdLineArgs["leader-elect-resource-lock"]; !ok {
		cmdLineArgs["leader-elect-resource-lock"] = []string{"configmaps"}
	}

	_, hostPathTemplateSet := cmdLineArgs["pv-recycler-pod-template-filepath-hostpath"]
	_, nfsTemplateSet := cmdLineArgs["pv-recycler-pod-template-filepath-nfs"]
	if !hostPathTemplateSet || !nfsTemplateSet {
		// OpenShift uses a different default volume recycler template than
		// Kubernetes. This default template is hardcoded in Kubernetes and it
		// isn't possible to pass it via ControllerContext. Crate a temporary
		// file with OpenShift's template and let's pretend it was set by user
		// as --recycler-pod-template-filepath-hostpath and
		// --pv-recycler-pod-template-filepath-nfs arguments.
		// This template then needs to be deleted by caller!
		templateFilename, err := createRecylerTemplate(recyclerImage)
		if err != nil {
			return nil, nil, err
		}

		cleanupFunctions = append(cleanupFunctions, func() {
			// Remove the template when it's not needed. This is called aftet
			// controller is initialized
			glog.V(4).Infof("Removing temporary file %s", templateFilename)
			err := os.Remove(templateFilename)
			if err != nil {
				glog.Warningf("Failed to remove %s: %v", templateFilename, err)
			}
		})

		if !hostPathTemplateSet {
			cmdLineArgs["pv-recycler-pod-template-filepath-hostpath"] = []string{templateFilename}
		}
		if !nfsTemplateSet {
			cmdLineArgs["pv-recycler-pod-template-filepath-nfs"] = []string{templateFilename}
		}
	}

	// resolve arguments
	controllerManager := controlleroptions.NewCMServer()
	if err := cmdflags.Resolve(cmdLineArgs, kubeControllerManagerAddFlags(controllerManager)); len(err) > 0 {
		return nil, cleanupFunctions, kerrors.NewAggregate(err)
	}

	// TODO make this configurable or discoverable.  This is going to prevent us from running the stock GC controller
	// IF YOU ADD ANYTHING TO THIS LIST, MAKE SURE THAT YOU UPDATE THEIR STRATEGIES TO PREVENT GC FINALIZERS
	controllerManager.GCIgnoredResources = append(controllerManager.GCIgnoredResources,
		// explicitly disabled from GC for now - not enough value to track them
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindingrestrictions"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "clusternetworks"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "egressnetworkpolicies"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "hostsubnets"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "netnamespaces"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthclientauthorizations"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthclients"},
		componentconfig.GroupResource{Group: "quota.openshift.io", Resource: "clusterresourcequotas"},
		componentconfig.GroupResource{Group: "user.openshift.io", Resource: "groups"},
		componentconfig.GroupResource{Group: "user.openshift.io", Resource: "identities"},
		componentconfig.GroupResource{Group: "user.openshift.io", Resource: "users"},
		componentconfig.GroupResource{Group: "image.openshift.io", Resource: "images"},

		// virtual resource
		componentconfig.GroupResource{Group: "project.openshift.io", Resource: "projects"},
		// these resources contain security information in their names, and we don't need to track them
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthaccesstokens"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthauthorizetokens"},
		// exposed already as cronjobs
		componentconfig.GroupResource{Group: "batch", Resource: "scheduledjobs"},
		// exposed already as extensions v1beta1 by other controllers
		componentconfig.GroupResource{Group: "apps", Resource: "deployments"},
		// exposed as autoscaling v1
		componentconfig.GroupResource{Group: "extensions", Resource: "horizontalpodautoscalers"},
		// exposed as security.openshift.io v1
		componentconfig.GroupResource{Group: "", Resource: "securitycontextconstraints"},
	)

	return controllerManager, cleanupFunctions, nil
}

func createRecylerTemplate(recyclerImage string) (string, error) {
	uid := int64(0)
	template := volume.NewPersistentVolumeRecyclerPodTemplate()
	template.Namespace = "openshift-infra"
	template.Spec.ServiceAccountName = bootstrappolicy.InfraPersistentVolumeRecyclerControllerServiceAccountName
	template.Spec.Containers[0].Image = recyclerImage
	template.Spec.Containers[0].Command = []string{"/usr/bin/openshift-recycle"}
	template.Spec.Containers[0].Args = []string{"/scrub"}
	template.Spec.Containers[0].SecurityContext = &kapiv1.SecurityContext{RunAsUser: &uid}
	template.Spec.Containers[0].ImagePullPolicy = kapiv1.PullIfNotPresent

	templateBytes, err := runtime.Encode(kapi.Codecs.LegacyCodec(kapiv1.SchemeGroupVersion), template)
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile("", "openshift-recycler-template-")
	if err != nil {
		return "", err
	}
	filename := f.Name()
	glog.V(4).Infof("Creating file %s with recycler templates", filename)

	_, err = f.Write(templateBytes)
	if err != nil {
		f.Close()
		os.Remove(filename)
		return "", err
	}
	f.Close()
	return filename, nil
}

func runEmbeddedKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout string, dynamicProvisioningEnabled bool, cmdLineArgs map[string][]string,
	recyclerImage string, informers *informers) {
	controllerapp.CreateControllerContext = newKubeControllerContext(informers)
	controllerapp.StartInformers = func(stop <-chan struct{}) {
		informers.Start(stop)
	}

	// TODO we need a real identity for this.  Right now it's just using the loopback connection like it used to.
	controllerManager, cleanupFunctions, err := newKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, recyclerImage, dynamicProvisioningEnabled, cmdLineArgs)
	defer func() {
		// Clean up any temporary files and similar stuff.
		// TODO: Make sure this defer is actually called - controllerapp.Run()
		// below never returns -> defer is not called.
		for _, f := range cleanupFunctions {
			f()
		}
	}()

	if err != nil {
		glog.Fatal(err)
	}
	// this does a second leader election, but doing the second leader election will allow us to move out process in
	// 3.8 if we so choose.
	if err := controllerapp.Run(controllerManager); err != nil {
		glog.Fatal(err)
	}
}

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)
}

// genericInternalResourceInformerFunc will return an internal informer for any resource matching
// its group resource, instead of the external version. Only valid for use where the type is accessed
// via generic interfaces, such as the garbage collector with ObjectMeta.
type genericInternalResourceInformerFunc func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)

func (fn genericInternalResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	resource.Version = runtime.APIVersionInternal
	return fn(resource)
}

type genericInformers struct {
	kinformers.SharedInformerFactory
	generic []GenericResourceInformer
	// bias is a map that tries loading an informer from another GVR before using the original
	bias map[schema.GroupVersionResource]schema.GroupVersionResource
}

func newGenericInformers(informers *informers) genericInformers {
	return genericInformers{
		SharedInformerFactory: informers.GetExternalKubeInformers(),
		generic: []GenericResourceInformer{
			// use our existing internal informers to satisfy the generic informer requests (which don't require strong
			// types).
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.appInformers.ForResource(resource)
			}),
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.authorizationInformers.ForResource(resource)
			}),
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.buildInformers.ForResource(resource)
			}),
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.imageInformers.ForResource(resource)
			}),
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.quotaInformers.ForResource(resource)
			}),
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.securityInformers.ForResource(resource)
			}),
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.templateInformers.ForResource(resource)
			}),
			informers.externalKubeInformers,
			genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
				return informers.internalKubeInformers.ForResource(resource)
			}),
		},
		bias: map[schema.GroupVersionResource]schema.GroupVersionResource{
			{Group: "rbac.authorization.k8s.io", Resource: "rolebindings", Version: "v1beta1"}:        {Group: "rbac.authorization.k8s.io", Resource: "rolebindings", Version: runtime.APIVersionInternal},
			{Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Version: "v1beta1"}: {Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Version: runtime.APIVersionInternal},
			{Group: "rbac.authorization.k8s.io", Resource: "roles", Version: "v1beta1"}:               {Group: "rbac.authorization.k8s.io", Resource: "roles", Version: runtime.APIVersionInternal},
			{Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Version: "v1beta1"}:        {Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Version: runtime.APIVersionInternal},
			{Group: "", Resource: "securitycontextconstraints", Version: "v1"}:                        {Group: "", Resource: "securitycontextconstraints", Version: runtime.APIVersionInternal},
		},
	}
}

func (i genericInformers) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	if try, ok := i.bias[resource]; ok {
		if res, err := i.ForResource(try); err == nil {
			return res, nil
		}
	}

	informer, firstErr := i.SharedInformerFactory.ForResource(resource)
	if firstErr == nil {
		return informer, nil
	}
	for _, generic := range i.generic {
		if informer, err := generic.ForResource(resource); err == nil {
			return informer, nil
		}
	}
	glog.V(4).Infof("Couldn't find informer for %v", resource)
	return nil, firstErr
}

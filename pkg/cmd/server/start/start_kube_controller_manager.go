package start

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kinformers "k8s.io/client-go/informers"
	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controlleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/volume"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

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
		// virtual and unwatchable resource, surfaced via rbac.authorization.k8s.io objects
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "clusterroles"},
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "clusterrolebindings"},
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "roles"},
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindings"},
		// these resources contain security information in their names, and we don't need to track them
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthaccesstokens"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthauthorizetokens"},
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

	templateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(kapiv1.SchemeGroupVersion), template)
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

	// Overwrite the informers, because we have our custom generic informers for quota.
	// TODO update quota to create its own informer like garbage collection or if we split this out, actually add our external types to the kube generic informer
	controllerapp.InformerFactoryOverride = externalKubeInformersWithExtraGenerics{
		SharedInformerFactory:   informers.GetExternalKubeInformers(),
		genericResourceInformer: informers.ToGenericInformer(),
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

type externalKubeInformersWithExtraGenerics struct {
	kinformers.SharedInformerFactory
	genericResourceInformer GenericResourceInformer
}

func (i externalKubeInformersWithExtraGenerics) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	return i.genericResourceInformer.ForResource(resource)
}

func (i externalKubeInformersWithExtraGenerics) Start(stopCh <-chan struct{}) {
	i.SharedInformerFactory.Start(stopCh)
	i.genericResourceInformer.Start(stopCh)
}

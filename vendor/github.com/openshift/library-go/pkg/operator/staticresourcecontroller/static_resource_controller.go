package staticresourcecontroller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/api"
	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	workQueueKey = "key"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(api.InstallKube(genericScheme))
}

type StaticResourceController struct {
	name      string
	manifests resourceapply.AssetFunc
	files     []string

	operatorClient v1helpers.OperatorClient
	clients        *resourceapply.ClientHolder

	eventRecorder events.Recorder

	factory *factory.Factory
}

// NewStaticResourceController returns a controller that maintains certain static manifests. Most "normal" types are supported,
// but feel free to add ones we missed.  Use .AddInformer(), .AddKubeInformers(), .AddNamespaceInformer or to provide triggering conditions.
func NewStaticResourceController(
	name string,
	manifests resourceapply.AssetFunc,
	files []string,
	clients *resourceapply.ClientHolder,
	operatorClient v1helpers.OperatorClient,
	eventRecorder events.Recorder,
) *StaticResourceController {
	c := &StaticResourceController{
		name:      name,
		manifests: manifests,
		files:     files,

		operatorClient: operatorClient,
		clients:        clients,

		eventRecorder: eventRecorder.WithComponentSuffix(strings.ToLower(name)),

		factory: factory.New().WithInformers(operatorClient.Informer()).ResyncEvery(1 * time.Minute),
	}

	return c
}

func (c *StaticResourceController) AddKubeInformers(kubeInformersByNamespace v1helpers.KubeInformersForNamespaces) *StaticResourceController {
	// set the informers so we can have caching clients
	c.clients = c.clients.WithKubernetesInformers(kubeInformersByNamespace)

	ret := c
	for _, file := range c.files {
		objBytes, err := c.manifests(file)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("missing %q: %v", file, err))
			continue
		}
		requiredObj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("cannot decode %q: %v", file, err))
			continue
		}
		metadata, err := meta.Accessor(requiredObj)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("cannot get metadata %q: %v", file, err))
			continue
		}

		// find the right subset of informers.  Interestingly, cluster scoped resources require cluster scoped informers
		var informer informers.SharedInformerFactory
		if _, ok := requiredObj.(*corev1.Namespace); ok {
			informer = kubeInformersByNamespace.InformersFor(metadata.GetName())
			if informer == nil {
				utilruntime.HandleError(fmt.Errorf("missing informer for namespace %q; no dynamic wiring added, time-based only.", metadata.GetName()))
				continue
			}
		} else {
			informer = kubeInformersByNamespace.InformersFor(metadata.GetNamespace())
			if informer == nil {
				utilruntime.HandleError(fmt.Errorf("missing informer for namespace %q; no dynamic wiring added, time-based only.", metadata.GetNamespace()))
				continue
			}
		}

		// iterate through the resources we know that are related to kube informers and add the pertinent informers
		switch t := requiredObj.(type) {
		case *corev1.Namespace:
			ret = ret.AddNamespaceInformer(informer.Core().V1().Namespaces().Informer(), t.Name)
		case *corev1.Service:
			ret = ret.AddInformer(informer.Core().V1().Namespaces().Informer())
		case *corev1.Pod:
			ret = ret.AddInformer(informer.Core().V1().Pods().Informer())
		case *corev1.ServiceAccount:
			ret = ret.AddInformer(informer.Core().V1().ServiceAccounts().Informer())
		case *corev1.ConfigMap:
			ret = ret.AddInformer(informer.Core().V1().ConfigMaps().Informer())
		case *corev1.Secret:
			ret = ret.AddInformer(informer.Core().V1().Secrets().Informer())
		case *rbacv1.ClusterRole:
			ret = ret.AddInformer(informer.Rbac().V1().ClusterRoles().Informer())
		case *rbacv1.ClusterRoleBinding:
			ret = ret.AddInformer(informer.Rbac().V1().ClusterRoleBindings().Informer())
		case *rbacv1.Role:
			ret = ret.AddInformer(informer.Rbac().V1().Roles().Informer())
		case *rbacv1.RoleBinding:
			ret = ret.AddInformer(informer.Rbac().V1().RoleBindings().Informer())
		case *storagev1.StorageClass:
			ret = ret.AddInformer(informer.Storage().V1().StorageClasses().Informer())
		case *storagev1.CSIDriver:
			ret = ret.AddInformer(informer.Storage().V1().CSIDrivers().Informer())
		default:
			// if there's a missing case, the caller can add an informer or count on a time based trigger.
			// if the controller doesn't handle it, then there will be failure from the underlying apply.
			klog.V(4).Infof("unhandled type %T", requiredObj)
		}
	}

	return ret
}

func (c *StaticResourceController) AddInformer(informer cache.SharedIndexInformer) *StaticResourceController {
	c.factory.WithInformers(informer)
	return c
}

func (c *StaticResourceController) AddNamespaceInformer(informer cache.SharedIndexInformer, namespaces ...string) *StaticResourceController {
	c.factory.WithNamespaceInformer(informer, namespaces...)
	return c
}

func (c StaticResourceController) Sync(ctx context.Context, syncContext factory.SyncContext) error {
	operatorSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}
	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	errors := []error{}
	directResourceResults := resourceapply.ApplyDirectly(c.clients, syncContext.Recorder(), c.manifests, c.files...)
	for _, currResult := range directResourceResults {
		if currResult.Error != nil {
			errors = append(errors, fmt.Errorf("%q (%T): %v", currResult.File, currResult.Type, currResult.Error))
			continue
		}
	}

	if len(errors) > 0 {
		message := ""
		for _, err := range errors {
			message = message + err.Error() + "\n"
		}
		errors = append(errors,
			appendErrors(v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
				Type:    fmt.Sprintf("%sDegraded", c.name),
				Status:  operatorv1.ConditionTrue,
				Reason:  "SyncError",
				Message: message,
			})))...,
		)
	} else {
		errors = append(errors,
			appendErrors(v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
				Type:   fmt.Sprintf("%sDegraded", c.name),
				Status: operatorv1.ConditionFalse,
				Reason: "AsExpected",
			})))...,
		)
	}

	return utilerrors.NewAggregate(errors)
}

func appendErrors(_ *operatorv1.OperatorStatus, _ bool, err error) []error {
	if err != nil {
		return []error{err}
	}
	return []error{}
}

func (c *StaticResourceController) Name() string {
	return "StaticResourceController"
}

func (c *StaticResourceController) Run(ctx context.Context, workers int) {
	c.factory.WithSync(c.Sync).ToController(c.Name(), c.eventRecorder).Run(ctx, workers)
}

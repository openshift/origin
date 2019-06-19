package requestlimit

import (
	"errors"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	"github.com/openshift/api/project"
	usertypedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/library-go/pkg/apiserver/admission/admissionrestconfig"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/origin/pkg/api/legacy"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	requestlimitapi "github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit"
	v1 "github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/v1"
	requestlimitapivalidation "github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/validation"
	uservalidation "github.com/openshift/origin/pkg/user/apis/user/validation"
)

// allowedTerminatingProjects is the number of projects that are owned by a user, are in terminating state,
// and do not count towards the user's limit.
const allowedTerminatingProjects = 2

const timeToWaitForCacheSync = 10 * time.Second

func Register(plugins *admission.Plugins) {
	plugins.Register("project.openshift.io/ProjectRequestLimit",
		func(config io.Reader) (admission.Interface, error) {
			pluginConfig, err := readConfig(config)
			if err != nil {
				return nil, err
			}
			if pluginConfig == nil {
				klog.Infof("Admission plugin %q is not configured so it will be disabled.", "project.openshift.io/ProjectRequestLimit")
				return nil, nil
			}
			return NewProjectRequestLimit(pluginConfig)
		})
}

func readConfig(reader io.Reader) (*requestlimitapi.ProjectRequestLimitConfig, error) {
	obj, err := helpers.ReadYAMLToInternal(reader, requestlimitapi.Install, v1.Install)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*requestlimitapi.ProjectRequestLimitConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	errs := requestlimitapivalidation.ValidateProjectRequestLimitConfig(config)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return config, nil
}

type projectRequestLimit struct {
	*admission.Handler
	userClient     usertypedclient.UsersGetter
	config         *requestlimitapi.ProjectRequestLimitConfig
	nsLister       corev1listers.NamespaceLister
	nsListerSynced func() bool
}

// ensure that the required Openshift admission interfaces are implemented
var _ = initializer.WantsExternalKubeInformerFactory(&projectRequestLimit{})
var _ = admissionrestconfig.WantsRESTClientConfig(&projectRequestLimit{})
var _ = admission.ValidationInterface(&projectRequestLimit{})

// Admit ensures that only a configured number of projects can be requested by a particular user.
func (o *projectRequestLimit) Validate(a admission.Attributes, _ admission.ObjectInterfaces) (err error) {
	if o.config == nil {
		return nil
	}
	switch a.GetResource().GroupResource() {
	case project.Resource("projectrequests"), legacy.Resource("projectrequests"):
	default:
		return nil
	}
	if _, isProjectRequest := a.GetObject().(*projectapi.ProjectRequest); !isProjectRequest {
		return nil
	}

	if !o.waitForSyncedStore(time.After(timeToWaitForCacheSync)) {
		return admission.NewForbidden(a, errors.New("project.openshift.io/ProjectRequestLimit: caches not synchronized"))
	}

	userName := a.GetUserInfo().GetName()
	projectCount, err := o.projectCountByRequester(userName)
	if err != nil {
		return err
	}
	maxProjects, hasLimit, err := o.maxProjectsByRequester(userName)
	if err != nil {
		return err
	}
	if hasLimit && projectCount >= maxProjects {
		return admission.NewForbidden(a, fmt.Errorf("user %s cannot create more than %d project(s).", userName, maxProjects))
	}
	return nil
}

// maxProjectsByRequester returns the maximum number of projects allowed for a given user, whether a limit exists, and an error
// if an error occurred. If a limit doesn't exist, the maximum number should be ignored.
func (o *projectRequestLimit) maxProjectsByRequester(userName string) (int, bool, error) {
	// service accounts have a different ruleset, check them
	if _, _, err := serviceaccount.SplitUsername(userName); err == nil {
		if o.config.MaxProjectsForServiceAccounts == nil {
			return 0, false, nil
		}

		return *o.config.MaxProjectsForServiceAccounts, true, nil
	}

	// if we aren't a valid username, we came in as cert user for certain, use our cert user rules
	if reasons := uservalidation.ValidateUserName(userName, false); len(reasons) != 0 {
		if o.config.MaxProjectsForSystemUsers == nil {
			return 0, false, nil
		}

		return *o.config.MaxProjectsForSystemUsers, true, nil
	}

	// prevent a user lookup if no limits are configured
	if len(o.config.Limits) == 0 {
		return 0, false, nil
	}

	user, err := o.userClient.Users().Get(userName, metav1.GetOptions{})
	if err != nil {
		return 0, false, err
	}
	userLabels := labels.Set(user.Labels)

	for _, limit := range o.config.Limits {
		selector := labels.Set(limit.Selector).AsSelector()
		if selector.Matches(userLabels) {
			if limit.MaxProjects == nil {
				return 0, false, nil
			}
			return *limit.MaxProjects, true, nil
		}
	}
	return 0, false, nil
}

func (o *projectRequestLimit) projectCountByRequester(userName string) (int, error) {
	// our biggest clusters have less than 10k namespaces.  project requests are infrequent.  This is iterating on an
	// in memory set of pointers.  I can live with all this to avoid a secondary cache.
	allNamespaces, err := o.nsLister.List(labels.Everything())
	if err != nil {
		return 0, err
	}
	namespaces := []*corev1.Namespace{}
	for i := range allNamespaces {
		ns := allNamespaces[i]
		if ns.Annotations[projectapi.ProjectRequester] == userName {
			namespaces = append(namespaces, ns)
		}
	}

	terminatingCount := 0
	for _, ns := range namespaces {
		if ns.Status.Phase == corev1.NamespaceTerminating {
			terminatingCount++
		}
	}
	count := len(namespaces)
	if terminatingCount > allowedTerminatingProjects {
		count -= allowedTerminatingProjects
	} else {
		count -= terminatingCount
	}
	return count, nil
}

func (o *projectRequestLimit) SetRESTClientConfig(restClientConfig rest.Config) {
	var err error
	o.userClient, err = usertypedclient.NewForConfig(&restClientConfig)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
}

func (o *projectRequestLimit) SetExternalKubeInformerFactory(kubeInformers informers.SharedInformerFactory) {
	o.nsLister = kubeInformers.Core().V1().Namespaces().Lister()
	o.nsListerSynced = kubeInformers.Core().V1().Namespaces().Informer().HasSynced
}

func (o *projectRequestLimit) waitForSyncedStore(timeout <-chan time.Time) bool {
	for !o.nsListerSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return o.nsListerSynced()
		}
	}

	return true
}

func (o *projectRequestLimit) ValidateInitialization() error {
	if o.userClient == nil {
		return fmt.Errorf("project.openshift.io/ProjectRequestLimit plugin requires an Openshift client")
	}
	if o.nsLister == nil {
		return fmt.Errorf("project.openshift.io/ProjectRequestLimit plugin needs a namespace lister")
	}
	if o.nsListerSynced == nil {
		return fmt.Errorf("project.openshift.io/ProjectRequestLimit plugin needs a namespace lister synced")
	}
	return nil
}

func NewProjectRequestLimit(config *requestlimitapi.ProjectRequestLimitConfig) (admission.Interface, error) {
	return &projectRequestLimit{
		config:  config,
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

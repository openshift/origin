package restrictusers

import (
	"errors"
	"fmt"
	"io"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/rbac"

	userapi "github.com/openshift/api/user/v1"
	authorizationv1informer "github.com/openshift/client-go/authorization/informers/externalversions/authorization/v1"
	authorizationv1listers "github.com/openshift/client-go/authorization/listers/authorization/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("authorization.openshift.io/RestrictSubjectBindings",
		func(config io.Reader) (admission.Interface, error) {
			return NewRestrictUsersAdmission()
		})
}

type GroupCache interface {
	GroupsFor(string) ([]*userapi.Group, error)
}

// restrictUsersAdmission implements admission.ValidateInterface and enforces
// restrictions on adding rolebindings in a project to permit only designated
// subjects.
type restrictUsersAdmission struct {
	*admission.Handler

	rbrLister  authorizationv1listers.RoleBindingRestrictionLister
	rbrSynced  cache.InformerSynced
	userClient userclient.Interface
	kubeClient kubernetes.Interface
	groupCache GroupCache
}

var _ = oadmission.WantsRESTClientConfig(&restrictUsersAdmission{})
var _ = oadmission.WantsUserInformer(&restrictUsersAdmission{})
var _ = oadmission.WantsRoleBindingRestrictionInformer(&restrictUsersAdmission{})
var _ = initializer.WantsExternalKubeClientSet(&restrictUsersAdmission{})
var _ = admission.ValidationInterface(&restrictUsersAdmission{})

const (
	timeToWaitForCacheSync = 10 * time.Second
)

// NewRestrictUsersAdmission configures an admission plugin that enforces
// restrictions on adding role bindings in a project.
func NewRestrictUsersAdmission() (admission.Interface, error) {
	return &restrictUsersAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

func (q *restrictUsersAdmission) SetExternalKubeClientSet(c kubernetes.Interface) {
	q.kubeClient = c
}

func (q *restrictUsersAdmission) SetRESTClientConfig(restClientConfig rest.Config) {
	var err error

	q.userClient, err = userclient.NewForConfig(&restClientConfig)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
}

func (q *restrictUsersAdmission) SetRoleBindingRestrictionInformer(informers authorizationv1informer.RoleBindingRestrictionInformer) {
	q.rbrLister = informers.Lister()
	q.rbrSynced = informers.Informer().HasSynced
}

func (q *restrictUsersAdmission) SetUserInformer(userInformers userinformer.SharedInformerFactory) {
	q.groupCache = usercache.NewGroupCache(userInformers.User().V1().Groups())
}

// subjectsDelta returns the relative complement of elementsToIgnore in
// elements (i.e., elements∖elementsToIgnore).
func subjectsDelta(elementsToIgnore, elements []rbac.Subject) []rbac.Subject {
	result := []rbac.Subject{}

	for _, el := range elements {
		keep := true
		for _, skipEl := range elementsToIgnore {
			if el == skipEl {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, el)
		}
	}

	return result
}

// Admit makes admission decisions that enforce restrictions on adding
// project-scoped role-bindings.  In order for a role binding to be permitted,
// each subject in the binding must be matched by some rolebinding restriction
// in the namespace.
func (q *restrictUsersAdmission) Validate(a admission.Attributes, _ admission.ObjectInterfaces) (err error) {

	// We only care about rolebindings
	if a.GetResource().GroupResource() != rbac.Resource("rolebindings") {
		return nil
	}

	// Ignore all operations that correspond to subresource actions.
	if len(a.GetSubresource()) != 0 {
		return nil
	}

	ns := a.GetNamespace()
	// Ignore cluster-level resources.
	if len(ns) == 0 {
		return nil
	}

	var oldSubjects []rbac.Subject

	obj, oldObj := a.GetObject(), a.GetOldObject()

	rolebinding, ok := obj.(*rbac.RoleBinding)
	if !ok {
		return admission.NewForbidden(a,
			fmt.Errorf("wrong object type for new rolebinding: %T", obj))
	}

	if len(rolebinding.Subjects) == 0 {
		klog.V(4).Infof("No new subjects; admitting")
		return nil
	}

	if oldObj != nil {
		oldrolebinding, ok := oldObj.(*rbac.RoleBinding)
		if !ok {
			return admission.NewForbidden(a,
				fmt.Errorf("wrong object type for old rolebinding: %T", oldObj))
		}
		oldSubjects = oldrolebinding.Subjects
	}

	klog.V(4).Infof("Handling rolebinding %s/%s",
		rolebinding.Namespace, rolebinding.Name)

	newSubjects := subjectsDelta(oldSubjects, rolebinding.Subjects)
	if len(newSubjects) == 0 {
		klog.V(4).Infof("No new subjects; admitting")
		return nil
	}

	if !q.waitForSyncedStore(time.After(timeToWaitForCacheSync)) {
		return admission.NewForbidden(a, errors.New("rbr cache not synchronized"))
	}

	roleBindingRestrictionList, err := q.rbrLister.List(labels.Everything())
	if err != nil {
		return admission.NewForbidden(a, fmt.Errorf("could not list rolebinding restrictions: %v", err))
	}
	if len(roleBindingRestrictionList) == 0 {
		klog.V(4).Infof("No rolebinding restrictions specified; admitting")
		return nil
	}

	checkers := []SubjectChecker{}
	for _, rbr := range roleBindingRestrictionList {
		checker, err := NewSubjectChecker(&rbr.Spec)
		if err != nil {
			return admission.NewForbidden(a, fmt.Errorf("could not create rolebinding restriction subject checker: %v", err))
		}
		checkers = append(checkers, checker)
	}

	roleBindingRestrictionContext, err := newRoleBindingRestrictionContext(ns,
		q.kubeClient, q.userClient.UserV1(), q.groupCache)
	if err != nil {
		return admission.NewForbidden(a, fmt.Errorf("could not create rolebinding restriction context: %v", err))
	}

	checker := NewUnionSubjectChecker(checkers)

	errs := []error{}
	for _, subject := range newSubjects {
		allowed, err := checker.Allowed(subject, roleBindingRestrictionContext)
		if err != nil {
			errs = append(errs, err)
		}
		if !allowed {
			errs = append(errs,
				fmt.Errorf("rolebindings to %s %q are not allowed in project %q",
					subject.Kind, subject.Name, ns))
		}
	}
	if len(errs) != 0 {
		return admission.NewForbidden(a, kerrors.NewAggregate(errs))
	}

	klog.V(4).Infof("All new subjects are allowed; admitting")

	return nil
}

func (q *restrictUsersAdmission) ValidateInitialization() error {
	if q.kubeClient == nil {
		return errors.New("RestrictUsersAdmission plugin requires a Kubernetes client")
	}
	if q.rbrLister == nil {
		return errors.New("RestrictUsersAdmission requires a rbrLister")
	}
	if q.rbrSynced == nil {
		return errors.New("RestrictUsersAdmission plugin requires a rbrlister synced")
	}
	if q.userClient == nil {
		return errors.New("RestrictUsersAdmission plugin requires an OpenShift user client")
	}
	if q.groupCache == nil {
		return errors.New("RestrictUsersAdmission plugin requires a group cache")
	}

	return nil
}

func (q *restrictUsersAdmission) waitForSyncedStore(timeout <-chan time.Time) bool {
	for !q.rbrSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return q.rbrSynced()
		}
	}

	return true
}

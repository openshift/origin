package restrictusers

import (
	"errors"
	"fmt"
	"io"

	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	userapi "github.com/openshift/api/user/v1"
	authorizationclient "github.com/openshift/client-go/authorization/clientset/versioned"
	authorizationtypedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("openshift.io/RestrictSubjectBindings",
		func(config io.Reader) (admission.Interface, error) {
			return NewRestrictUsersAdmission()
		})
}

type GroupCache interface {
	GroupsFor(string) ([]*userapi.Group, error)
}

// restrictUsersAdmission implements admission.Interface and enforces
// restrictions on adding rolebindings in a project to permit only designated
// subjects.
type restrictUsersAdmission struct {
	*admission.Handler

	roleBindingRestrictionsGetter authorizationtypedclient.RoleBindingRestrictionsGetter
	userClient                    userclient.Interface
	kclient                       kclientset.Interface
	groupCache                    GroupCache
}

var _ = oadmission.WantsOpenshiftInternalAuthorizationClient(&restrictUsersAdmission{})
var _ = oadmission.WantsOpenshiftInternalUserClient(&restrictUsersAdmission{})
var _ = oadmission.WantsUserInformer(&restrictUsersAdmission{})
var _ = kadmission.WantsInternalKubeClientSet(&restrictUsersAdmission{})

// NewRestrictUsersAdmission configures an admission plugin that enforces
// restrictions on adding role bindings in a project.
func NewRestrictUsersAdmission() (admission.Interface, error) {
	return &restrictUsersAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

func (q *restrictUsersAdmission) SetInternalKubeClientSet(c kclientset.Interface) {
	q.kclient = c
}

func (q *restrictUsersAdmission) SetOpenshiftInternalAuthorizationClient(roleBindingRestrictionsGetter authorizationclient.Interface) {
	q.roleBindingRestrictionsGetter = roleBindingRestrictionsGetter.Authorization()
}

func (q *restrictUsersAdmission) SetOpenshiftInternalUserClient(userClient userclient.Interface) {
	q.userClient = userClient
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
func (q *restrictUsersAdmission) Admit(a admission.Attributes) (err error) {

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
		glog.V(4).Infof("No new subjects; admitting")
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

	glog.V(4).Infof("Handling rolebinding %s/%s",
		rolebinding.Namespace, rolebinding.Name)

	newSubjects := subjectsDelta(oldSubjects, rolebinding.Subjects)
	if len(newSubjects) == 0 {
		glog.V(4).Infof("No new subjects; admitting")
		return nil
	}

	// TODO: Cache rolebinding restrictions.
	roleBindingRestrictionList, err := q.roleBindingRestrictionsGetter.RoleBindingRestrictions(ns).
		List(metav1.ListOptions{})
	if err != nil {
		return admission.NewForbidden(a, err)
	}
	if len(roleBindingRestrictionList.Items) == 0 {
		glog.V(4).Infof("No rolebinding restrictions specified; admitting")
		return nil
	}

	checkers := []SubjectChecker{}
	for _, rbr := range roleBindingRestrictionList.Items {
		checker, err := NewSubjectChecker(&rbr.Spec)
		if err != nil {
			return admission.NewForbidden(a, err)
		}
		checkers = append(checkers, checker)
	}

	roleBindingRestrictionContext, err := NewRoleBindingRestrictionContext(ns,
		q.kclient, q.userClient.User(), q.groupCache)
	if err != nil {
		return admission.NewForbidden(a, err)
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

	glog.V(4).Infof("All new subjects are allowed; admitting")

	return nil
}

func (q *restrictUsersAdmission) ValidateInitialization() error {
	if q.kclient == nil {
		return errors.New("RestrictUsersAdmission plugin requires a Kubernetes client")
	}
	if q.roleBindingRestrictionsGetter == nil {
		return errors.New("RestrictUsersAdmission plugin requires an OpenShift client")
	}
	if q.userClient == nil {
		return errors.New("RestrictUsersAdmission plugin requires an OpenShift user client")
	}
	if q.groupCache == nil {
		return errors.New("RestrictUsersAdmission plugin requires a group cache")
	}

	return nil
}

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
	kapi "k8s.io/kubernetes/pkg/api"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oclient "github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("openshift.io/RestrictSubjectBindings",
		func(config io.Reader) (admission.Interface, error) {
			return NewRestrictUsersAdmission()
		})
}

// restrictUsersAdmission implements admission.Interface and enforces
// restrictions on adding rolebindings in a project to permit only designated
// subjects.
type restrictUsersAdmission struct {
	*admission.Handler

	oclient    oclient.Interface
	kclient    kclientset.Interface
	groupCache *usercache.GroupCache
}

var _ = oadmission.WantsOpenshiftClient(&restrictUsersAdmission{})
var _ = oadmission.WantsGroupCache(&restrictUsersAdmission{})
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

func (q *restrictUsersAdmission) SetOpenshiftClient(c oclient.Interface) {
	q.oclient = c
}

func (q *restrictUsersAdmission) SetGroupCache(c *usercache.GroupCache) {
	q.groupCache = c
}

// objectReferenceDelta returns the relative complement of
// []ObjectReference elementsToIgnore in []ObjectReference elements
// (i.e., elements∖elementsToIgnore).
func objectReferenceDelta(elementsToIgnore, elements []kapi.ObjectReference) []kapi.ObjectReference {
	result := []kapi.ObjectReference{}

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
	// We only care about rolebindings and policybindings; ignore anything else.
	gr := a.GetResource().GroupResource()
	switch {
	case authorizationapi.IsResourceOrLegacy("policybindings", gr), authorizationapi.IsResourceOrLegacy("rolebindings", gr):
	default:
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

	var subjects, oldSubjects []kapi.ObjectReference

	obj, oldObj := a.GetObject(), a.GetOldObject()
	switch {
	case authorizationapi.IsResourceOrLegacy("rolebindings", gr):
		rolebinding, ok := obj.(*authorizationapi.RoleBinding)
		if !ok {
			return admission.NewForbidden(a,
				fmt.Errorf("wrong object type for new rolebinding: %T", obj))
		}

		subjects = rolebinding.Subjects
		if len(subjects) == 0 {
			return nil
		}

		if oldObj != nil {
			oldrolebinding, ok := oldObj.(*authorizationapi.RoleBinding)
			if !ok {
				return admission.NewForbidden(a,
					fmt.Errorf("wrong object type for old rolebinding: %T", oldObj))
			}

			oldSubjects = oldrolebinding.Subjects
		}

		glog.V(4).Infof("Handling rolebinding %s/%s",
			rolebinding.Namespace, rolebinding.Name)

	case authorizationapi.IsResourceOrLegacy("policybindings", gr):
		policybinding, ok := obj.(*authorizationapi.PolicyBinding)
		if !ok {
			return admission.NewForbidden(a,
				fmt.Errorf("wrong object type for new policybinding: %T", obj))
		}

		for _, rolebinding := range policybinding.RoleBindings {
			subjects = append(subjects, rolebinding.Subjects...)
		}
		if len(subjects) == 0 {
			return nil
		}

		if oldObj != nil {
			oldpolicybinding, ok := oldObj.(*authorizationapi.PolicyBinding)
			if !ok {
				return admission.NewForbidden(a,
					fmt.Errorf("wrong object type for old policybinding: %T", oldObj))
			}

			for _, rolebinding := range oldpolicybinding.RoleBindings {
				oldSubjects = append(oldSubjects, rolebinding.Subjects...)
			}
		}

		glog.V(4).Infof("Handling policybinding %s/%s",
			policybinding.Namespace, policybinding.Name)
	}

	newSubjects := objectReferenceDelta(oldSubjects, subjects)
	if len(newSubjects) == 0 {
		glog.V(4).Infof("No new subjects; admitting")
		return nil
	}

	// TODO: Cache rolebinding restrictions.
	roleBindingRestrictionList, err := q.oclient.RoleBindingRestrictions(ns).
		List(metav1.ListOptions{})
	if err != nil {
		return admission.NewForbidden(a, err)
	}
	if len(roleBindingRestrictionList.Items) == 0 {
		glog.V(4).Infof("No rolebinding restrictions specified; admitting")
		return nil
	}

	if !q.groupCache.Running() {
		return admission.NewForbidden(a, errors.New("groupCache not running"))
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
		q.kclient, q.oclient, q.groupCache)
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

func (q *restrictUsersAdmission) Validate() error {
	if q.kclient == nil {
		return errors.New("RestrictUsersAdmission plugin requires a Kubernetes client")
	}
	if q.oclient == nil {
		return errors.New("RestrictUsersAdmission plugin requires an OpenShift client")
	}
	if q.groupCache == nil {
		return errors.New("RestrictUsersAdmission plugin requires a group cache")
	}

	return nil
}

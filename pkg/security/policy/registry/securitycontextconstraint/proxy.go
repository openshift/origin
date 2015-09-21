package securitycontextconstraints

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/security/policy/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

type REST struct {
	client client.PodSecurityPolicyInterface
}

func NewREST(client client.PodSecurityPolicyInterface) *REST {
	return &REST{
		client: client,
	}
}

func constraintToPolicy(scc *api.SecurityContextConstraints) *api.PodSecurityPolicy {
	psp := &api.PodSecurityPolicy{}
	psp.ObjectMeta = scc.ObjectMeta

	psp.Spec.Privileged = scc.AllowPrivilegedContainer
	psp.Spec.Capabilities = scc.AllowedCapabilities
	psp.Spec.Volumes.HostPath = scc.AllowHostDirVolumePlugin
	psp.Spec.HostNetwork = scc.AllowHostNetwork
	//TODO should we error here if the psp isn't allowing all ports?
	if scc.AllowHostPorts {
		psp.Spec.HostPorts = []api.HostPortRange{
			{
				Start: 1,
				End:   65535,
			},
		}
	}
	psp.Spec.SELinuxContext = scc.SELinuxContext
	psp.Spec.RunAsUser = scc.RunAsUser
	psp.Spec.Users = scc.Users
	psp.Spec.Groups = scc.Groups
	return psp
}

func policyToConstraint(psp *api.PodSecurityPolicy) *api.SecurityContextConstraints {
	scc := &api.SecurityContextConstraints{}
	scc.ObjectMeta = psp.ObjectMeta
	scc.AllowPrivilegedContainer = psp.Spec.Privileged
	scc.AllowedCapabilities = psp.Spec.Capabilities
	scc.AllowHostDirVolumePlugin = psp.Spec.Volumes.HostPath
	scc.AllowHostNetwork = psp.Spec.HostNetwork
	//TODO is this safe or should we error here because it is an unsupported item?
	scc.AllowHostPorts = len(psp.Spec.HostPorts) > 0
	scc.SELinuxContext = psp.Spec.SELinuxContext
	scc.RunAsUser = psp.Spec.RunAsUser
	scc.Users = psp.Spec.Users
	scc.Groups = psp.Spec.Groups
	return scc
}

// New returns a new Project
func (s *REST) New() runtime.Object {
	return &api.SecurityContextConstraints{}
}

func (s *REST) NewList() runtime.Object {
	return &api.SecurityContextConstraintsList{}
}

func (s *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	psp, err := s.client.Get(name)
	if err != nil {
		return nil, err
	}
	return policyToConstraint(psp), nil
}

func (s *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	pspList, err := s.client.List(label, field)
	if err != nil {
		return nil, err
	}
	sccs := []api.SecurityContextConstraints{}
	for _, psp := range pspList.Items {
		sccs = append(sccs, *policyToConstraint(&psp))
	}
	sccList := &api.SecurityContextConstraintsList{
		Items: sccs,
	}
	return sccList, nil
}

func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	scc := obj.(*api.SecurityContextConstraints)
	psp, err := s.client.Create(constraintToPolicy(scc))
	if err != nil {
		return nil, err
	}
	return policyToConstraint(psp), nil
}

func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	scc := obj.(*api.SecurityContextConstraints)
	psp, err := s.client.Create(constraintToPolicy(scc))
	if err != nil {
		return nil, false, err
	}
	return policyToConstraint(psp), true, nil
}

func (s *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	err := s.client.Delete(name)
	return nil, err
}

func (s *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.client.Watch(label, field, resourceVersion)
}

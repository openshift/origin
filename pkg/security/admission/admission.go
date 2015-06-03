package admission

import (
	"fmt"
	"io"

	kadmission "github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	scc "github.com/GoogleCloudPlatform/kubernetes/pkg/securitycontextconstraints"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	allocator "github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
)

func init() {
	kadmission.RegisterPlugin("SecurityContextConstraint", func(client client.Interface, config io.Reader) (kadmission.Interface, error) {
		return NewConstraint(client), nil
	})
}

type constraint struct {
	*kadmission.Handler
	client client.Interface
	store  cache.Store
}

var _ kadmission.Interface = &constraint{}

func NewConstraint(kclient client.Interface) kadmission.Interface {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return kclient.SecurityContextConstraints().List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return kclient.SecurityContextConstraints().Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&kapi.SecurityContextConstraints{},
		store,
		0,
	)
	reflector.Run()

	return &constraint{
		Handler: kadmission.NewHandler(kadmission.Create, kadmission.Update),
		client:  kclient,
		store:   store,
	}
}

// Admit determines if the pod should be admitted based on the requested security context
// and the available SCCs.
//
// 1.  Find all the SCCs that the service account or user has access to
// 2.  Full resolve each SCC
//     1.  Determine if the user/selinux strategies requires pre-allocated values
//     2.  Retrieve pre-allocated values from the namespace and set them on the strategies
// 3.  Create the set of providers from the fully configured SCCs
// 4.  Generate and validate each container's SC against the set of SCC providers.  If any container
//     fails to validate then the entire request is rejected
func (c *constraint) Admit(a kadmission.Attributes) error {
	if a.GetResource() != string(kapi.ResourcePods) {
		return nil
	}

	pod, ok := a.GetObject().(*kapi.Pod)
	// if we can't convert then we don't handle this object so just return
	if !ok {
		return nil
	}

	// get all constraints that are usable by the user
	matchedConstraints, err := getMatchingSecurityContextConstraints(c.store, a.GetUserInfo())
	if err != nil {
		return kadmission.NewForbidden(a, err)
	}
	// if there is a service account get the constraints for the service account
	if len(pod.Spec.ServiceAccount) > 0 {
		serviceAccount, err := getServiceAccount(c.client, a.GetNamespace(), pod)
		if err != nil {
			return kadmission.NewForbidden(a, err)
		}

		userInfo := serviceaccount.UserInfo(a.GetNamespace(), serviceAccount.Name, string(serviceAccount.UID))
		saConstraints, err := getMatchingSecurityContextConstraints(c.store, userInfo)
		if err != nil {
			return kadmission.NewForbidden(a, err)
		}
		matchedConstraints = append(matchedConstraints, saConstraints...)
	}

	providers := make(map[string]scc.SecurityContextConstraintsProvider, 0)
	// namespace is declared here for reuse but we will not fetch it unless required by
	// the matched constraints
	var namespace *kapi.Namespace

	// set pre-allocated values on constraints.  By placing them in a map we also de-dupe the constraints
	// that match the sa and user.
	for _, constraint := range matchedConstraints {
		if requiresPreAllocatedUIDRange(constraint) {
			if namespace == nil {
				namespace, err = c.client.Namespaces().Get(a.GetNamespace())
				if err != nil {
					return kadmission.NewForbidden(a, err)
				}
			}

			min, max, err := getPreallocatedUIDRange(namespace)
			if err != nil {
				kadmission.NewForbidden(a, err)
			}
			constraint.RunAsUser.UIDRangeMin = min
			constraint.RunAsUser.UIDRangeMax = max
		}

		if requiresPreAllocatedSELinuxLevel(constraint) {
			if namespace == nil {
				namespace, err = c.client.Namespaces().Get(a.GetNamespace())
				if err != nil {
					return kadmission.NewForbidden(a, err)
				}
			}

			// if we don't have options create them here so we can add the preallocated settings
			if constraint.SELinuxContext.SELinuxOptions == nil {
				constraint.SELinuxContext.SELinuxOptions = &kapi.SELinuxOptions{}
			}

			level, err := getPreallocatedLevel(namespace)
			if err != nil {
				kadmission.NewForbidden(a, err)
			}
			constraint.SELinuxContext.SELinuxOptions.Level = level
		}

		provider, err := scc.NewSimpleProvider(constraint)
		if err != nil {
			return err
		}
		providers[constraint.Name] = provider
	}

	for i, container := range pod.Spec.Containers {
		context, providerName, err := c.getSecurityContextForContainer(pod, i, providers)
		if err != nil {
			return kadmission.NewForbidden(a, err)
		}
		// We validated against a context, set it on the container and annotated the pod
		pod.Spec.Containers[i].SecurityContext = context
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string, 0)
		}
		pod.Annotations[createPodAnnotationKey(&container)] = providerName
	}

	return nil
}

// getSecurityContextForContainer iterates through the providers and generates then validates
// the sc against the scc.  It operates on a copy of the pod so that it does not change any
// actual values on the containers since the scc provider will set defaults for anything that isn't
// specified.
func (c *constraint) getSecurityContextForContainer(pod *kapi.Pod, containerIndex int, providers map[string]scc.SecurityContextConstraintsProvider) (*kapi.SecurityContext, string, error) {
	for providerName, provider := range providers {
		// Create a security context for this provider

		// Since the provider changes values on the container we need to work with a copy so that
		// as we iterate through the providers we don't get defaults set from previous providers.
		podCopy, err := createPodCopy(pod)
		if err != nil {
			return nil, "", err
		}

		container := &podCopy.Spec.Containers[containerIndex]
		context, err := provider.CreateSecurityContext(podCopy, container)
		if err != nil {
			return nil, "", err
		}

		// Validate the copy of the container against the security context provider
		container.SecurityContext = context
		errs := provider.ValidateSecurityContext(pod, container)
		if len(errs) == 0 {
			return context, providerName, nil
		}
	}

	// If we have reached this code, we couldn't find an SCC that matched
	// the requested security context for this container
	return nil, "", fmt.Errorf("unable to find a valid security context constraint for container %s", pod.Spec.Containers[containerIndex].Name)
}

// createPodCopy creates a copy of a pod.
func createPodCopy(pod *kapi.Pod) (*kapi.Pod, error) {
	pCopy, err := kapi.Scheme.Copy(pod)
	if err != nil {
		return nil, err
	}
	podCopy, ok := pCopy.(*kapi.Pod)
	if !ok {
		return nil, fmt.Errorf("error converting copied container to a container type")
	}
	return podCopy, nil
}

// createPodAnnotationKey creates a container suffixed annotation key based on validatedSCCAnnotation.
func createPodAnnotationKey(container *kapi.Container) string {
	return fmt.Sprintf("%s-%s", allocator.ValidatedSCCAnnotation, container.Name)
}

// getMatchingSecurityContextConstraints returns constraints from the store that match the group,
// uid, or user of the service account.
func getMatchingSecurityContextConstraints(store cache.Store, userInfo user.Info) ([]*kapi.SecurityContextConstraints, error) {
	matchedConstraints := make([]*kapi.SecurityContextConstraints, 0)

	for _, c := range store.List() {
		constraint, ok := c.(*kapi.SecurityContextConstraints)
		if !ok {
			return nil, errors.NewInternalError(fmt.Errorf("error converting object from store to a security context constraint: %v", c))
		}
		for _, userGroup := range userInfo.GetGroups() {
			if constraintSupportsGroup(userGroup, constraint.Groups) {
				matchedConstraints = append(matchedConstraints, constraint)
				break
			}
		}

		for _, user := range constraint.Users {
			if userInfo.GetName() == user {
				matchedConstraints = append(matchedConstraints, constraint)
				break
			}
		}
	}

	return matchedConstraints, nil
}

// getPreallocatedUIDRange retrieves the annotated value from the service account, splits it to make
// the min/max and formats the data into the necessary types for the strategy options.
func getPreallocatedUIDRange(ns *kapi.Namespace) (*int64, *int64, error) {
	annotationVal, ok := ns.Annotations[allocator.UIDRangeAnnotation]
	if !ok {
		return nil, nil, errors.NewInternalError(fmt.Errorf("unable to find annotation %s", allocator.UIDRangeAnnotation))
	}
	uidBlock, err := uid.ParseBlock(annotationVal)
	if err != nil {
		return nil, nil, err
	}

	var min int64 = int64(uidBlock.Start)
	var max int64 = int64(uidBlock.End)
	return &min, &max, nil
}

// getPreallocatedLevel gets the annotated value from the service account.
func getPreallocatedLevel(ns *kapi.Namespace) (level string, err error) {
	level, ok := ns.Annotations[allocator.MCSAnnotation]
	if !ok {
		err = errors.NewInternalError(fmt.Errorf("unable to find annotation %s", allocator.MCSAnnotation))
		return
	}
	return
}

// requiresPreAllocatedUIDRange returns true if the strategy is must run in range and the min or max
// is not set.
func requiresPreAllocatedUIDRange(constraint *kapi.SecurityContextConstraints) bool {
	return constraint.RunAsUser.Type == kapi.RunAsUserStrategyMustRunAsRange &&
		(constraint.RunAsUser.UIDRangeMin == nil || constraint.RunAsUser.UIDRangeMax == nil)
}

// requiresPreAllocatedSELinuxLevel returns true if the strategy is must run as and the level is
// not set.
func requiresPreAllocatedSELinuxLevel(constraint *kapi.SecurityContextConstraints) bool {
	if constraint.SELinuxContext.Type == kapi.SELinuxStrategyMustRunAs {
		if constraint.SELinuxContext.SELinuxOptions == nil {
			return true
		}
		return constraint.SELinuxContext.SELinuxOptions.Level == ""
	}
	return false
}

// constraintSupportsGroup checks that group is in constraintGroups.
func constraintSupportsGroup(group string, constraintGroups []string) bool {
	for _, g := range constraintGroups {
		if g == group {
			return true
		}
	}
	return false
}

// getServiceAccount is a helper to get the service account.
func getServiceAccount(kclient client.Interface, ns string, pod *kapi.Pod) (*kapi.ServiceAccount, error) {
	serviceAccountName := pod.Spec.ServiceAccount
	if serviceAccountName == "" {
		return nil, errors.NewBadRequest("pod with no service account")
	}

	serviceAccount, err := kclient.ServiceAccounts(ns).Get(serviceAccountName)
	if err != nil {
		return nil, err
	}
	return serviceAccount, nil
}

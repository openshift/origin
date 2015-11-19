package admission

import (
	"fmt"
	"io"
	"sort"
	"strings"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	sc "k8s.io/kubernetes/pkg/securitycontext"
	scc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	allocator "github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/golang/glog"
)

func init() {
	kadmission.RegisterPlugin("SecurityContextConstraint", func(client client.Interface, config io.Reader) (kadmission.Interface, error) {
		constraintAdmitter := NewConstraint(client)
		constraintAdmitter.Run()
		return constraintAdmitter, nil
	})
}

type constraint struct {
	*kadmission.Handler
	client client.Interface

	reflector *cache.Reflector
	stopChan  chan struct{}
	store     cache.Store
}

var _ kadmission.Interface = &constraint{}

// NewConstraint creates a new SCC constraint admission plugin.
func NewConstraint(kclient client.Interface) *constraint {
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

	return &constraint{
		Handler: kadmission.NewHandler(kadmission.Create),
		client:  kclient,

		store:     store,
		reflector: reflector,
	}
}

func (a *constraint) Run() {
	if a.stopChan == nil {
		a.stopChan = make(chan struct{})
		a.reflector.RunUntil(a.stopChan)
	}
}
func (a *constraint) Stop() {
	if a.stopChan != nil {
		close(a.stopChan)
		a.stopChan = nil
	}
}

// Admit determines if the pod should be admitted based on the requested security context
// and the available SCCs.
//
// 1.  Find SCCs for the user.
// 2.  Find SCCs for the SA.  If there is an error retrieving SA SCCs it is not fatal.
// 3.  Remove duplicates between the user/SA SCCs.
// 4.  Create the providers, includes setting pre-allocated values if necessary.
// 5.  Try to generate and validate an SCC with providers.  If we find one then admit the pod
//     with the validated SCC.  If we don't find any reject the pod and give all errors from the
//     failed attempts.
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
	glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) in namespace %s with user info %v", pod.Name, pod.GenerateName, a.GetNamespace(), a.GetUserInfo())
	matchedConstraints, err := getMatchingSecurityContextConstraints(c.store, a.GetUserInfo())
	if err != nil {
		return kadmission.NewForbidden(a, err)
	}

	// get all constraints that are usable by the SA
	if len(pod.Spec.ServiceAccountName) > 0 {
		userInfo := serviceaccount.UserInfo(a.GetNamespace(), pod.Spec.ServiceAccountName, "")
		glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) with service account info %v", pod.Name, pod.GenerateName, userInfo)
		saConstraints, err := getMatchingSecurityContextConstraints(c.store, userInfo)
		if err != nil {
			return kadmission.NewForbidden(a, err)
		}
		matchedConstraints = append(matchedConstraints, saConstraints...)
	}

	// remove duplicate constraints and sort
	matchedConstraints = deduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(ByPriority(matchedConstraints))
	providers, errs := c.createProvidersFromConstraints(a.GetNamespace(), matchedConstraints)
	logProviders(pod, providers, errs)

	if len(providers) == 0 {
		return kadmission.NewForbidden(a, fmt.Errorf("no providers available to validated pod request"))
	}

	// all containers in a single pod must validate under a single provider or we will reject the request
	validationErrs := fielderrors.ValidationErrorList{}
	for _, provider := range providers {
		if errs := assignSecurityContext(provider, pod); len(errs) > 0 {
			validationErrs = append(validationErrs, errs.Prefix(fmt.Sprintf("provider %s: ", provider.GetSCCName()))...)
			continue
		}

		// the entire pod validated, annotate and accept the pod
		glog.V(4).Infof("pod %s (generate: %s) validated against provider %s", pod.Name, pod.GenerateName, provider.GetSCCName())
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = map[string]string{}
		}
		pod.ObjectMeta.Annotations[allocator.ValidatedSCCAnnotation] = provider.GetSCCName()
		return nil
	}

	// we didn't validate against any security context constraint provider, reject the pod and give the errors for each attempt
	glog.V(4).Infof("unable to validate pod %s (generate: %s) against any security context constraint: %v", pod.Name, pod.GenerateName, validationErrs)
	return kadmission.NewForbidden(a, fmt.Errorf("unable to validate against any security context constraint: %v", validationErrs))
}

// assignSecurityContext creates a security context for each container in the pod
// and validates that the sc falls within the scc constraints.  All containers must validate against
// the same scc or is not considered valid.
func assignSecurityContext(provider scc.SecurityContextConstraintsProvider, pod *kapi.Pod) fielderrors.ValidationErrorList {
	generatedSCs := make([]*kapi.SecurityContext, len(pod.Spec.Containers))

	errs := fielderrors.ValidationErrorList{}

	psc, err := provider.CreatePodSecurityContext(pod)
	if err != nil {
		errs = append(errs, fielderrors.NewFieldInvalid("spec.securityContext", pod.Spec.SecurityContext, err.Error()))
	}

	// save the original PSC and validate the generated PSC.  Leave the generated PSC
	// set for container generation/validation.  We will reset to original post container
	// validation.
	originalPSC := pod.Spec.SecurityContext
	pod.Spec.SecurityContext = psc
	errs = append(errs, provider.ValidatePodSecurityContext(pod).Prefix("spec.securityContext")...)

	// Note: this is not changing the original container, we will set container SCs later so long
	// as all containers validated under the same SCC.
	for i, containerCopy := range pod.Spec.Containers {
		// We will determine the effective security context for the container and validate against that
		// since that is how the sc provider will eventually apply settings in the runtime.
		// This results in an SC that is based on the Pod's PSC with the set fields from the container
		// overriding pod level settings.
		containerCopy.SecurityContext = sc.DetermineEffectiveSecurityContext(pod, &containerCopy)

		sc, err := provider.CreateContainerSecurityContext(pod, &containerCopy)
		if err != nil {
			errs = append(errs, fielderrors.NewFieldInvalid(fmt.Sprintf("spec.containers[%d].securityContext", i), "", err.Error()))
			continue
		}
		generatedSCs[i] = sc

		containerCopy.SecurityContext = sc
		errs = append(errs, provider.ValidateContainerSecurityContext(pod, &containerCopy).Prefix(fmt.Sprintf("spec.containers[%d].securityContext", i))...)
	}

	if len(errs) > 0 {
		// ensure psc is not mutated if there are errors
		pod.Spec.SecurityContext = originalPSC
		return errs
	}

	// if we've reached this code then we've generated and validated an SC for every container in the
	// pod so let's apply what we generated.  Note: the psc is already applied.
	for i, sc := range generatedSCs {
		pod.Spec.Containers[i].SecurityContext = sc
	}
	return nil
}

// createProvidersFromConstraints creates providers from the constraints supplied, including
// looking up pre-allocated values if necessary using the pod's namespace.
func (c *constraint) createProvidersFromConstraints(ns string, sccs []*kapi.SecurityContextConstraints) ([]scc.SecurityContextConstraintsProvider, []error) {
	var (
		// namespace is declared here for reuse but we will not fetch it unless required by the matched constraints
		namespace *kapi.Namespace
		// collected providers
		providers []scc.SecurityContextConstraintsProvider
		// collected errors to return
		errs []error
	)

	// set pre-allocated values on constraints
	for _, constraint := range sccs {
		var err error
		resolveUIDRange := requiresPreAllocatedUIDRange(constraint)
		resolveSELinuxLevel := requiresPreAllocatedSELinuxLevel(constraint)
		resolveFSGroup := requiresPreallocatedFSGroup(constraint)
		resolveSupplementalGroups := requiresPreallocatedSupplementalGroups(constraint)
		requiresNamespaceAllocations := resolveUIDRange || resolveSELinuxLevel || resolveFSGroup || resolveSupplementalGroups

		if requiresNamespaceAllocations {
			// Ensure we have the namespace
			namespace, err = c.getNamespace(ns, namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("error fetching namespace %s required to preallocate values for %s: %v", ns, constraint.Name, err))
				continue
			}
		}

		// Make a copy of the constraint so we don't mutate the store's cache
		var constraintCopy kapi.SecurityContextConstraints = *constraint
		constraint = &constraintCopy

		// Resolve the values from the namespace
		if resolveUIDRange {
			constraint.RunAsUser.UIDRangeMin, constraint.RunAsUser.UIDRangeMax, err = getPreallocatedUIDRange(namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to find pre-allocated uid annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err))
				continue
			}
		}
		if resolveSELinuxLevel {
			var level string
			if level, err = getPreallocatedLevel(namespace); err != nil {
				errs = append(errs, fmt.Errorf("unable to find pre-allocated mcs annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err))
				continue
			}

			// SELinuxOptions is a pointer, if we are resolving and it is already initialized
			// we need to make a copy of it so we don't manipulate the store's cache.
			if constraint.SELinuxContext.SELinuxOptions != nil {
				var seLinuxOptionsCopy kapi.SELinuxOptions = *constraint.SELinuxContext.SELinuxOptions
				constraint.SELinuxContext.SELinuxOptions = &seLinuxOptionsCopy
			} else {
				constraint.SELinuxContext.SELinuxOptions = &kapi.SELinuxOptions{}
			}
			constraint.SELinuxContext.SELinuxOptions.Level = level
		}
		if resolveFSGroup {
			fsGroup, err := getPreallocatedFSGroup(namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to find pre-allocated group annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err))
				continue
			}
			constraint.FSGroup.Ranges = fsGroup
		}
		if resolveSupplementalGroups {
			supplementalGroups, err := getPreallocatedSupplementalGroups(namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to find pre-allocated group annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err))
				continue
			}
			constraint.SupplementalGroups.Ranges = supplementalGroups
		}

		// Create the provider
		provider, err := scc.NewSimpleProvider(constraint)
		if err != nil {
			errs = append(errs, fmt.Errorf("error creating provider for SCC %s in namespace %s: %v", constraint.Name, ns, err))
			continue
		}
		providers = append(providers, provider)
	}
	return providers, errs
}

// getNamespace retrieves a namespace only if ns is nil.
func (c *constraint) getNamespace(name string, ns *kapi.Namespace) (*kapi.Namespace, error) {
	if ns != nil && name == ns.Name {
		return ns, nil
	}
	return c.client.Namespaces().Get(name)
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
		if ConstraintAppliesTo(constraint, userInfo) {
			matchedConstraints = append(matchedConstraints, constraint)
		}
	}

	return matchedConstraints, nil
}

// constraintAppliesTo inspects the constraint's users and groups against the userInfo to determine
// if it is usable by the userInfo.
func ConstraintAppliesTo(constraint *kapi.SecurityContextConstraints, userInfo user.Info) bool {
	for _, user := range constraint.Users {
		if userInfo.GetName() == user {
			return true
		}
	}
	for _, userGroup := range userInfo.GetGroups() {
		if constraintSupportsGroup(userGroup, constraint.Groups) {
			return true
		}
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

// getPreallocatedUIDRange retrieves the annotated value from the namespace, splits it to make
// the min/max and formats the data into the necessary types for the strategy options.
func getPreallocatedUIDRange(ns *kapi.Namespace) (*int64, *int64, error) {
	annotationVal, ok := ns.Annotations[allocator.UIDRangeAnnotation]
	if !ok {
		return nil, nil, fmt.Errorf("unable to find annotation %s", allocator.UIDRangeAnnotation)
	}
	if len(annotationVal) == 0 {
		return nil, nil, fmt.Errorf("found annotation %s but it was empty", allocator.UIDRangeAnnotation)
	}
	uidBlock, err := uid.ParseBlock(annotationVal)
	if err != nil {
		return nil, nil, err
	}

	var min int64 = int64(uidBlock.Start)
	var max int64 = int64(uidBlock.End)
	glog.V(4).Infof("got preallocated values for min: %d, max: %d for uid range in namespace %s", min, max, ns.Name)
	return &min, &max, nil
}

// getPreallocatedLevel gets the annotated value from the namespace.
func getPreallocatedLevel(ns *kapi.Namespace) (string, error) {
	level, ok := ns.Annotations[allocator.MCSAnnotation]
	if !ok {
		return "", fmt.Errorf("unable to find annotation %s", allocator.MCSAnnotation)
	}
	if len(level) == 0 {
		return "", fmt.Errorf("found annotation %s but it was empty", allocator.MCSAnnotation)
	}
	glog.V(4).Infof("got preallocated value for level: %s for selinux options in namespace %s", level, ns.Name)
	return level, nil
}

// getSupplementalGroupsAnnotation provides a backwards compatible way to get supplemental groups
// annotations from a namespace by looking for SupplementalGroupsAnnotation and falling back to
// UIDRangeAnnotation if it is not found.
func getSupplementalGroupsAnnotation(ns *kapi.Namespace) (string, error) {
	groups, ok := ns.Annotations[allocator.SupplementalGroupsAnnotation]
	if !ok {
		glog.V(4).Infof("unable to find supplemental group annotation %s falling back to %s", allocator.SupplementalGroupsAnnotation, allocator.UIDRangeAnnotation)

		groups, ok = ns.Annotations[allocator.UIDRangeAnnotation]
		if !ok {
			return "", fmt.Errorf("unable to find supplemental group or uid annotation for namespace %s", ns.Name)
		}
	}

	if len(groups) == 0 {
		return "", fmt.Errorf("unable to find groups using %s and %s annotations", allocator.SupplementalGroupsAnnotation, allocator.UIDRangeAnnotation)
	}
	return groups, nil
}

// getPreallocatedFSGroup gets the annotated value from the namespace.
func getPreallocatedFSGroup(ns *kapi.Namespace) ([]kapi.IDRange, error) {
	groups, err := getSupplementalGroupsAnnotation(ns)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("got preallocated value for groups: %s in namespace %s", groups, ns.Name)

	blocks, err := parseSupplementalGroupAnnotation(groups)
	if err != nil {
		return nil, err
	}
	return []kapi.IDRange{
		{
			Min: int64(blocks[0].Start),
			Max: int64(blocks[0].Start),
		},
	}, nil
}

// getPreallocatedSupplementalGroups gets the annotated value from the namespace.
func getPreallocatedSupplementalGroups(ns *kapi.Namespace) ([]kapi.IDRange, error) {
	groups, err := getSupplementalGroupsAnnotation(ns)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("got preallocated value for groups: %s in namespace %s", groups, ns.Name)

	blocks, err := parseSupplementalGroupAnnotation(groups)
	if err != nil {
		return nil, err
	}

	idRanges := []kapi.IDRange{}
	for _, block := range blocks {
		rng := kapi.IDRange{
			Min: int64(block.Start),
			Max: int64(block.End),
		}
		idRanges = append(idRanges, rng)
	}
	return idRanges, nil
}

// parseSupplementalGroupAnnotation parses the group annotation into blocks.
func parseSupplementalGroupAnnotation(groups string) ([]uid.Block, error) {
	blocks := []uid.Block{}
	segments := strings.Split(groups, ",")
	for _, segment := range segments {
		block, err := uid.ParseBlock(segment)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no blocks parsed from annotation %s", groups)
	}
	return blocks, nil
}

// requiresPreAllocatedUIDRange returns true if the strategy is must run in range and the min or max
// is not set.
func requiresPreAllocatedUIDRange(constraint *kapi.SecurityContextConstraints) bool {
	if constraint.RunAsUser.Type != kapi.RunAsUserStrategyMustRunAsRange {
		return false
	}
	return constraint.RunAsUser.UIDRangeMin == nil && constraint.RunAsUser.UIDRangeMax == nil
}

// requiresPreAllocatedSELinuxLevel returns true if the strategy is must run as and the level is not set.
func requiresPreAllocatedSELinuxLevel(constraint *kapi.SecurityContextConstraints) bool {
	if constraint.SELinuxContext.Type != kapi.SELinuxStrategyMustRunAs {
		return false
	}
	if constraint.SELinuxContext.SELinuxOptions == nil {
		return true
	}
	return constraint.SELinuxContext.SELinuxOptions.Level == ""
}

// requiresPreAllocatedSELinuxLevel returns true if the strategy is must run as and there is no
// range specified.
func requiresPreallocatedSupplementalGroups(constraint *kapi.SecurityContextConstraints) bool {
	if constraint.SupplementalGroups.Type != kapi.SupplementalGroupsStrategyMustRunAs {
		return false
	}
	return len(constraint.SupplementalGroups.Ranges) == 0
}

// requiresPreallocatedFSGroup returns true if the strategy is must run as and there is no
// range specified.
func requiresPreallocatedFSGroup(constraint *kapi.SecurityContextConstraints) bool {
	if constraint.FSGroup.Type != kapi.FSGroupStrategyMustRunAs {
		return false
	}
	return len(constraint.FSGroup.Ranges) == 0
}

// deduplicateSecurityContextConstraints ensures we have a unique slice of constraints.
func deduplicateSecurityContextConstraints(sccs []*kapi.SecurityContextConstraints) []*kapi.SecurityContextConstraints {
	deDuped := []*kapi.SecurityContextConstraints{}
	added := sets.NewString()

	for _, s := range sccs {
		if !added.Has(s.Name) {
			deDuped = append(deDuped, s)
			added.Insert(s.Name)
		}
	}
	return deDuped
}

// logProviders logs what providers were found for the pod as well as any errors that were encountered
// while creating providers.
func logProviders(pod *kapi.Pod, providers []scc.SecurityContextConstraintsProvider, providerCreationErrs []error) {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.GetSCCName()
	}
	glog.V(4).Infof("validating pod %s (generate: %s) against providers %s", pod.Name, pod.GenerateName, strings.Join(names, ","))

	for _, err := range providerCreationErrs {
		glog.V(4).Infof("provider creation error: %v", err)
	}
}

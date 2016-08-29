package scc

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	sc "k8s.io/kubernetes/pkg/securitycontext"
	kscc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oscache "github.com/openshift/origin/pkg/client/cache"
	allocator "github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
)

// SCCMatcher defines interface for SecurityContextConstraint matcher
type SCCMatcher interface {
	FindApplicableSCCs(user user.Info) ([]*kapi.SecurityContextConstraints, error)
}

// DefaultSCCMatcher implements default implementation for SCCMatcher interface
type DefaultSCCMatcher struct {
	cache *oscache.IndexerToSecurityContextConstraintsLister
}

// NewDefaultSCCMatcher builds and initializes a DefaultSCCMatcher
func NewDefaultSCCMatcher(c *oscache.IndexerToSecurityContextConstraintsLister) SCCMatcher {
	return DefaultSCCMatcher{cache: c}
}

// FindApplicableSCCs implements SCCMatcher interface for DefaultSCCMatcher
func (d DefaultSCCMatcher) FindApplicableSCCs(userInfo user.Info) ([]*kapi.SecurityContextConstraints, error) {
	var matchedConstraints []*kapi.SecurityContextConstraints
	constraints, err := d.cache.List()
	if err != nil {
		return nil, err
	}
	for _, constraint := range constraints {
		if ConstraintAppliesTo(constraint, userInfo) {
			matchedConstraints = append(matchedConstraints, constraint)
		}
	}
	return matchedConstraints, nil
}

// ConstraintAppliesTo inspects the constraint's users and groups against the userInfo to determine
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

// AssignSecurityContext creates a security context for each container in the pod
// and validates that the sc falls within the scc constraints.  All containers must validate against
// the same scc or is not considered valid.
func AssignSecurityContext(provider kscc.SecurityContextConstraintsProvider, pod *kapi.Pod, fldPath *field.Path) field.ErrorList {
	generatedSCs := make([]*kapi.SecurityContext, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))

	errs := field.ErrorList{}

	psc, generatedAnnotations, err := provider.CreatePodSecurityContext(pod)
	if err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec", "securityContext"), pod.Spec.SecurityContext, err.Error()))
	}

	// save the original PSC and validate the generated PSC.  Leave the generated PSC
	// set for container generation/validation.  We will reset to original post container
	// validation.
	originalPSC := pod.Spec.SecurityContext
	originalAnnotations := pod.Annotations

	pod.Spec.SecurityContext = psc
	pod.Annotations = generatedAnnotations
	errs = append(errs, provider.ValidatePodSecurityContext(pod, field.NewPath("spec", "securityContext"))...)

	// Note: this is not changing the original container, we will set container SCs later so long
	// as all containers validated under the same SCC.
	containerPath := field.NewPath("spec", "initContainers")
	for i, containerCopy := range pod.Spec.InitContainers {
		csc, resolutionErrs := resolveContainerSecurityContext(provider, pod, &containerCopy, containerPath.Index(i))
		errs = append(errs, resolutionErrs...)
		if len(resolutionErrs) > 0 {
			continue
		}
		generatedSCs[i] = csc
	}

	base := len(pod.Spec.InitContainers)

	// Note: this is not changing the original container, we will set container SCs later so long
	// as all containers validated under the same SCC.
	containerPath = field.NewPath("spec", "containers")
	for i, containerCopy := range pod.Spec.Containers {
		csc, resolutionErrs := resolveContainerSecurityContext(provider, pod, &containerCopy, containerPath.Index(i))
		errs = append(errs, resolutionErrs...)
		if len(resolutionErrs) > 0 {
			continue
		}
		generatedSCs[i+base] = csc
	}
	if len(errs) > 0 {
		// ensure psc is not mutated if there are errors
		pod.Spec.SecurityContext = originalPSC
		pod.Annotations = originalAnnotations
		return errs
	}

	// if we've reached this code then we've generated and validated an SC for every container in the
	// pod so let's apply what we generated.  Note: the psc is already applied.
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].SecurityContext = generatedSCs[i]
	}
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].SecurityContext = generatedSCs[i+base]
	}
	return nil
}

// resolveContainerSecurityContext checks the provided container against the provider, returning any
// validation errors encountered on the resulting security context, or the security context that was
// resolved. The SecurityContext field of the container is updated, so ensure that a copy of the original
// container is passed here if you wish to preserve the original input.
func resolveContainerSecurityContext(provider kscc.SecurityContextConstraintsProvider, pod *kapi.Pod, container *kapi.Container, path *field.Path) (*kapi.SecurityContext, field.ErrorList) {
	// We will determine the effective security context for the container and validate against that
	// since that is how the sc provider will eventually apply settings in the runtime.
	// This results in an SC that is based on the Pod's PSC with the set fields from the container
	// overriding pod level settings.
	container.SecurityContext = sc.DetermineEffectiveSecurityContext(pod, container)

	csc, err := provider.CreateContainerSecurityContext(pod, container)
	if err != nil {
		return nil, field.ErrorList{field.Invalid(path.Child("securityContext"), "", err.Error())}
	}
	container.SecurityContext = csc
	return csc, provider.ValidateContainerSecurityContext(pod, container, path.Child("securityContext"))
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

// DeduplicateSecurityContextConstraints ensures we have a unique slice of constraints.
func DeduplicateSecurityContextConstraints(sccs []*kapi.SecurityContextConstraints) []*kapi.SecurityContextConstraints {
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

// getNamespaceByName retrieves a namespace only if ns is nil.
func getNamespaceByName(name string, ns *kapi.Namespace, client clientset.Interface) (*kapi.Namespace, error) {
	if ns != nil && name == ns.Name {
		return ns, nil
	}
	return client.Core().Namespaces().Get(name)
}

// CreateProvidersFromConstraints creates providers from the constraints supplied, including
// looking up pre-allocated values if necessary using the pod's namespace.
func CreateProvidersFromConstraints(ns string, sccs []*kapi.SecurityContextConstraints, client clientset.Interface) ([]kscc.SecurityContextConstraintsProvider, []error) {
	var (
		// namespace is declared here for reuse but we will not fetch it unless required by the matched constraints
		namespace *kapi.Namespace
		// collected providers
		providers []kscc.SecurityContextConstraintsProvider
		// collected errors to return
		errs []error
	)

	// set pre-allocated values on constraints
	for _, constraint := range sccs {
		var (
			provider kscc.SecurityContextConstraintsProvider
			err      error
		)
		provider, namespace, err = CreateProviderFromConstraint(ns, namespace, constraint, client)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		providers = append(providers, provider)
	}
	return providers, errs
}

// CreateProviderFromConstraint creates a SecurityContextConstraintProvider from a SecurityContextConstraint
func CreateProviderFromConstraint(ns string, namespace *kapi.Namespace, constraint *kapi.SecurityContextConstraints, client clientset.Interface) (kscc.SecurityContextConstraintsProvider, *kapi.Namespace, error) {
	var err error
	resolveUIDRange := requiresPreAllocatedUIDRange(constraint)
	resolveSELinuxLevel := requiresPreAllocatedSELinuxLevel(constraint)
	resolveFSGroup := requiresPreallocatedFSGroup(constraint)
	resolveSupplementalGroups := requiresPreallocatedSupplementalGroups(constraint)
	requiresNamespaceAllocations := resolveUIDRange || resolveSELinuxLevel || resolveFSGroup || resolveSupplementalGroups

	if requiresNamespaceAllocations {
		// Ensure we have the namespace
		namespace, err = getNamespaceByName(ns, namespace, client)
		if err != nil {
			return nil, namespace, fmt.Errorf("error fetching namespace %s required to preallocate values for %s: %v", ns, constraint.Name, err)
		}
	}

	// Make a copy of the constraint so we don't mutate the store's cache
	var constraintCopy kapi.SecurityContextConstraints = *constraint
	constraint = &constraintCopy

	// Resolve the values from the namespace
	if resolveUIDRange {
		constraint.RunAsUser.UIDRangeMin, constraint.RunAsUser.UIDRangeMax, err = getPreallocatedUIDRange(namespace)
		if err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated uid annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}
	}
	if resolveSELinuxLevel {
		var level string
		if level, err = getPreallocatedLevel(namespace); err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated mcs annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
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
			return nil, namespace, fmt.Errorf("unable to find pre-allocated group annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}
		constraint.FSGroup.Ranges = fsGroup
	}
	if resolveSupplementalGroups {
		supplementalGroups, err := getPreallocatedSupplementalGroups(namespace)
		if err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated group annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}
		constraint.SupplementalGroups.Ranges = supplementalGroups
	}

	// Create the provider
	provider, err := kscc.NewSimpleProvider(constraint)
	if err != nil {
		return nil, namespace, fmt.Errorf("error creating provider for SCC %s in namespace %s: %v", constraint.Name, ns, err)
	}
	return provider, namespace, nil
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
